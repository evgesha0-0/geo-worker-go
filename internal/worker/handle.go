package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"geo-worker-go/internal/config"
	"geo-worker-go/internal/geometry"
	"geo-worker-go/internal/natsclient"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

func HandleRequestMessage(
	ctx context.Context,
	cfg config.Config,
	resources *natsclient.NATSResources,
	msg *nats.Msg,
) error {
	log.Printf("received request message: subject=%s size=%d bytes", msg.Subject, len(msg.Data))

	watchdogCtx, stopWatchdog := context.WithCancel(ctx)
	defer stopWatchdog()

	watchdogInterval := time.Duration(cfg.Ack_Wait) * time.Second / 2
	if watchdogInterval <= 0 {
		watchdogInterval = time.Second
	}

	go ackProgressWatchdog(watchdogCtx, msg, watchdogInterval)

	var body map[string]any

	if err := json.Unmarshal(msg.Data, &body); err != nil {
		return fmt.Errorf("decode request json: %w", err)
	}

	geometryData, zLevels, zPatch, taskUUID, areaID, layerID, err := geometry.ReadGeoJSONRequest(body)
	if err != nil {
		return fmt.Errorf("read geojson request: %w", err)
	}

	patchTiles, err := geometry.GetBelongingTiles(geometryData, zPatch)
	if err != nil {
		return fmt.Errorf("get patch tiles for z_patch=%d: %w", zPatch, err)
	}

	patches, err := geometry.GetPatches(geometryData, patchTiles, 0)
	if err != nil {
		return fmt.Errorf("get patches: %w", err)
	}

	patchMeta := make([]geometry.PatchMeta, 0, len(patches))

	for patchName, patchGeometry := range patches {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		tilesByZoom, totalTiles := geometry.ComputeTilesByZoom(
			patchGeometry,
			zLevels,
			patchName,
		)

		patchID := geometry.MakePatchID(taskUUID, patchName)

		patchMeta = append(patchMeta, geometry.PatchMeta{
			Name:        patchName,
			Geometry:    patchGeometry,
			PatchUUID:   patchID,
			TilesByZoom: tilesByZoom,
			TotalTiles:  totalTiles,
		})

		job := geometry.PatchJob{
			Name:        patchName,
			TaskUUID:    taskUUID,
			AreaID:      areaID,
			LayerID:     layerID,
			TilesByZoom: tilesByZoom,
			TotalTiles:  totalTiles,
		}

		if err := geometry.ProcessPatch(ctx, cfg, resources, job); err != nil {
			return fmt.Errorf("process patch %s: %w", patchName, err)
		}
	}

	featureCollection := geometry.BuildTaskFeatureCollection(patchMeta)

	if err := geometry.PublishTaskGeometry(resources, taskUUID, featureCollection); err != nil {
		return fmt.Errorf("publish task geometry: %w", err)
	}

	log.Printf("request message processed successfully: taskUUID=%s patches=%d", taskUUID, len(patches))

	return nil
}

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"geo-worker-go/internal/config"
	"geo-worker-go/internal/geometry"
	"geo-worker-go/internal/natsclient"
	"github.com/nats-io/nats.go"
)

func HandleRequestMessage(
	ctx context.Context,
	cfg config.Config,
	resources *natsclient.NATSResources,
	msg *nats.Msg,
) error {
	slog.Info(
		"received request message",
		"subject", msg.Subject,
		"size_bytes", len(msg.Data),
	)

	watchdogCtx, stopWatchdog := context.WithCancel(ctx)
	defer stopWatchdog()

	watchdogInterval := time.Duration(cfg.Ack_Wait) * time.Second / 2
	if watchdogInterval <= 0 {
		watchdogInterval = time.Second
	}

	go ackProgressWatchdog(watchdogCtx, msg, watchdogInterval)

	var body map[string]any

	err := json.Unmarshal(msg.Data, &body)
	if err != nil {
		return fmt.Errorf("decode request json: %w", err)
	}

	geometryData, zLevels, zPatch, taskUUID, areaID, layerID, err := geometry.ReadGeoJSONRequest(
		body,
	)
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
			return fmt.Errorf("process patch context done: %w", ctx.Err())
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

		err := geometry.ProcessPatch(ctx, cfg, resources, job)
		if err != nil {
			return fmt.Errorf("process patch %s: %w", patchName, err)
		}
	}

	featureCollection := geometry.BuildTaskFeatureCollection(patchMeta)

	err = geometry.PublishTaskGeometry(resources, taskUUID, featureCollection)
	if err != nil {
		return fmt.Errorf("publish task geometry: %w", err)
	}

	slog.Info(
		"request message processed successfully",
		"task_uuid", taskUUID,
		"patches", len(patches),
	)

	return nil
}

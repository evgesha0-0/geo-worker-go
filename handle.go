package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

func handleRequestMessage(
	ctx context.Context,
	cfg Config,
	resources *NATSResources,
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

	geometry, zLevels, zPatch, taskUUID, areaID, layerID, err := readGeoJSONRequest(body)
	if err != nil {
		return fmt.Errorf("read geojson request: %w", err)
	}

	patchTiles, err := getBelongingTiles(geometry, zPatch)
	if err != nil {
		return fmt.Errorf("get patch tiles for z_patch=%d: %w", zPatch, err)
	}

	patches, err := getPatches(geometry, patchTiles, 0)
	if err != nil {
		return fmt.Errorf("get patches: %w", err)
	}

	patchMeta := make([]PatchMeta, 0, len(patches))

	for patchName, patchGeometry := range patches {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		tilesByZoom, totalTiles := computeTilesByZoom(
			patchGeometry,
			zLevels,
			patchName,
		)

		patchID := makePatchID(taskUUID, patchName)

		patchMeta = append(patchMeta, PatchMeta{
			Name:        patchName,
			Geometry:    patchGeometry,
			PatchUUID:   patchID,
			TilesByZoom: tilesByZoom,
			TotalTiles:  totalTiles,
		})

		job := PatchJob{
			Name:        patchName,
			TaskUUID:    taskUUID,
			AreaID:      areaID,
			LayerID:     layerID,
			TilesByZoom: tilesByZoom,
			TotalTiles:  totalTiles,
		}

		if err := processPatch(ctx, cfg, resources, job); err != nil {
			return fmt.Errorf("process patch %s: %w", patchName, err)
		}
	}

	featureCollection := buildTaskFeatureCollection(patchMeta)

	if err := publishTaskGeometry(resources, taskUUID, featureCollection); err != nil {
		return fmt.Errorf("publish task geometry: %w", err)
	}

	log.Printf("request message processed successfully: taskUUID=%s patches=%d", taskUUID, len(patches))

	return nil
}

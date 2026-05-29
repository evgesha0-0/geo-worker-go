package geometry

import (
	"encoding/json"
	"fmt"

	"geo-worker-go/internal/models"
	"geo-worker-go/internal/natsclient"
	"log/slog"
	"sync"
)

type PatchMeta struct {
	Name        string
	Geometry    any
	PatchUUID   string
	TilesByZoom map[string][]models.Tile
	TotalTiles  int
}

func ComputeTilesByZoom(
	geometryData any,
	zLevels []int,
	patchName string,
) (map[string][]models.Tile, int) {
	tilesByZoom := make(map[string][]models.Tile)
	totalTiles := 0

	var mutex sync.Mutex
	var waitGroup sync.WaitGroup

	for _, zoom := range zLevels {
		zoom := zoom

		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()

			tiles, err := GetBelongingTiles(geometryData, zoom)
			if err != nil {
				slog.Error(
					"get belonging tiles failed",
					"zoom", zoom,
					"patch_name", patchName,
					"error", err,
				)

				mutex.Lock()
				tilesByZoom[fmt.Sprintf("%d", zoom)] = []models.Tile{}
				mutex.Unlock()

				return
			}

			serializedTiles := make([]models.Tile, 0, len(tiles))

			for _, tile := range tiles {
				serializedTiles = append(serializedTiles, SerializeTile(tile))
			}

			mutex.Lock()
			tilesByZoom[fmt.Sprintf("%d", zoom)] = serializedTiles
			totalTiles += len(serializedTiles)
			mutex.Unlock()
		}()
	}

	waitGroup.Wait()

	return tilesByZoom, totalTiles
}

func BuildTaskFeatureCollection(patchMeta []PatchMeta) models.FeatureCollection {
	features := make([]models.Feature, 0, len(patchMeta))

	for _, meta := range patchMeta {
		feature := models.Feature{
			Type:     "Feature",
			Geometry: meta.Geometry,
			Properties: models.FeatureProperties{
				PatchName:  meta.Name,
				PatchUUID:  meta.PatchUUID,
				TotalTiles: meta.TotalTiles,
			},
		}

		features = append(features, feature)
	}

	return models.FeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}
}

func PublishTaskGeometry(
	resources *natsclient.NATSResources,
	taskUUID string,
	featureCollection models.FeatureCollection,
) error {
	payload, err := json.Marshal(featureCollection)
	if err != nil {
		return fmt.Errorf("marshal task feature collection: %w", err)
	}

	_, err = resources.ObjectStore.PutBytes(taskUUID, payload)
	if err != nil {
		return fmt.Errorf("put task geometry to object store key=%s: %w", taskUUID, err)
	}

	slog.Info(
		"published task geometry to object store",
		"task_uuid", taskUUID,
		"features", len(featureCollection.Features),
		"size_bytes", len(payload),
	)

	return nil
}

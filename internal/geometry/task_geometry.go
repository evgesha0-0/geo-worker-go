package geometry

import (
	"encoding/json"
	"fmt"

	"geo-worker-go/internal/models"
	"geo-worker-go/internal/natsclient"
	"log"
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
				log.Printf("getBelongingTiles failed for zoom=%d in patch=%s: %v", zoom, patchName, err)

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

	log.Printf(
		"Published task geometry to object store key=%s features=%d size=%d",
		taskUUID,
		len(featureCollection.Features),
		len(payload),
	)

	return nil
}

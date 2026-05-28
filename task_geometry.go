package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
)

type PatchMeta struct {
	Name        string
	Geometry    any
	PatchUUID   string
	TilesByZoom map[string][]Tile
	TotalTiles  int
}

func computeTilesByZoom(
	geometry any,
	zLevels []int,
	patchName string,
) (map[string][]Tile, int) {
	tilesByZoom := make(map[string][]Tile)
	totalTiles := 0

	var mutex sync.Mutex
	var waitGroup sync.WaitGroup

	for _, zoom := range zLevels {
		zoom := zoom

		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()

			tiles, err := getBelongingTiles(geometry, zoom)
			if err != nil {
				log.Printf("getBelongingTiles failed for zoom=%d in patch=%s: %v", zoom, patchName, err)

				mutex.Lock()
				tilesByZoom[fmt.Sprintf("%d", zoom)] = []Tile{}
				mutex.Unlock()

				return
			}

			serializedTiles := make([]Tile, 0, len(tiles))

			for _, tile := range tiles {
				serializedTiles = append(serializedTiles, serializeTile(tile))
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

func buildTaskFeatureCollection(patchMeta []PatchMeta) FeatureCollection {
	features := make([]Feature, 0, len(patchMeta))

	for _, meta := range patchMeta {
		feature := Feature{
			Type:     "Feature",
			Geometry: meta.Geometry,
			Properties: FeatureProperties{
				PatchName:  meta.Name,
				PatchUUID:  meta.PatchUUID,
				TotalTiles: meta.TotalTiles,
			},
		}

		features = append(features, feature)
	}

	return FeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}
}

func publishTaskGeometry(
	resources *NATSResources,
	taskUUID string,
	featureCollection FeatureCollection,
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

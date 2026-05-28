package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

type PatchJob struct {
	Name        string
	TaskUUID    string
	AreaID      int
	LayerID     int
	TilesByZoom map[string][]Tile
	TotalTiles  int
}

func makePatchID(taskUUID string, patchName string) string {
	return uuid.NewSHA1(
		uuid.NameSpaceDNS,
		[]byte(fmt.Sprintf("%s:%s", taskUUID, patchName)),
	).String()
}

func processPatch(
	ctx context.Context,
	cfg Config,
	resources *NATSResources,
	job PatchJob,
) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	patchID := makePatchID(job.TaskUUID, job.Name)

	eventID, err := uuid.NewUUID()
	if err != nil {
		eventID = uuid.New()
	}

	patchMsg := PatchMessage{
		Name:        job.Name,
		TilesByZoom: job.TilesByZoom,
		AreaID:      job.AreaID,
		LayerID:     job.LayerID,
		TaskUUID:    job.TaskUUID,
		PatchUUID:   patchID,
	}

	progressMsg := ProgressMessage{
		EventID:        eventID.String(),
		TaskID:         job.TaskUUID,
		PatchID:        patchID,
		Status:         "pending",
		CompletedTiles: 0,
		TotalTiles:     job.TotalTiles,
		ErrorTiles:     0,
		Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
	}

	if err := publishPatch(resources.JS, cfg, patchMsg); err != nil {
		return fmt.Errorf("publish patch %s: %w", job.Name, err)
	}

	log.Printf("Published patch %s", job.Name)

	if err := publishProgress(resources.JS, cfg, progressMsg); err != nil {
		return fmt.Errorf("publish progress for patch %s: %w", job.Name, err)
	}

	log.Printf("Published progress for patch %s", job.Name)

	return nil
}

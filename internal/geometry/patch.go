package geometry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"geo-worker-go/internal/config"
	"geo-worker-go/internal/models"
	"geo-worker-go/internal/natsclient"
	"github.com/google/uuid"
)

type PatchJob struct {
	Name        string
	TaskUUID    string
	AreaID      int
	LayerID     int
	TilesByZoom map[string][]models.Tile
	TotalTiles  int
}

func MakePatchID(taskUUID string, patchName string) string {
	return uuid.NewSHA1(
		uuid.NameSpaceDNS,
		fmt.Appendf(nil, "%s:%s", taskUUID, patchName),
	).String()
}

func ProcessPatch(
	ctx context.Context,
	cfg config.Config,
	resources *natsclient.NATSResources,
	job PatchJob,
) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("handle request context done: %w", ctx.Err())
	default:
	}

	patchID := MakePatchID(job.TaskUUID, job.Name)

	eventID, err := uuid.NewUUID()
	if err != nil {
		eventID = uuid.New()
	}

	patchMsg := models.PatchMessage{
		Name:        job.Name,
		TilesByZoom: job.TilesByZoom,
		AreaID:      job.AreaID,
		LayerID:     job.LayerID,
		TaskUUID:    job.TaskUUID,
		PatchUUID:   patchID,
	}

	progressMsg := models.ProgressMessage{
		EventID:        eventID.String(),
		TaskID:         job.TaskUUID,
		PatchID:        patchID,
		Status:         "pending",
		CompletedTiles: 0,
		TotalTiles:     job.TotalTiles,
		ErrorTiles:     0,
		Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
	}

	err = natsclient.PublishPatch(resources.JS, cfg, patchMsg)
	if err != nil {
		return fmt.Errorf("publish patch %s: %w", job.Name, err)
	}

	slog.Info("Published patch", "patch_name", job.Name)

	err = natsclient.PublishProgress(resources.JS, cfg, progressMsg)
	if err != nil {
		return fmt.Errorf("publish progress for patch %s: %w", job.Name, err)
	}

	slog.Info("Published progress for patch", "patch_name", job.Name)

	return nil
}

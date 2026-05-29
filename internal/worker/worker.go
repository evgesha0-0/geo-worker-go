package worker

import (
	"context"
	"errors"
	"fmt"
	"geo-worker-go/internal/config"
	"geo-worker-go/internal/natsclient"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/nats-io/nats.go"
)

func StartWorker(ctx context.Context, cfg config.Config, resources *natsclient.NATSResources) error {
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(cfg.Concurrency)

	group.Go(func() error {
		return startAdvisoryLoop(groupCtx, cfg, resources)
	})

	for {
		select {
		case <-groupCtx.Done():
			slog.Info("worker shutdown requested")
			return group.Wait()

		default:
		}

		messages, err := resources.RequestSub.Fetch(
			cfg.Batch,
			nats.MaxWait(1*time.Second),
		)

		if err != nil {
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}

			return fmt.Errorf("fetch request messages: %w", err)
		}

		for _, msg := range messages {
			message := msg

			group.Go(func() error {
				if err := HandleRequestMessage(groupCtx, cfg, resources, message); err != nil {
					slog.Error("handle request message failed", "error", err)

					if nakErr := message.Nak(); nakErr != nil {
						slog.Error("nak message failed", "error", nakErr)
					}

					return nil
				}

				if ackErr := message.Ack(); ackErr != nil {
					slog.Error("ack message failed", "error", ackErr)
				}

				return nil
			})
		}
	}
}

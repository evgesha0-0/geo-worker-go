package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"geo-worker-go/internal/config"
	"geo-worker-go/internal/natsclient"
	"github.com/nats-io/nats.go"
	"golang.org/x/sync/errgroup"
)

func StartWorker(
	ctx context.Context,
	cfg config.Config,
	resources *natsclient.NATSResources,
) error {
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(cfg.Concurrency)

	group.Go(func() error {
		return startAdvisoryLoop(groupCtx, cfg, resources)
	})

	for {
		select {
		case <-groupCtx.Done():
			slog.Info("worker shutdown requested")

			err := group.Wait()
			if err != nil {
				return fmt.Errorf("wait worker group: %w", err)
			}

			return nil

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
				err := HandleRequestMessage(groupCtx, cfg, resources, message)
				if err != nil {
					slog.Error("handle request message failed", "error", err)

					nakErr := message.Nak()
					if nakErr != nil {
						slog.Error("nak message failed", "error", nakErr)
					}

					return nil
				}

				ackErr := message.Ack()
				if ackErr != nil {
					slog.Error("ack message failed", "error", ackErr)
				}

				return nil
			})
		}
	}
}

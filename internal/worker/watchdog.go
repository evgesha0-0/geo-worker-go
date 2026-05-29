package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

func ackProgressWatchdog(
	ctx context.Context,
	msg *nats.Msg,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("ack watchdog stopped")

			return

		case <-ticker.C:
			err := msg.InProgress()
			if err != nil {
				slog.Warn("send in_progress ack failed", "error", err)

				return
			}

			slog.Info("sent in_progress ack")
		}
	}
}

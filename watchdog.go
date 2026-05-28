package main

import (
	"context"
	"log"
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
			log.Println("ack watchdog stopped")
			return

		case <-ticker.C:
			if err := msg.InProgress(); err != nil {
				log.Printf("send in_progress ack failed: %v", err)
				return
			}

			log.Println("sent in_progress ack")
		}
	}
}

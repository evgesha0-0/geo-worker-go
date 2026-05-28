package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

func StartWorker(ctx context.Context, cfg Config, resources *NATSResources) error {
	semaphore := make(chan struct{}, cfg.Concurrency)

	var wg sync.WaitGroup

	advisoryErrCh := make(chan error, 1)

	go func() {
		advisoryErrCh <- startAdvisoryLoop(ctx, cfg, resources)
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("worker shutdown requested")
			wg.Wait()
			return nil

		case err := <-advisoryErrCh:
			if err != nil {
				return fmt.Errorf("advisory loop failed: %w", err)
			}

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
			semaphore <- struct{}{}
			wg.Add(1)

			go func(message *nats.Msg) {
				defer wg.Done()
				defer func() {
					<-semaphore
				}()

				if err := handleRequestMessage(ctx, cfg, resources, message); err != nil {
					log.Printf("handle request message failed: %v", err)

					if nakErr := message.Nak(); nakErr != nil {
						log.Printf("nak message failed: %v", nakErr)
					}

					return
				}

				if ackErr := message.Ack(); ackErr != nil {
					log.Printf("ack message failed: %v", ackErr)
				}
			}(msg)
		}
	}
}

package natsclient

import (
	"encoding/json"
	"fmt"

	"geo-worker-go/internal/config"
	"github.com/nats-io/nats.go"
)

func PublishJSON(
	jetStream nats.JetStreamContext,
	subject string,
	payload any,
) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal message for subject %s: %w", subject, err)
	}

	_, err = jetStream.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("publish message to subject %s: %w", subject, err)
	}

	return nil
}

func PublishPatch(
	js nats.JetStreamContext,
	cfg config.Config,
	patchMsg any,
) error {
	return PublishJSON(js, cfg.Subject_Patch, patchMsg)
}

func PublishProgress(
	js nats.JetStreamContext,
	cfg config.Config,
	progressMsg any,
) error {
	return PublishJSON(js, cfg.Subject_Progress, progressMsg)
}

func PublishDLQ(
	js nats.JetStreamContext,
	cfg config.Config,
	dlqPayload any,
) error {
	return PublishJSON(js, cfg.Subject_DLQ, dlqPayload)
}

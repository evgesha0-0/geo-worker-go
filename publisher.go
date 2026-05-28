package main

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

func publishJSON(
	js nats.JetStreamContext,
	subject string,
	payload any,
) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal message for subject %s: %w", subject, err)
	}

	if _, err := js.Publish(subject, data); err != nil {
		return fmt.Errorf("publish message to subject %s: %w", subject, err)
	}

	return nil
}

func publishPatch(
	js nats.JetStreamContext,
	cfg Config,
	patchMsg any,
) error {
	return publishJSON(js, cfg.Subject_Patch, patchMsg)
}

func publishProgress(
	js nats.JetStreamContext,
	cfg Config,
	progressMsg any,
) error {
	return publishJSON(js, cfg.Subject_Progress, progressMsg)
}

func publishDLQ(
	js nats.JetStreamContext,
	cfg Config,
	dlqPayload any,
) error {
	return publishJSON(js, cfg.Subject_DLQ, dlqPayload)
}

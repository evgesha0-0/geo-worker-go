package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"geo-worker-go/internal/config"
	"geo-worker-go/internal/models"
	"geo-worker-go/internal/natsclient"
	"github.com/nats-io/nats.go"
)

func startAdvisoryLoop(
	ctx context.Context,
	cfg config.Config,
	resources *natsclient.NATSResources,
) error {
	slog.Info("advisory loop started")

	for {
		rawMsg, err := resources.AdvisorySub.NextMsgWithContext(ctx)
		if err != nil {
			ctxErr := ctx.Err()
			if ctxErr != nil {
				slog.Info("advisory loop stopped")

				return fmt.Errorf("advisory loop stopped: %w", ctxErr)
			}

			return fmt.Errorf("read advisory message: %w", err)
		}

		go func(msg *nats.Msg) {
			err = handleAdvisoryMessage(cfg, resources, msg)
			if err != nil {
				slog.Error("handle advisory message failed", "error", err)
			}
		}(rawMsg)
	}
}

func handleAdvisoryMessage(
	cfg config.Config,
	resources *natsclient.NATSResources,
	rawMsg *nats.Msg,
) error {
	var advisory map[string]any

	err := json.Unmarshal(rawMsg.Data, &advisory)
	if err != nil {
		return fmt.Errorf("decode advisory json: %w", err)
	}

	stream := firstString(
		advisory["stream"],
		advisory["stream_name"],
	)

	consumer := firstAny(
		advisory["consumer"],
		advisory["consumer_name"],
		getNestedValue(advisory, "consumer_info", "name"),
	)

	streamSeq, ok := firstUint64(
		advisory["stream_seq"],
		advisory["stream_sequence"],
		advisory["stream_seqno"],
	)

	dlqRecord := models.DLQMessage{
		Advisory: advisory,
		Original: nil,
		Consumer: consumer,
	}

	if stream != "" && ok {
		rawOriginal, err := resources.JS.GetMsg(stream, streamSeq)
		if err != nil {
			slog.Error(
				"failed to fetch original message for DLQ advisory",
				"stream", stream,
				"seq", streamSeq,
				"error", err,
			)
		} else if rawOriginal != nil {
			dlqRecord.Original = string(rawOriginal.Data)
		}
	}

	err = natsclient.PublishDLQ(resources.JS, cfg, dlqRecord)
	if err != nil {
		return fmt.Errorf("publish dlq record: %w", err)
	}

	slog.Info(
		"published DLQ record",
		"stream", stream,
		"seq", streamSeq,
	)

	return nil
}

func firstString(values ...any) string {
	for _, value := range values {
		if str, ok := value.(string); ok && str != "" {
			return str
		}
	}

	return ""
}

func firstAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}

	return nil
}

func getNestedValue(source map[string]any, objectKey string, valueKey string) any {
	rawObject, ok := source[objectKey]
	if !ok {
		return nil
	}

	object, ok := rawObject.(map[string]any)
	if !ok {
		return nil
	}

	return object[valueKey]
}

func firstUint64(values ...any) (uint64, bool) {
	for _, value := range values {
		switch typedValue := value.(type) {
		case float64:
			if typedValue > 0 {
				return uint64(typedValue), true
			}

		case int:
			if typedValue > 0 {
				return uint64(typedValue), true
			}

		case int64:
			if typedValue > 0 {
				return uint64(typedValue), true
			}

		case uint64:
			if typedValue > 0 {
				return typedValue, true
			}

		case string:
			parsed, err := strconv.ParseUint(typedValue, 10, 64)
			if err == nil && parsed > 0 {
				return parsed, true
			}
		}
	}

	return 0, false
}

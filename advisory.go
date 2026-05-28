package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/nats-io/nats.go"
)

func startAdvisoryLoop(
	ctx context.Context,
	cfg Config,
	resources *NATSResources,
) error {
	log.Println("advisory loop started")

	for {
		rawMsg, err := resources.AdvisorySub.NextMsgWithContext(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("advisory loop stopped")
				return nil
			}

			return fmt.Errorf("read advisory message: %w", err)
		}

		go func(msg *nats.Msg) {
			if err := handleAdvisoryMessage(cfg, resources, msg); err != nil {
				log.Printf("handle advisory message failed: %v", err)
			}
		}(rawMsg)
	}
}

func handleAdvisoryMessage(
	cfg Config,
	resources *NATSResources,
	rawMsg *nats.Msg,
) error {
	var advisory map[string]any

	if err := json.Unmarshal(rawMsg.Data, &advisory); err != nil {
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

	dlqRecord := DLQMessage{
		Advisory: advisory,
		Original: nil,
		Consumer: consumer,
	}

	if stream != "" && ok {
		rawOriginal, err := resources.JS.GetMsg(stream, streamSeq)
		if err != nil {
			log.Printf("failed to fetch original message for DLQ advisory: stream=%s seq=%d err=%v", stream, streamSeq, err)
		} else if rawOriginal != nil {
			dlqRecord.Original = string(rawOriginal.Data)
		}
	}

	if err := publishDLQ(resources.JS, cfg, dlqRecord); err != nil {
		return fmt.Errorf("publish dlq record: %w", err)
	}

	log.Printf("Published DLQ record for stream=%s seq=%d", stream, streamSeq)

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

package natsclient

import (
	"errors"
	"fmt"
	"geo-worker-go/internal/config"
	"time"

	"github.com/nats-io/nats.go"
)

type NATSResources struct {
	Conn        *nats.Conn
	JS          nats.JetStreamContext
	ObjectStore nats.ObjectStore
	RequestSub  *nats.Subscription
	AdvisorySub *nats.Subscription
}

func ConnectNATS(cfg config.Config) (*NATSResources, error) {
	conn, err := nats.Connect(
		cfg.NATS_URL,
		nats.UserInfo(cfg.NATS_User, cfg.NATS_Pass),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create JetStream context: %w", err)
	}

	if err := ensureStream(js, cfg.Stream_Req, cfg.Subject_Req); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ensure request stream: %w", err)
	}

	if err := ensureStream(js, cfg.Stream_Patches, cfg.Subject_Patch); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ensure patches stream: %w", err)
	}

	if err := ensureStream(js, cfg.Stream_Progress, cfg.Subject_Progress); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ensure progress stream: %w", err)
	}

	if err := ensureStream(js, cfg.Stream_DLQ, cfg.Subject_DLQ); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ensure dlq stream: %w", err)
	}

	objectStore, err := js.ObjectStore(cfg.Object_Store_Bucket)
	if err != nil {
		objectStore, err = js.CreateObjectStore(&nats.ObjectStoreConfig{
			Bucket: cfg.Object_Store_Bucket,
		})
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("create object store bucket %s: %w", cfg.Object_Store_Bucket, err)
		}
	}

	requestSub, err := js.PullSubscribe(
		cfg.Subject_Req,
		cfg.Durable_Name,
		nats.BindStream(cfg.Stream_Req),
		nats.ManualAck(),
		nats.AckWait(time.Duration(cfg.Ack_Wait)*time.Second),
	)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create pull subscription: %w", err)
	}

	advisorySubject := fmt.Sprintf(
		"$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.%s.>",
		cfg.Stream_Req,
	)

	advisorySub, err := conn.SubscribeSync(advisorySubject)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create advisory subscription: %w", err)
	}

	return &NATSResources{
		Conn:        conn,
		JS:          js,
		ObjectStore: objectStore,
		RequestSub:  requestSub,
		AdvisorySub: advisorySub,
	}, nil
}

func ensureStream(js nats.JetStreamContext, streamName string, subject string) error {
	_, err := js.StreamInfo(streamName)
	if err == nil {
		return nil
	}

	if !errors.Is(err, nats.ErrStreamNotFound) {
		return fmt.Errorf("get stream info %s: %w", streamName, err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{subject},
	})

	if err != nil {
		return fmt.Errorf("create stream %s: %w", streamName, err)
	}

	return nil
}

func (r *NATSResources) Close() {
	if r == nil || r.Conn == nil {
		return
	}

	_ = r.Conn.Drain()
	r.Conn.Close()
}

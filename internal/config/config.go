package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	NATS_URL  string
	NATS_User string
	NATS_Pass string

	Stream_Req       string
	Subject_Req      string
	Stream_Patches   string
	Subject_Patch    string
	Stream_Progress  string
	Subject_Progress string
	Stream_DLQ       string
	Subject_DLQ      string

	Object_Store_Bucket string

	Durable_Name string
	Batch        int
	Concurrency  int
	Ack_Wait     int
	Max_Deliver  int
}

func requiredEnv(name string) (string, error) {
	value := os.Getenv(name)

	if value == "" {
		return "", fmt.Errorf("%s is not set", name)
	}

	return value, nil
}

func requiredIntEnv(name string) (int, error) {
	value, err := requiredEnv(name)
	if err != nil {
		return 0, err
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be integer: %w", name, err)
	}

	return result, nil
}

func LoadConfig() (Config, error) {
	natsURL, err := requiredEnv("NATS_URL")
	if err != nil {
		return Config{}, err
	}

	natsUser, err := requiredEnv("NATS_USER")
	if err != nil {
		return Config{}, err
	}

	natsPass, err := requiredEnv("NATS_PASS")
	if err != nil {
		return Config{}, err
	}

	streamReq, err := requiredEnv("STREAM_REQ")
	if err != nil {
		return Config{}, err
	}

	subjectReq, err := requiredEnv("SUBJECT_REQ")
	if err != nil {
		return Config{}, err
	}

	streamPatches, err := requiredEnv("STREAM_PATCHES")
	if err != nil {
		return Config{}, err
	}

	subjectPatch, err := requiredEnv("SUBJECT_PATCH")
	if err != nil {
		return Config{}, err
	}

	streamProgress, err := requiredEnv("STREAM_PROGRESS")
	if err != nil {
		return Config{}, err
	}

	subjectProgress, err := requiredEnv("SUBJECT_PROGRESS")
	if err != nil {
		return Config{}, err
	}

	streamDLQ, err := requiredEnv("STREAM_DLQ")
	if err != nil {
		return Config{}, err
	}

	subjectDLQ, err := requiredEnv("SUBJECT_DLQ")
	if err != nil {
		return Config{}, err
	}

	objectStoreBucket, err := requiredEnv("OBJECT_STORE_BUCKET")
	if err != nil {
		return Config{}, err
	}

	durableName, err := requiredEnv("DURABLE_NAME")
	if err != nil {
		return Config{}, err
	}

	batch, err := requiredIntEnv("BATCH")
	if err != nil {
		return Config{}, err
	}

	concurrency, err := requiredIntEnv("CONCURRENCY")
	if err != nil {
		return Config{}, err
	}

	ackWait, err := requiredIntEnv("ACK_WAIT")
	if err != nil {
		return Config{}, err
	}

	maxDeliver, err := requiredIntEnv("MAX_DELIVER")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		NATS_URL:            natsURL,
		NATS_User:           natsUser,
		NATS_Pass:           natsPass,
		Stream_Req:          streamReq,
		Subject_Req:         subjectReq,
		Stream_Patches:      streamPatches,
		Subject_Patch:       subjectPatch,
		Stream_Progress:     streamProgress,
		Subject_Progress:    subjectProgress,
		Stream_DLQ:          streamDLQ,
		Subject_DLQ:         subjectDLQ,
		Object_Store_Bucket: objectStoreBucket,
		Durable_Name:        durableName,
		Batch:               batch,
		Concurrency:         concurrency,
		Ack_Wait:            ackWait,
		Max_Deliver:         maxDeliver,
	}

	return cfg, nil
}

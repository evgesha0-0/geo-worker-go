package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"geo-worker-go/internal/config"
	"geo-worker-go/internal/natsclient"
	"geo-worker-go/internal/worker"
	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   false,
		Level:       slog.LevelInfo,
		ReplaceAttr: nil,
	}))
	slog.SetDefault(logger)

	err := godotenv.Load()
	if err != nil {
		slog.Warn(".env file not found", "error", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("load config failed", "error", err)

		return
	}

	resources, err := natsclient.ConnectNATS(cfg)
	if err != nil {
		slog.Error("connect NATS failed", "error", err)

		return
	}
	defer resources.Close()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	slog.Info("NATS connection, JetStream, Object Store and pull subscription are ready")

	err = worker.StartWorker(ctx, cfg, resources)
	if err != nil {
		slog.Error("worker failed", "error", err)

		return
	}
}

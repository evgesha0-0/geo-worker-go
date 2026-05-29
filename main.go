package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"geo-worker-go/internal/config"
	"geo-worker-go/internal/natsclient"
	"geo-worker-go/internal/worker"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := godotenv.Load(); err != nil {
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

	if err := worker.StartWorker(ctx, cfg, resources); err != nil {
		slog.Error("worker failed", "error", err)
		return
	}
}

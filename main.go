package main

import (
	"context"
	"geo-worker-go/internal/config"
	"geo-worker-go/internal/natsclient"
	"geo-worker-go/internal/worker"

	"log"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env file not found")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	resources, err := natsclient.ConnectNATS(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer resources.Close()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	log.Println("NATS connection, JetStream, Object Store and pull subscription are ready")

	if err := worker.StartWorker(ctx, cfg, resources); err != nil {
		log.Fatal(err)
	}
}

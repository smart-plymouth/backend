package main

import (
	"log"

	"github.com/hibiken/asynq"

	"github.com/smartplymouth/backend/internal/config"
	"github.com/smartplymouth/backend/internal/database"
	"github.com/smartplymouth/backend/internal/tasks"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr()},
		asynq.Config{
			Concurrency: cfg.WorkerConcurrency,
			Queues: map[string]int{
				"default": 1,
			},
		},
	)

	mux := asynq.NewServeMux()
	tasks.RegisterHandlers(mux, db, cfg)

	log.Printf("Starting worker with concurrency=%d", cfg.WorkerConcurrency)
	if err := srv.Run(mux); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}

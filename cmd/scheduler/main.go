package main

import (
	"log"

	"github.com/hibiken/asynq"

	"github.com/smartplymouth/backend/internal/config"
	"github.com/smartplymouth/backend/internal/tasks"
)

func main() {
	cfg := config.Load()

	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr()},
		nil,
	)

	// Fetch ED wait times every 5 minutes
	task := asynq.NewTask(tasks.TypeFetchWaitTimes, nil)
	if _, err := scheduler.Register("@every 5m", task); err != nil {
		log.Fatalf("Failed to register fetch_wait_times schedule: %v", err)
	}

	// Fetch air quality data every hour
	aqTask := asynq.NewTask(tasks.TypeFetchAirQuality, nil)
	if _, err := scheduler.Register("@every 1h", aqTask); err != nil {
		log.Fatalf("Failed to register fetch_air_quality schedule: %v", err)
	}

	log.Println("Starting scheduler")
	if err := scheduler.Run(); err != nil {
		log.Fatalf("Scheduler failed: %v", err)
	}
}

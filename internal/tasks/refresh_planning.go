package tasks

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/smartplymouth/backend/internal/config"
)

func NewRefreshPlanningHandler(db *gorm.DB, cfg *config.Config) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		today := time.Now()
		twoYearsAgo := today.AddDate(0, 0, -730)

		// Find the first Monday on or after twoYearsAgo
		daysUntilMonday := (8 - int(twoYearsAgo.Weekday())) % 7
		if twoYearsAgo.Weekday() == time.Monday {
			daysUntilMonday = 0
		}
		currentMonday := twoYearsAgo.AddDate(0, 0, daysUntilMonday)

		// We need a client to enqueue tasks - create one from config
		client := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr()})
		defer client.Close()

		weeksQueued := 0
		for !currentMonday.After(today) {
			payload, _ := json.Marshal(fetchWeeklyPayload{
				WeekStartISO: currentMonday.Format("2006-01-02"),
			})
			task := asynq.NewTask(TypeFetchWeeklyPlanning, payload)
			if _, err := client.Enqueue(task); err != nil {
				log.Printf("Failed to enqueue fetch for %s: %v",
					currentMonday.Format("2006-01-02"), err)
			}
			currentMonday = currentMonday.AddDate(0, 0, 7)
			weeksQueued++
		}

		log.Printf("Queued %d weekly fetch tasks for the last 2 years", weeksQueued)
		return nil
	}
}

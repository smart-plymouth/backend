package tasks

import (
	"context"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/smartplymouth/backend/internal/config"
)

func RegisterHandlers(mux *asynq.ServeMux, db *gorm.DB, cfg *config.Config) {
	mux.HandleFunc(TypeFetchWaitTimes, NewFetchWaitTimesHandler(db))
	mux.HandleFunc(TypeFetchWeeklyPlanning, NewFetchWeeklyPlanningHandler(db, cfg))
	mux.HandleFunc(TypeRefreshPlanningApplications, NewRefreshPlanningHandler(db, cfg))
	mux.HandleFunc(TypeAnalysePlanningApplication, NewAnalysePlanningHandler(db, cfg))
	mux.HandleFunc(TypeFetchAirQuality, NewFetchAirQualityHandler(db))
	mux.HandleFunc(TypeExampleTask, func(ctx context.Context, t *asynq.Task) error {
		return nil
	})
}

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/smartplymouth/backend/internal/config"
	"github.com/smartplymouth/backend/internal/database"
	"github.com/smartplymouth/backend/internal/routes"
	"github.com/smartplymouth/backend/internal/taskqueue"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	client := taskqueue.NewClient(cfg.RedisURL)
	defer client.Close()

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	routes.RegisterTestAPI(r, db)
	routes.RegisterEmergencyWaitTimes(r, db)
	routes.RegisterPlanning(r, db, client, cfg)
	routes.RegisterBirdMonitoring(r, db)

	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	log.Printf("Starting API server on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

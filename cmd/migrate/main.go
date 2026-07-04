package main

import (
	"log"
	"os"

	"github.com/smartplymouth/backend/internal/config"
	"github.com/smartplymouth/backend/internal/database"
	"github.com/smartplymouth/backend/internal/migrations"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}

	switch direction {
	case "up":
		if err := migrations.Up(db); err != nil {
			log.Fatalf("Migration up failed: %v", err)
		}
		log.Println("Migrations applied successfully")
	case "down":
		if err := migrations.Down(db); err != nil {
			log.Fatalf("Migration down failed: %v", err)
		}
		log.Println("Last migration reverted")
	default:
		log.Fatalf("Unknown direction: %s (use 'up' or 'down')", direction)
	}
}

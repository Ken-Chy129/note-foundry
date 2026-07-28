package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/config"
	"github.com/Ken-Chy129/note-foundry/backend/internal/platform/database"
)

func main() {
	databaseURL, err := config.LoadDatabaseURL(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := database.ApplyMigrations(ctx, pool); err != nil {
		log.Fatal(err)
	}
	log.Print("database migrations applied")
}

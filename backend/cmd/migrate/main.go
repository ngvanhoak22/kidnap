package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"journey-safety/backend/internal/config"
	"journey-safety/backend/internal/platform/database"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "down" && os.Args[1] != "status") {
		fmt.Fprintln(os.Stderr, "usage: migrate up|down|status")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = database.Migrate(ctx, cfg.DatabaseURL, os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "migration failed; verify database connectivity and migration state")
		os.Exit(1)
	}
}

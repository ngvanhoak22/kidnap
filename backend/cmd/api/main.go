package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"journey-safety/backend/internal/config"
	"journey-safety/backend/internal/platform/database"
	"journey-safety/backend/internal/server"
)

func main() { os.Exit(run()) }
func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid_config", "reason", err.Error())
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	db, err := database.Open(startup, cfg.DatabaseURL, cfg.DBMaxConnections)
	cancel()
	if err != nil {
		logger.Error("database_unavailable")
		return 1
	}
	defer db.Close()
	if cfg.Environment == "production" && !database.SafeApplicationRole(ctx, db) {
		logger.Error("production_database_role_requires_non_admin_permissions")
		return 1
	}
	router, err := server.Router(cfg, db, logger)
	if err != nil {
		logger.Error("router_configuration_failed")
		return 1
	}
	srv := &http.Server{Addr: cfg.Address, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan error, 1)
	go func() {
		logger.Info("server_started", "address", cfg.Address, "environment", cfg.Environment)
		done <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdown, release := context.WithTimeout(context.Background(), 10*time.Second)
		defer release()
		if err := srv.Shutdown(shutdown); err != nil {
			logger.Error("shutdown_timeout")
			srv.Close()
			return 1
		}
		return 0
	case err := <-done:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http_server_failed")
			return 1
		}
		return 0
	}
}

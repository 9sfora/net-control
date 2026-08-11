package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charlesduarte/net-control/backend/internal/acs/command"
	"github.com/charlesduarte/net-control/backend/internal/platform/config"
	"github.com/charlesduarte/net-control/backend/internal/platform/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("worker started", "environment", cfg.Environment)
	scheduler := command.NewScheduler(db, logger, cfg.CommandProcessingTimeout, cfg.CommandRetryIntervals)
	ticker := time.NewTicker(cfg.WorkerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("worker stopped")
			return
		case <-ticker.C:
			result, err := scheduler.Run(ctx)
			if err != nil {
				logger.ErrorContext(ctx, "command scheduler failed", "error", err)
				continue
			}
			if result.Waiting+result.Expired+result.Recovered+result.Failed > 0 {
				logger.InfoContext(ctx, "command maintenance completed",
					"waiting", result.Waiting, "expired", result.Expired,
					"recovered", result.Recovered, "failed", result.Failed)
			}
		}
	}
}

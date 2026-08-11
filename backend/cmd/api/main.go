package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/charlesduarte/net-control/backend/internal/acs/cwmp"
	"github.com/charlesduarte/net-control/backend/internal/core/auth"
	"github.com/charlesduarte/net-control/backend/internal/platform/config"
	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/charlesduarte/net-control/backend/internal/platform/httpserver"
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

	tokens := auth.NewTokenManager(cfg.AuthTokenSecret, cfg.AuthTokenTTL)
	authHandler := auth.NewHandler(auth.NewService(auth.NewPostgresRepository(db), tokens), tokens)
	cwmpHandler := cwmp.NewHandler(cwmp.NewRepository(db, cfg.ACSDefaultTenantSlug), logger)
	hosts := httpserver.VirtualHosts{Application: cfg.ApplicationFQDN, ACS: cfg.ACSFQDN}
	server := httpserver.New(cfg.HTTPAddress, hosts, db, logger, authHandler.Register, cwmpHandler.Register)
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	logger.Info("api started", "address", cfg.HTTPAddress, "environment", cfg.Environment)

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api stopped unexpectedly", "error", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

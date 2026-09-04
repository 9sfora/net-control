package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/charlesduarte/net-control/backend/internal/acs/catalog"
	acscommand "github.com/charlesduarte/net-control/backend/internal/acs/command"
	"github.com/charlesduarte/net-control/backend/internal/acs/cwmp"
	"github.com/charlesduarte/net-control/backend/internal/acs/dashboard"
	"github.com/charlesduarte/net-control/backend/internal/acs/firmware"
	acsrouter "github.com/charlesduarte/net-control/backend/internal/acs/router"
	"github.com/charlesduarte/net-control/backend/internal/core/auth"
	"github.com/charlesduarte/net-control/backend/internal/platform/config"
	"github.com/charlesduarte/net-control/backend/internal/platform/database"
	"github.com/charlesduarte/net-control/backend/internal/platform/httpserver"
	"github.com/charlesduarte/net-control/backend/internal/platform/security"
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
	wifiCipher, err := security.NewCipher(cfg.WiFiEncryptionKey)
	if err != nil {
		logger.Error("Wi-Fi encryption initialization failed", "error", err)
		os.Exit(1)
	}
	authHandler := auth.NewHandler(auth.NewService(auth.NewPostgresRepository(db), tokens), tokens)
	routerRepository := acsrouter.NewRepository(db)
	routerHandler := acsrouter.NewHandler(routerRepository, tokens)
	operationsHandler := acsrouter.NewOperationsHandler(routerRepository, tokens, cfg.ACSOnlineTimeout, cfg.ACSOfflineTimeout)
	configurationHandler := acsrouter.NewConfigurationHandler(routerRepository, tokens, wifiCipher)
	commandHandler := acscommand.NewHandler(acscommand.NewRepository(db), tokens)
	dashboardHandler := dashboard.NewHandler(dashboard.NewRepository(db), tokens, cfg.ACSOnlineTimeout, cfg.ACSOfflineTimeout)
	catalogHandler := catalog.NewHandler(catalog.NewRepository(db), tokens)
	firmwareStorage := firmware.NewStorage(cfg.FirmwareStoragePath, cfg.FirmwareMaxFileSize)
	firmwareHandler := firmware.NewHandler(firmware.NewRepository(db), firmwareStorage, tokens, logger)
	firmwareDeliveryHandler := firmware.NewDeliveryHandler(db, cfg.ACSDefaultTenantSlug, firmwareStorage)
	cwmpDispatcher := cwmp.NewDispatcher(db, cfg.ACSDefaultTenantSlug, wifiCipher, cfg.ACSPublicURL)
	cwmpHandler := cwmp.NewHandler(cwmp.NewRepository(db, cfg.ACSDefaultTenantSlug), cwmpDispatcher, logger, cfg.ACSCookieSecure)
	hosts := httpserver.VirtualHosts{Application: cfg.ApplicationFQDN, ACS: cfg.ACSFQDN}
	registerApplication := func(mux *http.ServeMux) {
		authHandler.Register(mux)
		routerHandler.Register(mux)
		operationsHandler.Register(mux)
		configurationHandler.Register(mux)
		commandHandler.Register(mux)
		dashboardHandler.Register(mux)
		catalogHandler.Register(mux)
		firmwareHandler.Register(mux)
	}
	registerACS := func(mux *http.ServeMux) { cwmpHandler.Register(mux); firmwareDeliveryHandler.Register(mux) }
	server := httpserver.New(cfg.HTTPAddress, hosts, db, logger, registerApplication, registerACS)
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

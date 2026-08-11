package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type Database interface {
	Ping(context.Context) error
}

type VirtualHosts struct {
	Application string
	ACS         string
}

func New(address string, hosts VirtualHosts, db Database, logger *slog.Logger, registerApplication, registerACS func(*http.ServeMux)) *http.Server {
	applicationMux := http.NewServeMux()
	acsMux := http.NewServeMux()
	health := func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
	ready := func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_ERROR", "Database unavailable")
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
	for _, mux := range []*http.ServeMux{applicationMux, acsMux} {
		mux.HandleFunc("GET /health", health)
		mux.HandleFunc("GET /ready", ready)
	}
	if registerApplication != nil {
		registerApplication(applicationMux)
	}
	if registerACS != nil {
		registerACS(acsMux)
	}
	notFound := func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Resource not found")
	}
	applicationMux.HandleFunc("/", notFound)
	acsMux.HandleFunc("/", notFound)
	hostHandler := routeByHost(hosts, applicationMux, acsMux)

	return &http.Server{
		Addr:              address,
		Handler:           middleware(logger, hostHandler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

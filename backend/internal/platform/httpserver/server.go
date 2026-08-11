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

func New(address string, db Database, logger *slog.Logger, registerRoutes func(*http.ServeMux)) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_ERROR", "Database unavailable")
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	if registerRoutes != nil {
		registerRoutes(mux)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Resource not found")
	})

	return &http.Server{
		Addr:              address,
		Handler:           middleware(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// Package httpapi wires the HTTP routes: the JSON API under /api and the
// built single-page frontend for every other path.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Pinger is the part of the database pool the health check needs.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewRouter builds the HTTP handler. staticDir is the folder with the built
// frontend; if it has no index.html, only the API is served.
func NewRouter(db Pinger, staticDir string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", health(db))

		// Unknown API paths get JSON errors, never the SPA's index.html.
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		})
		r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		})
	})

	if spa := spaHandler(staticDir); spa != nil {
		r.Get("/*", spa.ServeHTTP)
	}
	return r
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// health reports whether the server can reach PostgreSQL right now.
func health(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			slog.Error("health check: database ping failed", "err", err)
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "unavailable", Database: "unreachable"})
			return
		}
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Database: "ok"})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "err", err)
	}
}

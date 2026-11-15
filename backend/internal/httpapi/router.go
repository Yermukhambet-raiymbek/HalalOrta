// Package httpapi wires the HTTP routes: the JSON API under /api and the
// built single-page frontend for every other path.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"littlevillage/internal/store"
)

// Options configure the router.
type Options struct {
	// StaticDir is the folder with the built frontend. Without an
	// index.html in it, only the API is served.
	StaticDir string
	// CookieSecure forces the Secure flag on the session cookie.
	CookieSecure bool
}

// NewRouter builds the HTTP handler.
func NewRouter(st *store.Store, opts Options) http.Handler {
	fh := &familyHandlers{store: st, cookieSecure: opts.CookieSecure}

	r := chi.NewRouter()
	r.Use(logRequests)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Use(noStore)

		// Public endpoints.
		r.Get("/health", health(st))
		r.Post("/families", fh.create)
		r.Post("/families/join", fh.join)
		r.Get("/invites/{token}", fh.invite)

		// Private endpoints: all go through requireMember.
		r.Group(func(r chi.Router) {
			r.Use(requireMember(st))
			r.Get("/me", fh.me)
			r.Get("/family", fh.family)
			r.Post("/session/switch", fh.switchMember)
		})

		// Unknown API paths get JSON errors, never the SPA's index.html.
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusNotFound, codeNotFound, "Not found.")
		})
		r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusMethodNotAllowed, codeMethodNotAllowed, "Method not allowed.")
		})
	})

	if spa := spaHandler(opts.StaticDir); spa != nil {
		r.Get("/*", spa.ServeHTTP)
	}
	return r
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

type pinger interface {
	Ping(ctx context.Context) error
}

// health reports whether the server can reach PostgreSQL right now.
func health(db pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

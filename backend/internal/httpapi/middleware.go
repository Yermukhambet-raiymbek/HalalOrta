package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"littlevillage/internal/auth"
	"littlevillage/internal/store"
)

// requireMember is the single gate for private endpoints. It resolves the
// device cookie to the active member and puts the session in the request
// context. Handlers take the family ID from that session only.
func requireMember(st *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := auth.DeviceToken(r)
			if token == "" {
				writeUnauthenticated(w)
				return
			}
			sess, err := st.SessionByToken(r.Context(), token)
			if errors.Is(err, store.ErrNotFound) {
				writeUnauthenticated(w)
				return
			}
			if err != nil {
				writeInternal(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithSession(r.Context(), sess)))
		})
	}
}

func writeUnauthenticated(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, codeUnauthenticated,
		"No family member is active in this browser. Create a family or join one with an invite link.")
}

// noStore keeps API responses out of browser and proxy caches.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// logRequests logs one line per request. It logs the route pattern, not the
// raw path, so invite tokens in URLs never reach the logs.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		slog.Info("request", "method", r.Method, "route", routePattern(r), "status", status,
			"duration", time.Since(start).Round(time.Microsecond))
	})
}

func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
		return rc.RoutePattern()
	}
	return "unmatched"
}

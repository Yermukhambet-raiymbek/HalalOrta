// Command server runs the LittleVillage service: the JSON API under /api and
// the built frontend for every other path.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"littlevillage/content"
	"littlevillage/internal/config"
	"littlevillage/internal/db"
	"littlevillage/internal/httpapi"
	"littlevillage/internal/migrate"
	"littlevillage/internal/store"
	"littlevillage/migrations"
)

const shutdownTimeout = 10 * time.Second

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("server stopped with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// Cancelled on Ctrl+C or SIGTERM (what Docker and most hosts send on stop).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	applied, err := migrate.Up(ctx, pool, migrations.FS)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	slog.Info("database schema is up to date", "applied_now", applied)

	// Lesson files are validated before anything is served: a bad file
	// stops the start with an error that names the file.
	lessons, err := content.LoadEmbedded()
	if err != nil {
		return fmt.Errorf("lesson content: %w", err)
	}
	st := store.New(pool)
	if err := st.SyncLessons(ctx, lessons); err != nil {
		return fmt.Errorf("sync lessons: %w", err)
	}
	slog.Info("lessons synced", "count", len(lessons))

	router := httpapi.NewRouter(st, httpapi.Options{
		StaticDir:    cfg.StaticDir,
		CookieSecure: cfg.CookieSecure,
	})
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr, "static_dir", cfg.StaticDir)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}
	stop() // a second signal now terminates the process immediately

	slog.Info("shutting down: finishing in-flight requests")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	slog.Info("server stopped")
	return nil
}

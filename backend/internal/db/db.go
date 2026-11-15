// Package db opens the PostgreSQL connection pool.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	connectAttempts = 15
	pingTimeout     = 2 * time.Second
	retryDelay      = time.Second
)

// Connect creates a pgx pool and waits until the database answers a ping.
// The database may still be starting (for example right after a deploy), so
// the first ping is retried for a short while before giving up.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	for attempt := 1; ; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return pool, nil
		}
		if attempt == connectAttempts {
			pool.Close()
			return nil, fmt.Errorf("connect to database after %d attempts: %w", attempt, err)
		}
		slog.Warn("database not ready, retrying", "attempt", attempt, "err", err)

		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(retryDelay):
		}
	}
}

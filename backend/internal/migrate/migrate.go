// Package migrate applies embedded SQL migrations in order, each exactly once.
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// lockID is an arbitrary constant key for the PostgreSQL advisory lock that
// serializes migration runs when several instances start at the same time.
const lockID int64 = 7_271_000_001

// Up applies every *.sql file in fsys whose version (file name without
// ".sql") is not yet recorded in schema_migrations. Files run in lexical
// order. Each file runs in its own transaction together with the insert of
// its version, so a failed migration leaves no partial state behind.
//
// The first migration must create the schema_migrations table.
// Up returns the versions it applied during this call.
func Up(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) ([]string, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(names)

	// An advisory lock belongs to a session, so every step uses one connection.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		return nil, fmt.Errorf("take migration lock: %w", err)
	}
	defer func() {
		// A fresh context lets the unlock run even if ctx was cancelled.
		if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockID); err != nil {
			// Closing the session releases the lock as well.
			_ = conn.Conn().Close(context.Background())
		}
	}()

	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return nil, err
	}

	var done []string
	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		if applied[version] {
			continue
		}
		sql, err := fs.ReadFile(fsys, name)
		if err != nil {
			return done, fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := apply(ctx, conn, version, string(sql)); err != nil {
			return done, fmt.Errorf("apply migration %s: %w", name, err)
		}
		done = append(done, version)
	}
	return done, nil
}

// appliedVersions returns the recorded versions, or an empty set on a fresh
// database where schema_migrations does not exist yet.
func appliedVersions(ctx context.Context, conn *pgxpool.Conn) (map[string]bool, error) {
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT to_regclass('schema_migrations') IS NOT NULL").Scan(&exists); err != nil {
		return nil, fmt.Errorf("check schema_migrations: %w", err)
	}
	applied := make(map[string]bool)
	if !exists {
		return applied, nil
	}

	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	versions, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	for _, v := range versions {
		applied[v] = true
	}
	return applied, nil
}

func apply(ctx context.Context, conn *pgxpool.Conn, version, sql string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful commit

	// Without arguments pgx uses the simple protocol, so a file may hold
	// several statements.
	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

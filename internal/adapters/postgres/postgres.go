// Package postgres provides the PostgreSQL-backed user repository.
// It is intentionally small: one database, one table (users), standard
// database/sql + lib/pq so the binary stays a static CGO-free build.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Open opens a pooled connection to PostgreSQL, waiting until the instance
// answers (useful during rollout when the API pod starts before the database
// pod becomes ready).
func Open(ctx context.Context, dsn string, timeout time.Duration) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	deadline := time.Now().Add(timeout)
	backoff := 500 * time.Millisecond
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return db, nil
		}
		if time.Now().After(deadline) {
			_ = db.Close()
			return nil, fmt.Errorf("postgres is not reachable: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = db.Close()
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 3*time.Second {
			backoff *= 2
		}
	}
}

// Close releases the pool.
func Close(db *sql.DB) {
	if db != nil {
		_ = db.Close()
	}
}

// schema is idempotent so every replica can apply it on boot.
const schema = `
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL,
    last_login_at TIMESTAMPTZ
);
`

// Migrate applies the minimal user schema.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("apply postgres schema: %w", err)
	}
	return nil
}

// isUniqueViolation reports whether the error is a Postgres 23505 unique
// constraint violation (duplicate username).
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

// maskPassword hides the password from error messages and logs.
func maskPassword(dsn string) string {
	const key = "password="
	i := strings.LastIndex(dsn, key)
	if i < 0 {
		return dsn
	}
	rest := dsn[i+len(key):]
	end := strings.IndexAny(rest, "& ")
	if end < 0 {
		end = len(rest)
	}
	if end == 0 {
		return dsn[:i] + key + "***"
	}
	return dsn[:i] + key + "***" + rest[end:]
}

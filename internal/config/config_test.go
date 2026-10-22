package config

import (
	"testing"
)

// TestPostgresDSN guards against DSN assembly regressions. The k8s
// deployment configures Postgres via PGHOST/PGUSER/PGPASSWORD/PGDATABASE
// (no PG_DSN), so the assembled URL must be parseable by lib/pq: exactly
// one "@" between credentials and host, and the right database in the path.
func TestPostgresDSN(t *testing.T) {
	cfg := Config{
		PGUser:     "wallet",
		PGPassword: "wallet-demo-change-me",
		PGHost:     "postgres.wallet-demo.svc.cluster.local",
		PGPort:     "5432",
		PGDatabase: "wallet",
		PGSSLMode:  "disable",
	}
	got := cfg.PostgresDSN()
	want := "postgres://wallet:wallet-demo-change-me@postgres.wallet-demo.svc.cluster.local:5432/wallet?sslmode=disable"
	if got != want {
		t.Fatalf("PostgresDSN() = %q, want %q", got, want)
	}

	// Passwords with reserved characters must be percent-escaped.
	cfg.PGPassword = "p@ss:word/x"
	if got := cfg.PostgresDSN(); got != "postgres://wallet:p%40ss%3Aword%2Fx@postgres.wallet-demo.svc.cluster.local:5432/wallet?sslmode=disable" {
		t.Fatalf("PostgresDSN() with special password = %q", got)
	}

	cfg.PGPassword = ""
	if got := cfg.PostgresDSN(); got != "postgres://wallet@postgres.wallet-demo.svc.cluster.local:5432/wallet?sslmode=disable" {
		t.Fatalf("PostgresDSN() without password = %q", got)
	}

	// An explicit PG_DSN always wins, unchanged.
	cfg.PGDSN = "postgres://custom@localhost:1234/db?sslmode=prefer"
	if got := cfg.PostgresDSN(); got != "postgres://custom@localhost:1234/db?sslmode=prefer" {
		t.Fatalf("PostgresDSN() with PGDSN = %q", got)
	}
}

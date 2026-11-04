//go:build integration

package db_test

import (
	"context"
	"database/sql"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"wishix/server/internal/db"
)

// Reuse only the randomly named schema from the transaction fixtures. Goose
// never connects to public or changes the working application's version table.
func liveMigrator(t *testing.T) (*sql.DB, *db.Migrator, string) {
	t.Helper()
	_, _, _, table := liveTransactions(t)
	schema, _, _ := strings.Cut(table, ".")
	dsn, err := db.ConnectionString()
	if err != nil {
		t.Fatal(err)
	}
	connection, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("parse test connection settings")
	}
	query := connection.Query()
	query.Set("search_path", schema)
	connection.RawQuery = query.Encode()
	pool, err := sql.Open("pgx", connection.String())
	if err != nil {
		t.Fatal("open isolated migration connection")
	}
	t.Cleanup(func() { _ = pool.Close() })
	migrator, err := db.NewSQLMigrator(pool)
	if err != nil {
		t.Fatal(err)
	}
	return pool, migrator, schema
}

func TestIntegrationGooseUpDownUp(t *testing.T) {
	pool, migrator, schema := liveMigrator(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := migrator.Apply(ctx); err != nil {
		t.Fatal("automatic up:", err)
	}
	assertVersion := func(want int64) {
		t.Helper()
		version, err := migrator.GetDBVersion(ctx)
		if err != nil || version != want {
			t.Fatalf("version: got %d, want %d; %v", version, want, err)
		}
	}
	assertVersion(1)
	if _, err := pool.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, email_verified_at, created_at) VALUES ('00000000-0000-4000-8000-000000000001', 'migration_test', 'migration@example.test', 'test-hash', now(), now())`); err != nil {
		t.Fatal(err)
	}
	if err := migrator.Apply(ctx); err != nil {
		t.Fatal("repeated up:", err)
	}
	var count int
	if err := pool.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil || count != 1 {
		t.Fatal("repeated up lost account data")
	}
	statuses, err := migrator.Status(ctx)
	if err != nil || len(statuses) != 1 || statuses[0].State != goose.StateApplied {
		t.Fatal("incorrect Goose status:", err)
	}
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("down:", err)
	}
	assertVersion(0)
	if err := pool.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema = $1 AND table_name IN ('users', 'auth_challenges', 'auth_sessions', 'auth_rate_limits')`, schema).Scan(&count); err != nil || count != 0 {
		t.Fatal("down did not remove auth tables")
	}
	if _, err := migrator.UpTo(ctx, 1); err != nil {
		t.Fatal("up-to after down:", err)
	}
	assertVersion(1)
	if _, err := migrator.DownTo(ctx, 0); err != nil {
		t.Fatal("down-to zero:", err)
	}
	assertVersion(0)
	if _, err := migrator.UpByOne(ctx); err != nil {
		t.Fatal("up-by-one:", err)
	}
	assertVersion(1)
}

func TestIntegrationGooseAdoptsExistingSchemaWithoutDeletingData(t *testing.T) {
	pool, migrator, _ := liveMigrator(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err := migrator.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO users (id, username, email, password_hash, email_verified_at, created_at) VALUES ('00000000-0000-4000-8000-000000000002', 'legacy_user', 'legacy@example.test', 'test-hash', now(), now())`,
		`CREATE TABLE schema_migrations (version STRING PRIMARY KEY, checksum STRING NOT NULL)`,
		`INSERT INTO schema_migrations VALUES ('001_email_auth.sql', 'legacy-checksum')`,
		`DROP TABLE goose_db_version`,
	} {
		if _, err := pool.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	// A new process sees the pre-Goose schema, including its historical metadata.
	fresh, err := db.NewSQLMigrator(pool)
	if err != nil {
		t.Fatal(err)
	}
	if err := fresh.Apply(ctx); err != nil {
		t.Fatal("adopt legacy schema:", err)
	}
	var count int
	if err := pool.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE username = 'legacy_user'").Scan(&count); err != nil || count != 1 {
		t.Fatal("legacy account was not preserved")
	}
	if err := pool.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE checksum = 'legacy-checksum'").Scan(&count); err != nil || count != 1 {
		t.Fatal("legacy history was changed")
	}
	version, err := fresh.GetDBVersion(ctx)
	if err != nil || version != 1 {
		t.Fatal("Goose did not record the adopted version")
	}
}

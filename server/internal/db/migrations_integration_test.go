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
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
	assertVersion(11)
	if _, err := pool.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, email_verified_at, created_at) VALUES ('00000000-0000-4000-8000-000000000001', 'migration_test', 'migration@example.test', 'test-hash', now(), now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.ExecContext(ctx, `INSERT INTO wishes (id, owner_id, title, price, currency, priority, created_at) VALUES ('00000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000001', 'Kept wish', 12.34, 'EUR', 'medium', now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.ExecContext(ctx, `INSERT INTO wish_shop_links (wish_id, position, url) VALUES ('00000000-0000-4000-8000-000000000002', 0, 'https://example.test/item')`); err != nil {
		t.Fatal(err)
	}
	if err := migrator.Apply(ctx); err != nil {
		t.Fatal("repeated up:", err)
	}
	var count int
	if err := pool.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil || count != 1 {
		t.Fatal("repeated up lost account data")
	}
	if err := pool.QueryRowContext(ctx, "SELECT count(*) FROM wishes JOIN wish_shop_links ON wishes.id = wish_shop_links.wish_id WHERE price = 12.34 AND currency = 'EUR'").Scan(&count); err != nil || count != 1 {
		t.Fatal("repeated up lost wishes or shop links")
	}
	statuses, err := migrator.Status(ctx)
	if err != nil || len(statuses) != 11 {
		t.Fatal("incorrect Goose status:", err)
	}
	for _, status := range statuses {
		if status.State != goose.StateApplied {
			t.Fatalf("migration not applied: %v", status)
		}
	}
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("profile name down:", err)
	}
	assertVersion(10)
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("received status down:", err)
	}
	assertVersion(9)
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("facebook down:", err)
	}
	assertVersion(8)
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("google down:", err)
	}
	assertVersion(7)
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("down:", err)
	}
	assertVersion(6)
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("reservations down:", err)
	}
	assertVersion(5)
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("timezone down:", err)
	}
	assertVersion(4)
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("lists down:", err)
	}
	assertVersion(3)
	if err := pool.QueryRowContext(ctx, "SELECT count(*) FROM wishes").Scan(&count); err != nil || count != 1 {
		t.Fatal("lists rollback removed existing wishes")
	}
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("images down:", err)
	}
	assertVersion(2)
	if err := pool.QueryRowContext(ctx, "SELECT count(*) FROM wishes").Scan(&count); err != nil || count != 1 {
		t.Fatal("image rollback removed existing wishes")
	}
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("wishes down:", err)
	}
	assertVersion(1)
	if err := pool.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema = $1 AND table_name IN ('wishes', 'wish_shop_links')`, schema).Scan(&count); err != nil || count != 0 {
		t.Fatal("wishes down did not remove wish tables")
	}
	if err := pool.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil || count != 1 {
		t.Fatal("wishes rollback removed existing accounts")
	}
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("auth down:", err)
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

func TestIntegrationListsUpgradePreservesExistingCollection(t *testing.T) {
	pool, migrator, _ := liveMigrator(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if _, err := migrator.UpTo(ctx, 3); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO users (id, username, email, password_hash, email_verified_at, created_at) VALUES ('00000000-0000-4000-8000-000000000001', 'existing_user', 'existing@example.test', 'test-hash', now(), now())`,
		`INSERT INTO wishes (id, owner_id, title, price, currency, priority, created_at) VALUES ('00000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000001', 'Existing gift', 49.95, 'MDL', 'high', now())`,
		`INSERT INTO wish_shop_links (wish_id, position, url) VALUES ('00000000-0000-4000-8000-000000000002', 0, 'https://example.test/product')`,
		`INSERT INTO wish_images (wish_id, version, data) VALUES ('00000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000003', decode('ffd8ffd9', 'hex'))`,
	} {
		if _, err := pool.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrator.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	assertData := func() {
		t.Helper()
		if err := pool.QueryRowContext(ctx, `SELECT count(*) FROM wishes w JOIN wish_images i ON i.wish_id = w.id JOIN wish_shop_links s ON s.wish_id = w.id WHERE w.title = 'Existing gift' AND w.price = 49.95 AND w.currency = 'MDL' AND i.data = decode('ffd8ffd9', 'hex') AND s.url = 'https://example.test/product'`).Scan(&count); err != nil || count != 1 {
			t.Fatal("lists migration modified existing data", err, count)
		}
	}
	assertData()
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("facebook rollback", err)
	}
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("google rollback", err)
	}
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("reservation message rollback", err)
	}
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("reservations rollback", err)
	}
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("timezone rollback", err)
	}
	if err := pool.QueryRowContext(ctx, `SELECT lists_version FROM wishes WHERE id = '00000000-0000-4000-8000-000000000002'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("existing wish membership version not initialized", err, count)
	}
	if _, err := migrator.Down(ctx); err != nil {
		t.Fatal("lists rollback", err)
	}
	assertData()
}

func TestIntegrationGooseAdoptsExistingSchemaWithoutDeletingData(t *testing.T) {
	pool, migrator, _ := liveMigrator(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
	if err != nil || version != 11 {
		t.Fatal("Goose did not record the adopted version")
	}
}

package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Migrator struct{ database *gorm.DB }

func NewMigrator(database *gorm.DB) *Migrator { return &Migrator{database: database} }

// Apply runs forward-only, idempotent DDL before accepting HTTP requests.
// CockroachDB schema changes run individually, outside application transactions.
// A partially applied migration can be resumed; recorded files cannot be edited.
func (m *Migrator) Apply(ctx context.Context) error {
	database := m.database.WithContext(ctx)
	if err := database.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version STRING PRIMARY KEY, checksum STRING NOT NULL)`).Error; err != nil {
		return err
	}
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		body, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		// Git autocrlf must not change a migration checksum across platforms.
		sql := strings.ReplaceAll(string(body), "\r\n", "\n")
		sum := sha256.Sum256([]byte(sql))
		checksum := hex.EncodeToString(sum[:])
		var rows []struct{ Checksum string }
		if err := database.Raw("SELECT checksum FROM schema_migrations WHERE version = ?", entry.Name()).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) > 0 {
			if rows[0].Checksum != checksum {
				return fmt.Errorf("migration %s checksum mismatch", entry.Name())
			}
			continue
		}
		// These SQL files contain only simple DDL, with no procedural bodies.
		for _, statement := range strings.Split(sql, ";") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if err := database.Exec(statement).Error; err != nil {
				return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
			}
		}
		if err := database.Exec("INSERT INTO schema_migrations (version, checksum) VALUES (?, ?) ON CONFLICT (version) DO NOTHING", entry.Name(), checksum).Error; err != nil {
			return err
		}
		rows = nil
		if err := database.Raw("SELECT checksum FROM schema_migrations WHERE version = ?", entry.Name()).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) != 1 || rows[0].Checksum != checksum {
			return fmt.Errorf("migration %s checksum mismatch", entry.Name())
		}
	}
	return nil
}

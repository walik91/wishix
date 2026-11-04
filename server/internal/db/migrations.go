package db

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrator uses the same Goose provider and embedded SQL for startup and CLI.
// It does not own the pool: its caller must close the connection.
type Migrator struct{ *goose.Provider }

func NewMigrator(database *gorm.DB) (*Migrator, error) {
	pool, err := database.DB()
	if err != nil {
		return nil, err
	}
	return NewSQLMigrator(pool)
}

func NewSQLMigrator(pool *sql.DB) (*Migrator, error) {
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, pool, files, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return nil, err
	}
	return &Migrator{Provider: provider}, nil
}

// Apply is the automatic startup path: never roll back migrations implicitly.
func (m *Migrator) Apply(ctx context.Context) error {
	_, err := m.Up(ctx)
	return err
}

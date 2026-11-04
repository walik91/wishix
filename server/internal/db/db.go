package db

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"

	"go.uber.org/fx"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// New prepares CockroachDB access and registers its connection lifecycle.
func New(lifecycle fx.Lifecycle) (*gorm.DB, error) {
	name, user, password := os.Getenv("DB_NAME"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD")
	if name == "" || user == "" || password == "" {
		return nil, errors.New("DB_NAME, DB_USER and DB_PASSWORD are required")
	}
	dsn := url.URL{Scheme: "postgresql", Host: "db:26257", Path: "/" + name, User: url.UserPassword(user, password)}
	query := url.Values{"sslmode": {"verify-full"}, "sslrootcert": {"/db-certs/ca.crt"}, "connect_timeout": {"5"}}
	dsn.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(dsn.String()), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, errors.New("invalid CockroachDB connection configuration")
	}
	connection, err := database.DB()
	if err != nil {
		return nil, err
	}
	connection.SetMaxOpenConns(10)
	connection.SetMaxIdleConns(5)
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := connection.PingContext(ctx); err != nil {
				// Fx does not call OnStop for a hook whose OnStart failed.
				_ = connection.Close()
				return errors.New("CockroachDB connection check failed; check DB settings and database availability")
			}
			return nil
		},
		OnStop: func(context.Context) error { return connection.Close() },
	})
	return database, nil
}

// SQL exposes the same underlying pool for infrastructure health checks.
func SQL(database *gorm.DB) (*sql.DB, error) { return database.DB() }

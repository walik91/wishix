package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"text/tabwriter"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"wishix/server/internal/db"
)

const usage = "usage: migrate up | up-by-one | up-to VERSION | down | down-to VERSION | redo | reset | status | version"

type command struct {
	name   string
	target int64
}

func parseCommand(args []string) (command, error) {
	if len(args) == 0 {
		return command{}, errors.New(usage)
	}
	c := command{name: args[0]}
	switch c.name {
	case "up", "up-by-one", "down", "redo", "reset", "status", "version":
		if len(args) != 1 {
			return command{}, errors.New(usage)
		}
	case "up-to", "down-to":
		if len(args) != 2 {
			return command{}, errors.New(usage)
		}
		version, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || version < 0 || (c.name == "up-to" && version == 0) {
			return command{}, errors.New("VERSION must be a positive integer (down-to also accepts 0)")
		}
		c.target = version
	default:
		return command{}, errors.New(usage)
	}
	return c, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	c, err := parseCommand(args)
	if err != nil {
		return err
	}
	dsn, err := db.ConnectionString()
	if err != nil {
		return err
	}
	pool, err := sql.Open("pgx", dsn)
	if err != nil {
		return errors.New("invalid CockroachDB connection configuration")
	}
	defer pool.Close()
	pool.SetMaxOpenConns(2)
	if err := pool.PingContext(ctx); err != nil {
		return errors.New("CockroachDB connection failed; check DB settings and database availability")
	}
	migrator, err := db.NewSQLMigrator(pool)
	if err != nil {
		return err
	}
	return execute(ctx, c, migrator.Provider, output)
}

func execute(ctx context.Context, c command, provider *goose.Provider, output io.Writer) error {
	var results []*goose.MigrationResult
	var err error
	switch c.name {
	case "up":
		results, err = provider.Up(ctx)
	case "up-to":
		results, err = provider.UpTo(ctx, c.target)
	case "up-by-one", "down":
		var result *goose.MigrationResult
		if c.name == "down" {
			result, err = provider.Down(ctx)
		} else {
			result, err = provider.UpByOne(ctx)
		}
		if result != nil {
			results = append(results, result)
		}
	case "down-to", "reset":
		results, err = provider.DownTo(ctx, c.target)
	case "redo":
		var result *goose.MigrationResult
		result, err = provider.Down(ctx)
		if err == nil {
			fmt.Fprintln(output, result)
			result, err = provider.UpByOne(ctx)
			if result != nil {
				results = append(results, result)
			}
		}
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			return err
		}
		table := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "VERSION\tSTATE\tMIGRATION")
		for _, status := range statuses {
			fmt.Fprintf(table, "%d\t%s\t%s\n", status.Source.Version, status.State, status.Source.Path)
		}
		return table.Flush()
	case "version":
		version, err := provider.GetDBVersion(ctx)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, version)
		return err
	default:
		return errors.New(usage)
	}
	if err != nil {
		return err
	}
	for _, result := range results {
		fmt.Fprintln(output, result)
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Database version: %d\n", version)
	return err
}

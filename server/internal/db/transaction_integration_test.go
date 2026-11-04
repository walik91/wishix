//go:build integration

package db_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"wishix/server/internal/db"
)

type integrationLifecycle struct{ hooks []fx.Hook }

func (lifecycle *integrationLifecycle) Append(hook fx.Hook) {
	lifecycle.hooks = append(lifecycle.hooks, hook)
}

// These fixtures live only in a randomly named test schema, never in public or
// product tables. Cleanup is registered only after creating that schema.
func liveTransactions(t *testing.T) (*gorm.DB, *db.Executor, *db.TransactionManager, string) {
	t.Helper()
	lifecycle := &integrationLifecycle{}
	database, err := db.New(lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for _, hook := range lifecycle.hooks {
		if err := hook.OnStart(ctx); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for index := len(lifecycle.hooks) - 1; index >= 0; index-- {
			if err := lifecycle.hooks[index].OnStop(context.Background()); err != nil {
				t.Error(err)
			}
		}
	})
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "tx_test_" + hex.EncodeToString(random[:])
	if err := database.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	table := schema + ".probe"
	if err := database.Exec("CREATE TABLE " + table + " (id STRING PRIMARY KEY, value INT NOT NULL DEFAULT 0)").Error; err != nil {
		t.Fatal(err)
	}
	executor := db.NewExecutor(database)
	return database, executor, db.NewTransactionManager(executor), table
}

type probeRepository struct {
	executor *db.Executor
	table    string
}

func (repo probeRepository) insert(ctx context.Context, id string) error {
	return repo.executor.DB(ctx).Exec("INSERT INTO "+repo.table+" (id) VALUES (?)", id).Error
}

func assertRows(t *testing.T, database *gorm.DB, table string, expected int64) {
	t.Helper()
	var count int64
	if err := database.Table(table).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("want %d rows, got %d", expected, count)
	}
}

func TestIntegrationAtomicCommitAndRollbackAcrossRepositories(t *testing.T) {
	database, executor, manager, table := liveTransactions(t)
	first := probeRepository{executor, table}
	second := probeRepository{executor, table}
	err := manager.WithTransaction(context.Background(), func(ctx context.Context) error {
		var isolation string
		if err := executor.DB(ctx).Raw("SHOW transaction_isolation").Scan(&isolation).Error; err != nil {
			return err
		}
		if isolation != "serializable" {
			return fmt.Errorf("unexpected isolation: %s", isolation)
		}
		if err := first.insert(ctx, "committed-first"); err != nil {
			return err
		}
		return manager.WithTransaction(ctx, func(nestedCtx context.Context) error { return second.insert(nestedCtx, "committed-second") })
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, database, table, 2)

	expected := errors.New("verification rollback")
	err = manager.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := first.insert(ctx, "rolled-back-first"); err != nil {
			return err
		}
		if err := second.insert(ctx, "rolled-back-second"); err != nil {
			return err
		}
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("got %v", err)
	}
	assertRows(t, database, table, 2)

	err = manager.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := first.insert(ctx, "ignored-nested-error"); err != nil {
			return err
		}
		_ = manager.WithTransaction(ctx, func(context.Context) error { return expected })
		return nil
	})
	if !errors.Is(err, expected) {
		t.Fatalf("got %v", err)
	}
	assertRows(t, database, table, 2)
}

func TestIntegrationRetriesRealCockroachConflict(t *testing.T) {
	database, executor, manager, table := liveTransactions(t)
	if err := database.Exec("INSERT INTO " + table + " (id, value) VALUES ('counter', 0)").Error; err != nil {
		t.Fatal(err)
	}
	attempts := 0
	var stale context.Context
	err := manager.WithTransaction(context.Background(), func(ctx context.Context) error {
		attempts++
		if attempts > 1 && !errors.Is(executor.DB(stale).Error, db.ErrTransactionClosed) {
			return errors.New("previous attempt remained active")
		}
		stale = ctx
		var value int
		if err := executor.DB(ctx).Raw("SELECT value FROM " + table + " WHERE id = 'counter'").Scan(&value).Error; err != nil {
			return err
		}
		if attempts == 1 {
			// This deliberately models a second, independent request after the
			// first transaction has already read and returned the old value.
			// Only the test uses the root handle inside a callback.
			if err := database.Exec("UPDATE " + table + " SET value = value + 1 WHERE id = 'counter'").Error; err != nil {
				return err
			}
		}
		return executor.DB(ctx).Exec("UPDATE "+table+" SET value = ? WHERE id = 'counter'", value+1).Error
	})
	if err != nil || attempts != 2 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
	var value int
	if err := database.Raw("SELECT value FROM " + table + " WHERE id = 'counter'").Scan(&value).Error; err != nil {
		t.Fatal(err)
	}
	if value != 2 {
		t.Fatalf("concurrent update was lost: got %d", value)
	}
	assertRows(t, database, table, 1)
	if !errors.Is(executor.DB(stale).Exec("SELECT 1").Error, db.ErrTransactionClosed) {
		t.Fatal("finished context escaped its transaction")
	}
}

func TestIntegrationCancellationRollsBack(t *testing.T) {
	database, executor, manager, table := liveTransactions(t)
	repo := probeRepository{executor, table}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := manager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := repo.insert(txCtx, "cancelled-write"); err != nil {
			return err
		}
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	assertRows(t, database, table, 0)
}

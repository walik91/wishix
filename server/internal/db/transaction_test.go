package db_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cockroachdb/cockroach-go/v2/crdb"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"wishix/server/internal/db"
)

func mockTransactions(t *testing.T) (*db.Executor, *db.TransactionManager, sqlmock.Sqlmock) {
	t.Helper()
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	database, err := gorm.Open(postgres.New(postgres.Config{Conn: connection}), &gorm.Config{
		DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		_ = connection.Close()
	})
	executor := db.NewExecutor(database)
	return executor, db.NewTransactionManager(executor), mock
}

func begin(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec("^SAVEPOINT cockroach_restart$").WillReturnResult(sqlmock.NewResult(0, 0))
}

func commit(mock sqlmock.Sqlmock) {
	mock.ExpectExec("^RELEASE SAVEPOINT cockroach_restart$").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
}

func restart(mock sqlmock.Sqlmock) {
	for _, statement := range []string{"ROLLBACK", "BEGIN", "SAVEPOINT cockroach_restart"} {
		mock.ExpectExec("^" + statement + "$").WillReturnResult(sqlmock.NewResult(0, 0))
	}
}

func TestTransactionCommitsAndJoinsNestedCalls(t *testing.T) {
	executor, manager, mock := mockTransactions(t)
	begin(mock)
	mock.ExpectExec("INSERT INTO probe").WithArgs(1).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO probe").WithArgs(2).WillReturnResult(sqlmock.NewResult(0, 1))
	commit(mock)
	otherRepository := executor
	err := manager.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := executor.DB(ctx).Exec("INSERT INTO probe (id) VALUES (?)", 1).Error; err != nil {
			return err
		}
		return manager.WithTransaction(ctx, func(nestedCtx context.Context) error {
			if executor.DB(ctx).Statement.ConnPool != otherRepository.DB(nestedCtx).Statement.ConnPool {
				t.Fatal("nested call did not join the same SQL transaction")
			}
			return otherRepository.DB(nestedCtx).Exec("INSERT INTO probe (id) VALUES (?)", 2).Error
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTransactionRollsBackIgnoredNestedError(t *testing.T) {
	executor, manager, mock := mockTransactions(t)
	begin(mock)
	mock.ExpectExec("INSERT INTO probe").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()
	expected := errors.New("nested business failure")
	err := manager.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := executor.DB(ctx).Exec("INSERT INTO probe (id) VALUES (1)").Error; err != nil {
			return err
		}
		_ = manager.WithTransaction(ctx, func(context.Context) error { return expected })
		return nil // This must not commit partially completed work.
	})
	if !errors.Is(err, expected) {
		t.Fatalf("want nested error, got %v", err)
	}
}

func TestTransactionRetriesConflictWithFreshContext(t *testing.T) {
	executor, manager, mock := mockTransactions(t)
	begin(mock)
	restart(mock)
	commit(mock)
	attempts := 0
	var previous context.Context
	err := manager.WithTransaction(context.Background(), func(ctx context.Context) error {
		attempts++
		if attempts == 1 {
			previous = ctx
			return fmt.Errorf("conflict: %w", &pgconn.PgError{Code: "40001"})
		}
		if !errors.Is(executor.DB(previous).Error, db.ErrTransactionClosed) {
			t.Fatal("context from the previous attempt remained usable")
		}
		if executor.DB(ctx).Error != nil {
			t.Fatal("current attempt is not usable")
		}
		return nil
	})
	if err != nil || attempts != 2 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestTransactionRetryLimit(t *testing.T) {
	_, manager, mock := mockTransactions(t)
	begin(mock)
	for range 3 {
		restart(mock)
	}
	mock.ExpectExec("^ROLLBACK TO SAVEPOINT cockroach_restart$").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	attempts := 0
	err := manager.WithTransaction(context.Background(), func(context.Context) error {
		attempts++
		return &pgconn.PgError{Code: "40001"}
	})
	var limit *crdb.MaxRetriesExceededError
	if attempts != 4 || !errors.As(err, &limit) {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestTransactionDoesNotRetryConstraintViolation(t *testing.T) {
	_, manager, mock := mockTransactions(t)
	begin(mock)
	mock.ExpectRollback()
	expected := &pgconn.PgError{Code: "23505"}
	attempts := 0
	err := manager.WithTransaction(context.Background(), func(context.Context) error { attempts++; return expected })
	if !errors.Is(err, expected) || attempts != 1 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestTransactionPreservesAmbiguousCommitError(t *testing.T) {
	_, manager, mock := mockTransactions(t)
	begin(mock)
	mock.ExpectExec("^RELEASE SAVEPOINT cockroach_restart$").WillReturnError(errors.New("connection lost"))
	mock.ExpectRollback()
	err := manager.WithTransaction(context.Background(), func(context.Context) error { return nil })
	var ambiguous *crdb.AmbiguousCommitError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("commit uncertainty was lost: %v", err)
	}
}

func TestTransactionPanicRollsBackAndPropagates(t *testing.T) {
	executor, manager, mock := mockTransactions(t)
	begin(mock)
	mock.ExpectRollback()
	var saved context.Context
	defer func() {
		if recover() != "verification panic" {
			t.Error("panic was swallowed or replaced")
		}
		if !errors.Is(executor.DB(saved).Error, db.ErrTransactionClosed) {
			t.Error("panicked scope remained active")
		}
	}()
	_ = manager.WithTransaction(context.Background(), func(ctx context.Context) error { saved = ctx; panic("verification panic") })
}

func TestRecoveredNestedPanicStillRollsBack(t *testing.T) {
	_, manager, mock := mockTransactions(t)
	begin(mock)
	mock.ExpectRollback()
	err := manager.WithTransaction(context.Background(), func(ctx context.Context) error {
		func() {
			defer func() { _ = recover() }()
			_ = manager.WithTransaction(ctx, func(context.Context) error { panic("nested") })
		}()
		return nil
	})
	if !errors.Is(err, db.ErrTransactionAborted) {
		t.Fatalf("got %v", err)
	}
}

func TestExecutorRejectsStaleAndForeignContexts(t *testing.T) {
	executor, manager, mock := mockTransactions(t)
	foreign, foreignManager, _ := mockTransactions(t)
	begin(mock)
	mock.ExpectRollback()
	var saved context.Context
	err := manager.WithTransaction(context.Background(), func(ctx context.Context) error {
		saved = ctx
		if !errors.Is(foreign.DB(ctx).Exec("SELECT 1").Error, db.ErrDifferentDatabase) {
			t.Error("foreign query was not rejected")
		}
		if err := foreignManager.WithTransaction(ctx, func(context.Context) error { t.Fatal("foreign callback executed"); return nil }); !errors.Is(err, db.ErrDifferentDatabase) {
			t.Errorf("foreign transaction was not rejected: %v", err)
		}
		return nil
	})
	if !errors.Is(err, db.ErrDifferentDatabase) {
		t.Fatalf("got %v", err)
	}
	if !errors.Is(executor.DB(saved).Exec("SELECT 1").Error, db.ErrTransactionClosed) {
		t.Fatal("stale query escaped to the pool")
	}
	if err := manager.WithTransaction(saved, func(context.Context) error { t.Fatal("stale callback executed"); return nil }); !errors.Is(err, db.ErrTransactionClosed) {
		t.Fatalf("got %v", err)
	}
	if executor.DB(context.Background()).Error != nil {
		t.Fatal("shared GORM handle was modified")
	}
}

func TestExecutorPropagatesDerivedContext(t *testing.T) {
	executor, manager, mock := mockTransactions(t)
	begin(mock)
	commit(mock)
	err := manager.WithTransaction(context.Background(), func(ctx context.Context) error {
		child, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if executor.DB(child).Statement.Context != child {
			t.Fatal("child deadline/cancellation was lost")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCancelledContextDoesNotStartTransaction(t *testing.T) {
	_, manager, _ := mockTransactions(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := manager.WithTransaction(ctx, func(context.Context) error { t.Fatal("cancelled callback executed"); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestParallelTransactionsHaveSeparateConnections(t *testing.T) {
	executor, manager, mock := mockTransactions(t)
	mock.MatchExpectationsInOrder(false)
	for range 2 {
		begin(mock)
		commit(mock)
	}
	connections := make(chan gorm.ConnPool, 2)
	proceed := make(chan struct{})
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			if err := manager.WithTransaction(context.Background(), func(ctx context.Context) error {
				connections <- executor.DB(ctx).Statement.ConnPool
				<-proceed
				return nil
			}); err != nil {
				t.Error(err)
			}
		})
	}
	first, second := <-connections, <-connections
	close(proceed)
	workers.Wait()
	if first == second {
		t.Fatal("parallel operations shared an SQL transaction")
	}
}

// Ensure the fixture uses the real database/sql transaction type.
var _ gorm.ConnPool = (*sql.Tx)(nil)

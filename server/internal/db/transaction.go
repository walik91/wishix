package db

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/cockroach-go/v2/crdb"
	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbgorm"
	"gorm.io/gorm"
)

var (
	ErrTransactionClosed  = errors.New("transaction context is no longer active")
	ErrDifferentDatabase  = errors.New("transaction context belongs to a different database")
	ErrTransactionAborted = errors.New("transaction aborted by a recovered nested panic")
)

type transactionKey struct{}

// A scope exists for one callback attempt, even when CockroachDB reuses the
// underlying SQL transaction while retrying with its restart savepoint.
type transactionScope struct {
	root   *gorm.DB
	tx     *gorm.DB
	active atomic.Bool
	mu     sync.Mutex
	err    error
}

func (scope *transactionScope) fail(err error) {
	if err == nil {
		return
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.err == nil {
		scope.err = err
	}
}

func (scope *transactionScope) failure() error {
	scope.mu.Lock()
	defer scope.mu.Unlock()
	return scope.err
}

func scopeFrom(ctx context.Context) *transactionScope {
	scope, _ := ctx.Value(transactionKey{}).(*transactionScope)
	return scope
}

// Executor is immutable and safe to share between repositories through Fx.
// Each query must use DB(ctx) with the context received by that repository.
type Executor struct{ root *gorm.DB }

func NewExecutor(database *gorm.DB) *Executor { return &Executor{root: database} }

func (executor *Executor) validate(scope *transactionScope) error {
	if scope.root != executor.root {
		return ErrDifferentDatabase
	}
	if !scope.active.Load() {
		return ErrTransactionClosed
	}
	return nil
}

// DB returns a fresh GORM session using either the shared pool or the current
// transaction. Invalid scopes return a session with Error set; they never fall
// back to the pool and cannot execute a query outside the intended transaction.
func (executor *Executor) DB(ctx context.Context) *gorm.DB {
	if scope := scopeFrom(ctx); scope != nil {
		if err := executor.validate(scope); err != nil {
			scope.fail(err)
			session := executor.root.WithContext(ctx)
			_ = session.AddError(err)
			return session
		}
		return scope.tx.WithContext(ctx)
	}
	return executor.root.WithContext(ctx)
}

// TransactionManager owns transaction boundaries, not entity CRUD methods.
type TransactionManager struct{ executor *Executor }

func NewTransactionManager(executor *Executor) *TransactionManager {
	return &TransactionManager{executor: executor}
}

// WithTransaction runs fn synchronously in a SERIALIZABLE transaction. Nested
// calls join the active transaction. A nested failure makes the entire attempt
// fail, even if its caller accidentally suppresses that error.
//
// CockroachDB may run fn more than once: keep external side effects out of fn,
// propagate its txCtx to every repository call, and create attempt-local state
// inside fn. Do not retain txCtx or use the transaction concurrently.
func (manager *TransactionManager) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return errors.New("transaction callback is nil")
	}
	if scope := scopeFrom(ctx); scope != nil {
		if err := manager.executor.validate(scope); err != nil {
			scope.fail(err)
			return err
		}
		return invoke(ctx, scope, fn)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	transactionCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	transactionCtx = crdb.WithRetryPolicy(transactionCtx, &crdb.ExpBackoffRetryPolicy{
		RetryLimit: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
	})
	return crdbgorm.ExecuteTx(transactionCtx, manager.executor.root,
		&sql.TxOptions{Isolation: sql.LevelSerializable}, func(tx *gorm.DB) error {
			scope := &transactionScope{root: manager.executor.root, tx: tx}
			scope.active.Store(true)
			defer scope.active.Store(false)
			return invoke(context.WithValue(transactionCtx, transactionKey{}, scope), scope, fn)
		})
}

func invoke(ctx context.Context, scope *transactionScope, fn func(context.Context) error) (err error) {
	defer func() {
		if value := recover(); value != nil {
			scope.fail(ErrTransactionAborted)
			panic(value)
		}
	}()
	if err = ctx.Err(); err != nil {
		scope.fail(err)
		return err
	}
	if err = scope.failure(); err != nil {
		return err
	}
	if err = fn(ctx); err != nil {
		scope.fail(err)
	}
	if err = ctx.Err(); err != nil {
		scope.fail(err)
	}
	return scope.failure()
}

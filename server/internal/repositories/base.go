package repositories

import (
	"context"

	"gorm.io/gorm"

	"wishix/server/internal/db"
)

// Base provides the context-aware query entry point shared by repositories.
// Embed it and initialize it with NewBase in each repository constructor.
type Base struct{ executor *db.Executor }

func NewBase(executor *db.Executor) Base { return Base{executor: executor} }

// Ctx returns a fresh GORM session bound to ctx and its active transaction,
// or the shared pool when ctx has no transaction. Invalid transaction contexts
// produce a session with Error set and never fall back to the shared pool.
func (r *Base) Ctx(ctx context.Context) *gorm.DB { return r.executor.DB(ctx) }

// Package repositories contains CockroachDB access implemented with GORM.
// Repositories embed Base and start queries with Ctx(ctx).
// Auth is the first persistence scenario; business rules belong in services.
// Do not introduce fake repositories for API liveness.
package repositories

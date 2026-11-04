package services

import (
	"context"
	"database/sql"
)

type HealthStatus struct {
	Status  string
	Service string
}

type Health struct{ database *sql.DB }

func NewHealth(database *sql.DB) *Health {
	return &Health{database: database}
}

func (health *Health) Ready(ctx context.Context) HealthStatus {
	if err := health.database.PingContext(ctx); err != nil {
		return HealthStatus{Status: "unavailable", Service: "wishix-api"}
	}
	return health.Status()
}

// Status checks application liveness, independently of database availability.
func (*Health) Status() HealthStatus {
	return HealthStatus{Status: "ok", Service: "wishix-api"}
}

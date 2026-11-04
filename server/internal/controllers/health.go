package controllers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"wishix/server/internal/services"
)

type Health struct {
	service *services.Health
	logger  *slog.Logger
}

func NewHealth(service *services.Health, logger *slog.Logger) *Health {
	return &Health{service: service, logger: logger}
}

func (controller *Health) Get(w http.ResponseWriter, _ *http.Request) {
	controller.write(w, controller.service.Status(), http.StatusOK)
}

func (controller *Health) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	status := controller.service.Ready(ctx)
	code := http.StatusOK
	if status.Status != "ok" {
		code = http.StatusServiceUnavailable
	}
	controller.write(w, status, code)
}

func (controller *Health) write(w http.ResponseWriter, status services.HealthStatus, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}{Status: status.Status, Service: status.Service}); err != nil {
		controller.logger.Error("write health response", "error", err)
	}
}

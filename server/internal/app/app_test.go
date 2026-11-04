package app_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"go.uber.org/fx"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"wishix/server/internal/app"
	"wishix/server/internal/db"
	"wishix/server/internal/services"
)

func TestHTTPServerLifecycle(t *testing.T) {
	var server *http.Server
	var executor *db.Executor
	var transactor services.Transactor
	application := httpTestApp(t, fx.Populate(&server, &executor, &transactor))
	if err := application.Err(); err != nil {
		t.Fatal(err)
	}
	server.Addr = "127.0.0.1:0"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := application.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		if err := application.Stop(stopCtx); err != nil {
			t.Error(err)
		}
	})

	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + server.Addr + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var status struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || status.Status != "ok" || status.Service != "wishix-api" {
		t.Fatalf("unexpected health response: HTTP %d, %+v", response.StatusCode, status)
	}

	// Liveness stays healthy while readiness reports database unavailability.
	ready, err := client.Get("http://" + server.Addr + "/api/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer ready.Body.Close()
	var readiness map[string]string
	if err := json.NewDecoder(ready.Body).Decode(&readiness); err != nil {
		t.Fatal(err)
	}
	if ready.StatusCode != http.StatusServiceUnavailable || readiness["status"] != "unavailable" || len(readiness) != 2 {
		t.Fatalf("readiness must report a generic 503 without database error details: %d %+v", ready.StatusCode, readiness)
	}

	if err := application.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		t.Fatalf("Fx did not release the HTTP listening port: %v", err)
	}
	_ = listener.Close()
}

func TestHTTPServerRejectsOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var server *http.Server
	application := httpTestApp(t, fx.Populate(&server))
	if err := application.Err(); err != nil {
		t.Fatal(err)
	}
	server.Addr = listener.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := application.Start(ctx); err == nil {
		_ = application.Stop(ctx)
		t.Fatal("Fx reported successful startup while the HTTP port was already occupied")
	}
}

// HTTP lifecycle tests do not need a running database. Replacing the GORM
// provider keeps database startup hooks out of these focused tests.
func httpTestApp(t *testing.T, options ...fx.Option) *fx.App {
	t.Helper()
	connection, err := sql.Open("pgx", "postgresql://unused@127.0.0.1:1/unused?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	database, err := gorm.Open(postgres.New(postgres.Config{Conn: connection}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	return app.New(append(options, fx.Replace(database))...)
}

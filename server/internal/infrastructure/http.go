package infrastructure

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"go.uber.org/fx"

	"wishix/server/internal/controllers"
)

func NewMux(health *controllers.Health, auth *controllers.Auth, wishes *controllers.Wishes, productImport *controllers.ProductImport, lists *controllers.Lists) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health.Get)
	mux.HandleFunc("GET /api/ready", health.Ready)
	auth.RegisterRoutes(mux)
	wishes.RegisterRoutes(mux)
	productImport.RegisterRoutes(mux)
	lists.RegisterRoutes(mux)
	return mux
}

type Schema interface{ Apply(context.Context) error }

func NewHTTPServer(lifecycle fx.Lifecycle, mux *http.ServeMux, logger *slog.Logger, shutdowner fx.Shutdowner, schema Schema) *http.Server {
	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      25 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := schema.Apply(ctx); err != nil {
				logger.Error("database migration failed", "error", err)
				return errors.New("database migration failed; check schema and migration versions")
			}
			listener, err := net.Listen("tcp", server.Addr)
			if err != nil {
				return err
			}
			server.Addr = listener.Addr().String()
			logger.Info("Wishix API started", "address", server.Addr)
			go func() {
				if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Error("HTTP server failed", "error", err)
					if err := shutdowner.Shutdown(fx.ExitCode(1)); err != nil {
						logger.Error("request application shutdown", "error", err)
					}
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
	return server
}

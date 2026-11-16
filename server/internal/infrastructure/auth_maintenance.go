package infrastructure

import (
	"context"
	"log/slog"
	"time"

	"go.uber.org/fx"
	"wishix/server/internal/services"
)

func StartAuthMaintenance(lifecycle fx.Lifecycle, auth *services.Auth, google *services.Google, logger *slog.Logger) {
	var cancel context.CancelFunc
	done := make(chan struct{})
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			var ctx context.Context
			ctx, cancel = context.WithCancel(context.Background())
			go func() {
				defer close(done)
				ticker := time.NewTicker(time.Hour)
				defer ticker.Stop()
				// The HTTP startup hook has already applied migrations.
				for {
					task, stop := context.WithTimeout(ctx, 30*time.Second)
					if err := auth.Cleanup(task); err != nil && ctx.Err() == nil {
						logger.Error("authentication maintenance failed")
					}
					if err := google.Cleanup(task); err != nil && ctx.Err() == nil {
						logger.Error("Google authentication maintenance failed")
					}
					stop()
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
					}
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			select {
			case <-done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}

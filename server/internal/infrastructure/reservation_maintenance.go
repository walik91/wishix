package infrastructure

import (
	"context"
	"log/slog"
	"time"

	"go.uber.org/fx"
	"wishix/server/internal/services"
)

func StartReservationMaintenance(lifecycle fx.Lifecycle, bookings *services.Reservations, logger *slog.Logger) {
	var cancel context.CancelFunc
	done := make(chan struct{})
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			var ctx context.Context
			ctx, cancel = context.WithCancel(context.Background())
			go func() {
				defer close(done)
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for {
					work, stop := context.WithTimeout(ctx, 20*time.Second)
					if err := bookings.ProcessNotifications(work); err != nil && ctx.Err() == nil {
						logger.Error("reservation notification delivery failed")
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

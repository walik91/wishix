package app

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"gorm.io/gorm"

	"wishix/server/internal/auth"
	"wishix/server/internal/controllers"
	"wishix/server/internal/db"
	"wishix/server/internal/infrastructure"
	"wishix/server/internal/productimport"
	"wishix/server/internal/repositories"
	"wishix/server/internal/services"
)

// New assembles dependencies and verifies the database before starting HTTP.
func New(options ...fx.Option) *fx.App {
	defaults := []fx.Option{
		fx.Provide(
			func() *slog.Logger { return slog.New(slog.NewJSONHandler(os.Stdout, nil)) },
			services.NewHealth,
			controllers.NewHealth,
			auth.LoadConfig,
			auth.NewMailer,
			func(mailer *auth.Mailer) services.CodeMailer { return mailer },
			fx.Annotate(repositories.NewAuth, fx.As(new(services.AuthRepository))),
			services.NewAuth,
			controllers.NewAuth,
			auth.NewGoogleClient,
			func(provider *auth.GoogleClient) services.GoogleProvider { return provider },
			fx.Annotate(repositories.NewGoogle, fx.As(new(services.GoogleRepository))),
			services.NewGoogle,
			controllers.NewGoogle,
			auth.NewFacebookClient,
			func(provider *auth.FacebookClient) services.FacebookProvider { return provider },
			fx.Annotate(repositories.NewFacebook, fx.As(new(services.FacebookRepository))),
			services.NewFacebook,
			controllers.NewFacebook,
			fx.Annotate(repositories.NewWishes, fx.As(new(services.WishRepository))),
			services.NewWishes,
			controllers.NewWishes,
			fx.Annotate(repositories.NewLists, fx.As(new(services.ListRepository))),
			services.NewLists,
			controllers.NewLists,
			fx.Annotate(repositories.NewReservations, fx.As(new(services.ReservationRepository))),
			func(mailer *auth.Mailer) services.ReservationMailer { return mailer },
			services.NewReservations,
			controllers.NewReservations,
			func() *productimport.Importer { return productimport.New() },
			services.NewProductImport,
			controllers.NewProductImport,
			fx.Annotate(db.NewMigrator, fx.As(new(infrastructure.Schema))),
			infrastructure.NewMux,
			infrastructure.NewHTTPServer,
			db.New,
			db.SQL,
			db.NewExecutor,
			fx.Annotate(db.NewTransactionManager, fx.As(new(services.Transactor))),
		),
		fx.WithLogger(func(logger *slog.Logger) fxevent.Logger {
			return &fxevent.SlogLogger{Logger: logger}
		}),
		fx.Invoke(func(*gorm.DB) {}, func(*http.Server) {}, infrastructure.StartAuthMaintenance, infrastructure.StartReservationMaintenance),
		fx.StartTimeout(60 * time.Second),
		fx.StopTimeout(5 * time.Second),
	}

	return fx.New(append(defaults, options...)...)
}

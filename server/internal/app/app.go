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
			fx.Annotate(auth.NewMailer, fx.As(new(services.CodeMailer))),
			fx.Annotate(repositories.NewAuth, fx.As(new(services.AuthRepository))),
			services.NewAuth,
			controllers.NewAuth,
			fx.Annotate(repositories.NewWishes, fx.As(new(services.WishRepository))),
			services.NewWishes,
			controllers.NewWishes,
			fx.Annotate(repositories.NewLists, fx.As(new(services.ListRepository))),
			services.NewLists,
			controllers.NewLists,
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
		fx.Invoke(func(*gorm.DB) {}, func(*http.Server) {}, infrastructure.StartAuthMaintenance),
		fx.StartTimeout(60 * time.Second),
		fx.StopTimeout(5 * time.Second),
	}

	return fx.New(append(defaults, options...)...)
}

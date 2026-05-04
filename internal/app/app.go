package app

import (
	"context"
	"fmt"
	"log/slog"

	grpcapp "github.com/acyushka/oregon-resource-service/internal/app/grpc"
	"github.com/acyushka/oregon-resource-service/internal/config"
	"github.com/acyushka/oregon-resource-service/internal/repository/postgres"
	service "github.com/acyushka/oregon-resource-service/internal/service/resource"
)

type App struct {
	GRPC *grpcapp.App
	repo *postgres.Repository
	log  *slog.Logger
}

func New(ctx context.Context, cfg *config.Config, log *slog.Logger) (*App, error) {
	if log == nil {
		log = slog.Default()
	}

	repo, err := postgres.New(ctx, makeDSN(cfg.Database), log)
	if err != nil {
		return nil, fmt.Errorf("app.New: init postgres: %w", err)
	}
	log.InfoContext(ctx, "postgres initialized")

	resourceService := service.NewService(repo, log)

	grpcServer := grpcapp.New(
		cfg.GRPC.Booking.Port,
		cfg.GRPC.Public.Port,
		resourceService,
		resourceService,
		log,
	)

	return &App{
		GRPC: grpcServer,
		repo: repo,
		log:  log,
	}, nil
}

func (a *App) MustRun() {
	a.GRPC.MustRun()
}

func (a *App) Run() error {
	a.log.InfoContext(context.Background(), "starting grpc app")
	return a.GRPC.Run()
}

func (a *App) Stop() error {
	a.log.InfoContext(context.Background(), "stopping grpc app")
	a.GRPC.Stop()
	if err := a.repo.Close(); err != nil {
		return fmt.Errorf("app.Stop: close repository: %w", err)
	}
	a.log.InfoContext(context.Background(), "repository closed")

	return nil
}

func makeDSN(cfg config.Database) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
	)
}

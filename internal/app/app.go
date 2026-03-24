package app

import (
	"context"
	"fmt"

	grpcapp "github.com/acyushka/oregon-resource-service/internal/app/grpc"
	"github.com/acyushka/oregon-resource-service/internal/config"
	"github.com/acyushka/oregon-resource-service/internal/repository/postgres"
	service "github.com/acyushka/oregon-resource-service/internal/service/resource"
)

type App struct {
	GRPC *grpcapp.App
	repo *postgres.Repository
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {

	repo, err := postgres.New(ctx, makeDSN(cfg.Database))
	if err != nil {
		return nil, fmt.Errorf("app.New: init postgres: %w", err)
	}

	resourceService := service.NewService(repo)

	grpcServer := grpcapp.New(
		cfg.GRPC.Booking.Port,
		cfg.GRPC.Public.Port,
		resourceService,
		resourceService,
	)

	return &App{
		GRPC: grpcServer,
		repo: repo,
	}, nil
}

func (a *App) MustRun() {
	a.GRPC.MustRun()
}

func (a *App) Run() error {
	return a.GRPC.Run()
}

func (a *App) Stop() error {
	a.GRPC.Stop()
	if err := a.repo.Close(); err != nil {
		return fmt.Errorf("app.Stop: close repository: %w", err)
	}

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

package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/OnYyon/oregon-api-gateway/pkg/logger"
	"github.com/OnYyon/oregon-api-gateway/pkg/observability/tracer"
	"github.com/acyushka/oregon-resource-service/internal/app"
	"github.com/acyushka/oregon-resource-service/internal/config"
)

func main() {
	cfg := config.MustLoad()

	if err := os.MkdirAll("logs", 0o750); err != nil {
		panic(err)
	}
	logFile, err := os.OpenFile("logs/resource.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			slog.ErrorContext(context.Background(), "failed to close log file", slog.Any("error", err))
		}
	}()

	logCfg := &logger.Config{
		Level:       slog.LevelInfo,
		Format:      "json",
		AddSource:   false,
		Out:         io.MultiWriter(os.Stdout, logFile),
		ServiceName: "resource-service",
		Environment: cfg.Env,
	}
	log := logger.New(logCfg)
	slog.SetDefault(log)

	tracerProvider, err := tracer.New(context.Background(), &tracer.Config{
		ServiceName: "ResourceService",
		EndPoint:    cfg.Tracer.EndPoint,
		Insecure:    cfg.Tracer.Insecure,
		SampleRatio: cfg.Tracer.SampleRatio,
	})
	if err != nil {
		log.ErrorContext(context.Background(), "failed to init tracer", slog.Any("error", err))
	}

	defer func() {
		if err := tracerProvider.Shutdown(context.Background()); err != nil {
			log.ErrorContext(context.Background(), "failed to shutdown tracer", slog.Any("error", err))
		}
	}()

	application, err := app.New(context.Background(), cfg, log)
	if err != nil {
		log.ErrorContext(context.Background(), "failed to init app", slog.Any("error", err))
		os.Exit(1)
	}

	log.InfoContext(context.Background(), "application initialized")

	stopCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-stopCtx.Done()
		log.InfoContext(context.Background(), "shutdown signal received")

		if err := application.Stop(); err != nil {
			log.ErrorContext(context.Background(), "failed to stop app", slog.Any("error", err))
			return
		}

		log.InfoContext(context.Background(), "application stopped")
	}()

	if err := application.Run(); err != nil {
		log.ErrorContext(context.Background(), "application run failed", slog.Any("error", err))
		os.Exit(1)
	}

}

package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/acyushka/oregon-resource-service/internal/app"
	"github.com/acyushka/oregon-resource-service/internal/config"
)

func main() {
	cfg := config.MustLoad()

	application, err := app.New(context.Background(), cfg)
	if err != nil {
		panic(fmt.Errorf("init app: %w", err))
	}

	stopCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-stopCtx.Done()
		if err := application.Stop(); err != nil {
			panic(fmt.Errorf("stop app: %w", err))
		}
	}()

	if err := application.Run(); err != nil {
		panic(fmt.Errorf("run app: %w", err))
	}

}

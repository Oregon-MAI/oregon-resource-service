package grpcapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"

	bookinghandler "github.com/acyushka/oregon-resource-service/internal/grpc/resource/booking"
	publichandler "github.com/acyushka/oregon-resource-service/internal/grpc/resource/public"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type App struct {
	booking *serverUnit
	public  *serverUnit
	log     *slog.Logger
}

type serverUnit struct {
	name string
	port int

	server   *grpc.Server
	listener net.Listener
}

func New(
	bookingPort int,
	publicPort int,
	bookingService bookinghandler.ResourceServiceBooking,
	publicService publichandler.ResourceServicePublic,
	log *slog.Logger,
) *App {
	if log == nil {
		log = slog.Default()
	}

	bookingServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		rpcLoggingUnaryInterceptor(log),
		recoveryUnaryInterceptor(log),
	))
	publicServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		rpcLoggingUnaryInterceptor(log),
		recoveryUnaryInterceptor(log),
	))

	reflection.Register(bookingServer)
	reflection.Register(publicServer)

	bookinghandler.NewServer(bookingServer, bookingService)
	publichandler.NewServer(publicServer, publicService)

	return &App{
		booking: &serverUnit{
			name:   "booking",
			port:   bookingPort,
			server: bookingServer,
		},
		public: &serverUnit{
			name:   "public",
			port:   publicPort,
			server: publicServer,
		},
		log: log,
	}
}

func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		panic(err)
	}
}

func (a *App) Run() error {
	const op = "grpcapp.Run"

	units := []*serverUnit{a.booking, a.public}
	a.log.Info("starting grpc servers", slog.Int("servers_count", len(units)))

	for _, unit := range units {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", unit.port))
		if err != nil {
			a.Stop()
			return fmt.Errorf("%s: listen %s: %w", op, unit.name, err)
		}
		unit.listener = listener

		a.log.Info("grpc listener started", slog.String("server", unit.name), slog.String("addr", listener.Addr().String()))
	}

	var (
		group    errgroup.Group
		stopOnce sync.Once
	)

	for _, unit := range units {
		u := unit

		group.Go(func() error {
			a.log.Info("grpc server serving", slog.String("server", u.name), slog.Int("port", u.port))

			err := u.server.Serve(u.listener)
			if err == nil || errors.Is(err, grpc.ErrServerStopped) {
				a.log.Info("grpc server stopped", slog.String("server", u.name))
				return nil
			}

			stopOnce.Do(a.Stop)
			a.log.Error("grpc server serve failed", slog.String("server", u.name), slog.Any("error", err))

			return fmt.Errorf("%s: serve %s: %w", op, u.name, err)
		})
	}

	return group.Wait()
}

func (a *App) Stop() {
	a.log.Info("graceful stopping grpc servers")

	var wg sync.WaitGroup
	for _, unit := range []*serverUnit{a.booking, a.public} {
		wg.Add(1)
		go func(s *serverUnit) {
			defer wg.Done()
			a.log.Info("stopping grpc server", slog.String("server", s.name), slog.Int("port", s.port))
			s.server.GracefulStop()
		}(unit)
	}
	wg.Wait()
}

func recoveryUnaryInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic recovered in grpc handler", slog.String("method", info.FullMethod), slog.Any("panic", recovered))
				err = status.Error(codes.Internal, "internal error")
			}
		}()

		return handler(ctx, req)
	}
}

func rpcLoggingUnaryInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		resp, err := handler(ctx, req)

		if err != nil {
			log.Warn(
				"grpc request failed",
				slog.String("method", info.FullMethod),
				slog.String("grpc_code", status.Code(err).String()),
				slog.Any("error", err),
			)
		}

		return resp, err
	}
}

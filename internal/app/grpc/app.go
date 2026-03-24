package grpcapp

import (
	"context"
	"errors"
	"fmt"
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
) *App {
	bookingServer := grpc.NewServer(grpc.UnaryInterceptor(recoveryUnaryInterceptor()))
	publicServer := grpc.NewServer(grpc.UnaryInterceptor(recoveryUnaryInterceptor()))

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
	for _, unit := range units {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", unit.port))
		if err != nil {
			a.Stop()
			return fmt.Errorf("%s: listen %s: %w", op, unit.name, err)
		}
		unit.listener = listener
	}

	var (
		group    errgroup.Group
		stopOnce sync.Once
	)

	for _, unit := range units {
		u := unit

		group.Go(func() error {
			err := u.server.Serve(u.listener)
			if err == nil || errors.Is(err, grpc.ErrServerStopped) {
				return nil
			}

			stopOnce.Do(a.Stop)

			return fmt.Errorf("%s: serve %s: %w", op, u.name, err)
		})
	}

	return group.Wait()
}

func (a *App) Stop() {
	var wg sync.WaitGroup
	for _, unit := range []*serverUnit{a.booking, a.public} {
		wg.Add(1)
		go func(s *serverUnit) {
			defer wg.Done()
			s.server.GracefulStop()
		}(unit)
	}
	wg.Wait()
}

func recoveryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = status.Error(codes.Internal, "internal error")
			}
		}()

		return handler(ctx, req)
	}
}

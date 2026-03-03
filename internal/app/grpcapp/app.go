package grpcapp

import (
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"
)

type App struct {
	log    *slog.Logger
	server *grpc.Server
	addr   string
	lis    net.Listener
}

func New(log *slog.Logger, server *grpc.Server, addr string) (*App, error) {
	if log == nil {
		return nil, fmt.Errorf("logger is nil")
	}
	if server == nil {
		return nil, fmt.Errorf("grpc server is nil")
	}
	if addr == "" {
		return nil, fmt.Errorf("grpc addr is empty")
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen grpc: %w", err)
	}

	return &App{
		log:    log,
		server: server,
		addr:   addr,
		lis:    lis,
	}, nil
}

func (a *App) Run() error {
	const op = "grpcapp.Run"

	log := a.log.With(
		slog.String("op", op),
		slog.String("addr", a.lis.Addr().String()),
	)

	log.Info("grpc server started")

	if err := a.server.Serve(a.lis); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (a *App) Stop() {
	const op = "grpcapp.Stop"

	a.log.With(
		slog.String("op", op),
		slog.String("addr", a.addr),
	).Info("stopping grpc server")

	a.server.GracefulStop()
}

func (a *App) Addr() string {
	if a == nil || a.lis == nil {
		return a.addr
	}
	return a.lis.Addr().String()
}

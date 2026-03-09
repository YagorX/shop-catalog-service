package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	health "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	grpcapp "github.com/YagorX/shop-catalog-service/internal/app/grpcapp"
	httpapp "github.com/YagorX/shop-catalog-service/internal/app/httpapp"
	"github.com/YagorX/shop-catalog-service/internal/config"
	"github.com/YagorX/shop-catalog-service/internal/observability"
	cachedrepo "github.com/YagorX/shop-catalog-service/internal/repository/cached"
	postgresrepo "github.com/YagorX/shop-catalog-service/internal/repository/postgres"
	redisrepo "github.com/YagorX/shop-catalog-service/internal/repository/redis"
	catalogsvc "github.com/YagorX/shop-catalog-service/internal/service/catalog"
	grpcHandlers "github.com/YagorX/shop-catalog-service/internal/transport/grpc/v1/handlers"
	httpv1 "github.com/YagorX/shop-catalog-service/internal/transport/http/v1"
	catalogv1 "github.com/YagorX/shop-contracts/gen/go/proto/catalog/v1"
	_ "github.com/jackc/pgx/v5/stdlib"
	goredis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type App struct {
	logger *slog.Logger

	httpApp      *httpapp.App
	grpcApp      *grpcapp.App
	healthServer *health.Server
	errCh        chan error

	db          *sql.DB
	redisClient *goredis.Client

	shutdownTracing func(context.Context) error
}

type readinessChecker struct {
	db    *sql.DB
	redis *goredis.Client
}

func (c *readinessChecker) Check(ctx context.Context) error {
	if c == nil {
		return errors.New("readiness checker is nil")
	}
	if c.db == nil {
		return errors.New("postgres is not initialized")
	}
	if c.redis == nil {
		return errors.New("redis is not initialized")
	}
	if err := c.db.PingContext(ctx); err != nil {
		return fmt.Errorf("postgres not ready: %w", err)
	}
	if err := c.redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis not ready: %w", err)
	}
	return nil
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	runtimeLogger := observability.NewLogger(observability.LoggerOptions{
		Service: cfg.ServiceName,
		Env:     cfg.Env,
		Version: cfg.Version,
		Level:   cfg.LogLevel,
	})
	observability.SetDefaultLogger(runtimeLogger.Logger)

	shutdownTracing, err := observability.InitTracing(
		ctx,
		cfg.ServiceName,
		cfg.Version,
		cfg.Env,
		cfg.OTLP.Endpoint,
	)
	if err != nil {
		return nil, fmt.Errorf("init tracing: %w", err)
	}

	db, err := sql.Open("pgx", cfg.PostgresDSN())
	if err != nil {
		_ = shutdownTracing(context.Background())
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		_ = shutdownTracing(context.Background())
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	redisClient := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	redisPingCtx, redisPingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer redisPingCancel()

	if err := redisClient.Ping(redisPingCtx).Err(); err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		_ = shutdownTracing(context.Background())
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	postgresRepo, err := postgresrepo.NewProductRepository(runtimeLogger.Logger, db)
	if err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		_ = shutdownTracing(context.Background())
		return nil, fmt.Errorf("create postgres repository: %w", err)
	}

	cache, err := redisrepo.NewCache(runtimeLogger.Logger, redisClient, cfg.Redis.TTL)
	if err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		_ = shutdownTracing(context.Background())
		return nil, fmt.Errorf("create redis cache: %w", err)
	}

	repo, err := cachedrepo.NewProductRepository(runtimeLogger.Logger, cache, postgresRepo)
	if err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		_ = shutdownTracing(context.Background())
		return nil, fmt.Errorf("create cached repository: %w", err)
	}

	catalogService, err := catalogsvc.NewCatalogService(runtimeLogger.Logger, repo)
	if err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		_ = shutdownTracing(context.Background())
		return nil, fmt.Errorf("create catalog service: %w", err)
	}

	grpcHandler, err := grpcHandlers.NewHandler(catalogService)
	if err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		_ = shutdownTracing(context.Background())
		return nil, fmt.Errorf("create grpc handler: %w", err)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(observability.GRPCServerStatsHandler()),
	)
	catalogv1.RegisterCatalogServiceServer(grpcServer, grpcHandler)
	grpcHealth := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, grpcHealth)
	grpcHealth.SetServingStatus("proto.catalog.v1.CatalogService", healthpb.HealthCheckResponse_SERVING)
	reflection.Register(grpcServer)

	httpRouter := httpv1.NewRouter(httpv1.RouterDeps{
		LogLevelController: runtimeLogger,
		ReadinessChecker: &readinessChecker{
			db:    db,
			redis: redisClient,
		},
	})

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr(),
		Handler:           httpRouter,
		ReadHeaderTimeout: 5 * time.Second,
	}

	grpcRuntime, err := grpcapp.New(runtimeLogger.Logger, grpcServer, cfg.GRPCAddr())
	if err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		_ = shutdownTracing(context.Background())
		return nil, fmt.Errorf("create grpc app: %w", err)
	}

	httpRuntime, err := httpapp.New(runtimeLogger.Logger, httpServer)
	if err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		_ = shutdownTracing(context.Background())
		return nil, fmt.Errorf("create http app: %w", err)
	}

	return &App{
		logger:          runtimeLogger.Logger,
		httpApp:         httpRuntime,
		grpcApp:         grpcRuntime,
		db:              db,
		redisClient:     redisClient,
		shutdownTracing: shutdownTracing,
		healthServer:    grpcHealth,
		errCh:           make(chan error, 2),
	}, nil
}

func (a *App) Run() error {
	if a == nil {
		return errors.New("app is nil")
	}

	go func() {
		if err := a.grpcApp.Run(); err != nil {
			a.errCh <- fmt.Errorf("grpc app failed: %w", err)
		}
	}()

	go func() {
		if err := a.httpApp.Run(); err != nil {
			a.errCh <- fmt.Errorf("http app failed: %w", err)
		}
	}()

	a.logger.Info("catalog service bootstrap completed",
		slog.String("grpc_addr", a.grpcApp.Addr()),
		slog.String("http_addr", a.httpApp.Addr()),
		slog.String("repository_backend", "postgres+redis-cache"),
	)

	return nil
}

func (a *App) Errors() <-chan error {
	if a == nil {
		return nil
	}
	return a.errCh
}

func (a *App) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}

	var shutdownErr error

	if a.grpcApp != nil {
		a.healthServer.SetServingStatus("proto.catalog.v1.CatalogService", healthpb.HealthCheckResponse_NOT_SERVING)
		a.grpcApp.Stop()
	}

	if a.httpApp != nil {
		if err := a.httpApp.Stop(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("stop http app: %w", err))
		}
	}

	if a.redisClient != nil {
		if err := a.redisClient.Close(); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("close redis client: %w", err))
		}
	}

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("close postgres db: %w", err))
		}
	}

	if a.shutdownTracing != nil {
		if err := a.shutdownTracing(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("shutdown tracing: %w", err))
		}
	}

	a.logger.Info("catalog service stopped")

	return shutdownErr
}

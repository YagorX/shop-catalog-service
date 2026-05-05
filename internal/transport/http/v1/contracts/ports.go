package contracts

import (
	"context"
	"log/slog"

	"github.com/YagorX/shop-catalog-service/internal/domain"
)

type LogLevelController interface {
	SetLevel(level string) error
	Level() slog.Level
}

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type ReadinessChecker interface {
	Check(ctx context.Context) error
}

type CatalogService interface {
	CreateProduct(ctx context.Context, cmd domain.CreateProductCommand) (domain.Product, error)
	UpdateProduct(ctx context.Context, cmd domain.UpdateProductCommand) (domain.Product, error)
	UpdateStock(ctx context.Context, productID string, delta int32) (domain.Product, error)
}

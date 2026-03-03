package catalog

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/YagorX/shop-catalog-service/internal/domain"
	"github.com/YagorX/shop-catalog-service/internal/observability"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

type ProductRepository interface {
	List(ctx context.Context, limit, offset int) ([]domain.Product, error)
	GetByID(ctx context.Context, id string) (domain.Product, error)
}

type CatalogService struct {
	logger     *slog.Logger
	repository ProductRepository
}

func NewCatalogService(logger *slog.Logger, repository ProductRepository) (*CatalogService, error) {
	if logger == nil {
		return nil, errors.New("logger is empty")
	}
	if repository == nil {
		return nil, errors.New("repository is empty")
	}
	return &CatalogService{logger: logger, repository: repository}, nil
}

func (service *CatalogService) ListProducts(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	const op = "service.catalog.ListProducts"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	ctx, span := otel.Tracer("catalog-service/internal/service/catalog").Start(ctx, "service.catalog.ListProducts")
	defer span.End()

	span.SetAttributes(
		attribute.Int("catalog.limit", limit),
		attribute.Int("catalog.offset", offset),
	)

	defer func() {
		metrics.CatalogRequestDuration.WithLabelValues("ListProducts").Observe(time.Since(startedAt).Seconds())
	}()

	if service == nil || service.repository == nil || service.logger == nil {
		err := errors.New("catalog service is not initialized")
		slog.Error("catalog service is not initialized", slog.String("op", op))
		metrics.CatalogRequestsTotal.WithLabelValues("ListProducts", "error").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, "catalog service is not initialized")
		return nil, err
	}

	originalLimit := limit
	service.logger.Debug("list products started",
		slog.String("op", op),
		slog.Int("limit", limit),
		slog.Int("offset", offset),
	)

	if offset < 0 {
		service.logger.Warn("invalid pagination offset",
			slog.String("op", op),
			slog.Int("offset", offset),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		metrics.CatalogRequestsTotal.WithLabelValues("ListProducts", "invalid_argument").Inc()
		span.RecordError(domain.ErrInvalidPagination)
		span.SetStatus(codes.Error, domain.ErrInvalidPagination.Error())
		return nil, domain.ErrInvalidPagination
	}

	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	products, err := service.repository.List(ctx, limit, offset)
	if err != nil {
		service.logger.Error("repository list failed",
			slog.String("op", op),
			slog.Int("limit", originalLimit),
			slog.Int("effective_limit", limit),
			slog.Int("offset", offset),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		metrics.CatalogRequestsTotal.WithLabelValues("ListProducts", "error").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	service.logger.Info("list products completed",
		slog.String("op", op),
		slog.Int("limit", originalLimit),
		slog.Int("effective_limit", limit),
		slog.Int("offset", offset),
		slog.Int("result_count", len(products)),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)
	metrics.CatalogRequestsTotal.WithLabelValues("ListProducts", "success").Inc()
	span.SetAttributes(attribute.Int("catalog.result_count", len(products)))
	span.SetStatus(codes.Ok, "success")

	return products, nil
}

func (service *CatalogService) GetProduct(ctx context.Context, id string) (domain.Product, error) {
	const op = "service.catalog.GetProduct"
	startedAt := time.Now()
	metrics := observability.MustMetrics()

	ctx, span := otel.Tracer("catalog-service/internal/service/catalog").Start(ctx, "service.catalog.GetProduct")
	defer span.End()

	span.SetAttributes(
		attribute.String("catalog.product_id", id),
	)

	defer func() {
		metrics.CatalogRequestDuration.WithLabelValues("GetProduct").Observe(time.Since(startedAt).Seconds())
	}()

	if service == nil || service.repository == nil || service.logger == nil {
		err := errors.New("catalog service is not initialized")
		slog.Error("catalog service is not initialized", slog.String("op", op))
		metrics.CatalogRequestsTotal.WithLabelValues("GetProduct", "error").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, "catalog service is not initialized")
		return domain.Product{}, err
	}

	service.logger.Debug("get product started",
		slog.String("op", op),
		slog.String("product_id", id),
	)

	if strings.TrimSpace(id) == "" {
		service.logger.Warn("product id is required",
			slog.String("op", op),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		metrics.CatalogRequestsTotal.WithLabelValues("GetProduct", "invalid_argument").Inc()
		err := errors.New("product id is required")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domain.Product{}, err
	}

	product, err := service.repository.GetByID(ctx, id)
	if err != nil {
		level := slog.LevelError
		msg := "repository get product failed"
		if errors.Is(err, domain.ErrProductNotFound) {
			level = slog.LevelWarn
			msg = "product not found"
		}

		service.logger.Log(ctx, level, msg,
			slog.String("op", op),
			slog.String("product_id", id),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		status := "error"
		if errors.Is(err, domain.ErrProductNotFound) {
			status = "not_found"
		}
		metrics.CatalogRequestsTotal.WithLabelValues("GetProduct", status).Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domain.Product{}, err
	}

	service.logger.Info("get product completed",
		slog.String("op", op),
		slog.String("product_id", id),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)
	metrics.CatalogRequestsTotal.WithLabelValues("GetProduct", "success").Inc()

	span.SetAttributes(attribute.String("catalog.result_product_id", product.ID))
	span.SetStatus(codes.Ok, "success")

	return product, nil
}

package in_memory

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/YagorX/shop-catalog-service/internal/domain"
	catalogsvc "github.com/YagorX/shop-catalog-service/internal/service/catalog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type ProductRepository struct {
	logger   *slog.Logger
	mu       sync.RWMutex
	products []domain.Product
	byID     map[string]domain.Product
}

func NewProductRepository(logger *slog.Logger, seed []domain.Product) (*ProductRepository, error) {
	if len(seed) == 0 {
		seed = defaultProducts()
	}
	if logger == nil {
		logger = slog.Default()
	}

	repo := &ProductRepository{
		logger:   logger,
		products: make([]domain.Product, 0, len(seed)),
		byID:     make(map[string]domain.Product, len(seed)),
	}

	for _, p := range seed {
		if err := p.Validate(); err != nil {
			return nil, err
		}
		repo.products = append(repo.products, p)
		repo.byID[p.ID] = p
	}

	return repo, nil
}

func (r *ProductRepository) List(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	const op = "repository.in_memory.ProductRepository.List"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/in_memory").Start(ctx, op)
	defer span.End()

	span.SetAttributes(
		attribute.Int("repository.limit", limit),
		attribute.Int("repository.offset", offset),
	)

	r.logger.Debug("repository list started",
		slog.String("op", op),
		slog.Int("limit", limit),
		slog.Int("offset", offset),
	)

	select {
	case <-ctx.Done():
		r.logger.Warn("repository list canceled",
			slog.String("op", op),
			slog.String("error", ctx.Err().Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(ctx.Err())
		span.SetStatus(codes.Error, ctx.Err().Error())
		return nil, ctx.Err()
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if offset >= len(r.products) {
		r.logger.Debug("repository list completed",
			slog.String("op", op),
			slog.Int("limit", limit),
			slog.Int("offset", offset),
			slog.Int("result_count", 0),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.SetAttributes(attribute.Int("repository.result_count", 0))
		span.SetStatus(codes.Ok, "success")
		return []domain.Product{}, nil
	}

	end := offset + limit
	if end > len(r.products) {
		end = len(r.products)
	}

	out := make([]domain.Product, 0, end-offset)
	out = append(out, r.products[offset:end]...)

	r.logger.Debug("repository list completed",
		slog.String("op", op),
		slog.Int("limit", limit),
		slog.Int("offset", offset),
		slog.Int("result_count", len(out)),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	span.SetAttributes(attribute.Int("repository.total_len", len(out)))
	span.SetStatus(codes.Ok, "success")

	return out, nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id string) (domain.Product, error) {
	const op = "repository.in_memory.ProductRepository.GetByID"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/in_memory").Start(ctx, op)
	defer span.End()

	span.SetAttributes(
		attribute.String("repository.product_id", id),
	)

	r.logger.Debug("repository get by id started",
		slog.String("op", op),
		slog.String("product_id", id),
	)

	select {
	case <-ctx.Done():
		r.logger.Warn("repository get by id canceled",
			slog.String("op", op),
			slog.String("product_id", id),
			slog.String("error", ctx.Err().Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(ctx.Err())
		span.SetStatus(codes.Error, ctx.Err().Error())
		return domain.Product{}, ctx.Err()
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.byID[id]
	if !ok {
		r.logger.Warn("repository product not found",
			slog.String("op", op),
			slog.String("product_id", id),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(domain.ErrProductNotFound)
		span.SetStatus(codes.Error, domain.ErrProductNotFound.Error())
		return domain.Product{}, domain.ErrProductNotFound
	}

	r.logger.Debug("repository get by id completed",
		slog.String("op", op),
		slog.String("product_id", id),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	span.SetAttributes(attribute.String("repository.result_product_id", p.ID))
	span.SetStatus(codes.Ok, "success")

	return p, nil
}

var _ catalogsvc.ProductRepository = (*ProductRepository)(nil)

func defaultProducts() []domain.Product {
	return []domain.Product{
		{
			ID:          "prod-001",
			SKU:         "SKU-TSHIRT-001",
			Name:        "Go T-Shirt",
			Description: "Soft cotton t-shirt with Go gopher print",
			PriceCents:  2499,
			Currency:    "USD",
			Stock:       42,
			Active:      true,
		},
		{
			ID:          "prod-002",
			SKU:         "SKU-MUG-001",
			Name:        "Observability Mug",
			Description: "Ceramic mug for logs, metrics and traces discussions",
			PriceCents:  1599,
			Currency:    "USD",
			Stock:       30,
			Active:      true,
		},
		{
			ID:          "prod-003",
			SKU:         "SKU-STICKER-001",
			Name:        "Telemetry Stickers Pack",
			Description: "Stickers pack with Kafka, Prometheus and Jaeger themes",
			PriceCents:  799,
			Currency:    "USD",
			Stock:       120,
			Active:      true,
		},
		{
			ID:          "prod-004",
			SKU:         "SKU-HOODIE-001",
			Name:        "SRE Hoodie",
			Description: "Warm hoodie for incident response nights",
			PriceCents:  5999,
			Currency:    "USD",
			Stock:       12,
			Active:      true,
		},
		{
			ID:          "prod-005",
			SKU:         "SKU-NOTEBOOK-001",
			Name:        "Runbook Notebook",
			Description: "Notebook for runbooks, postmortems and debug notes",
			PriceCents:  1299,
			Currency:    "USD",
			Stock:       55,
			Active:      true,
		},
	}
}

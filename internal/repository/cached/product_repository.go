package cached

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/YagorX/shop-catalog-service/internal/domain"
	"github.com/YagorX/shop-catalog-service/internal/observability"
	rediscache "github.com/YagorX/shop-catalog-service/internal/repository/redis"
	catalogsvc "github.com/YagorX/shop-catalog-service/internal/service/catalog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type ProductRepository struct {
	logger *slog.Logger
	cache  *rediscache.Cache
	next   catalogsvc.ProductRepository
}

func NewProductRepository(
	logger *slog.Logger,
	cache *rediscache.Cache,
	next catalogsvc.ProductRepository,
) (*ProductRepository, error) {
	if next == nil {
		return nil, errors.New("next repository is empty")
	}
	if cache == nil {
		return nil, errors.New("cache is empty")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &ProductRepository{
		logger: logger,
		cache:  cache,
		next:   next,
	}, nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id string) (domain.Product, error) {
	const op = "repository.cached.ProductRepository.GetByID"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/cached").Start(ctx, op)
	defer span.End()

	metrics := observability.MustMetrics()

	cacheGetStartedAt := time.Now()
	defer func() {
		metrics.CacheRequestDuration.WithLabelValues("GetProduct", "get").Observe(time.Since(cacheGetStartedAt).Seconds())
	}()

	span.SetAttributes(attribute.String("repository.product_id", id))

	r.logger.Debug("cached get by id started",
		slog.String("op", op),
		slog.String("product_id", id),
	)

	product, hit, err := r.cache.GetProduct(ctx, id)
	if err != nil {
		r.logger.Error("cache get failed",
			slog.String("op", op),
			slog.String("product_id", id),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		metrics.CacheRequestsTotal.WithLabelValues("GetProduct", "error").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domain.Product{}, err
	}

	if hit {
		r.logger.Info("cache hit",
			slog.String("op", op),
			slog.String("product_id", id),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)

		metrics.CacheRequestsTotal.WithLabelValues("GetProduct", "hit").Inc()

		span.SetAttributes(
			attribute.Bool("cache.hit", true),
			attribute.String("repository.result_product_id", product.ID),
		)
		span.SetStatus(codes.Ok, "cache_hit")

		return product, nil
	}

	metrics.CacheRequestsTotal.WithLabelValues("GetProduct", "miss").Inc()

	r.logger.Info("cache miss",
		slog.String("op", op),
		slog.String("product_id", id),
	)

	span.SetAttributes(attribute.Bool("cache.hit", false))

	product, err = r.next.GetByID(ctx, id)
	if err != nil {
		level := slog.LevelError
		msg := "fallback repository get failed"
		if errors.Is(err, domain.ErrProductNotFound) {
			level = slog.LevelWarn
			msg = "product not found in fallback repository"
		}

		r.logger.Log(ctx, level, msg,
			slog.String("op", op),
			slog.String("product_id", id),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)

		metrics.CacheRequestsTotal.WithLabelValues("GetProduct", "error").Inc()

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domain.Product{}, err
	}

	cacheSetStartedAt := time.Now()
	if err := r.cache.SetProduct(ctx, product); err != nil {
		r.logger.Warn("cache set failed after fallback success",
			slog.String("op", op),
			slog.String("product_id", id),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)

		metrics.CacheRequestsTotal.WithLabelValues("GetProduct", "set_error").Inc()

		// Запрос не ломаем, потому что source of truth уже ответил.
		span.AddEvent("cache_set_failed")
		span.SetAttributes(attribute.String("cache.set_error", err.Error()))
	} else {
		metrics.CacheRequestDuration.WithLabelValues("GetProduct", "set").Observe(time.Since(cacheSetStartedAt).Seconds())
		r.logger.Debug("cache set completed after fallback success",
			slog.String("op", op),
			slog.String("product_id", id),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
	}

	r.logger.Info("cached get by id completed",
		slog.String("op", op),
		slog.String("product_id", id),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	span.SetAttributes(attribute.String("repository.result_product_id", product.ID))
	span.SetStatus(codes.Ok, "success")

	return product, nil
}

func (r *ProductRepository) List(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	const op = "repository.cached.ProductRepository.List"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/cached").Start(ctx, op)
	defer span.End()

	metrics := observability.MustMetrics()
	cacheGetStartedAt := time.Now()
	defer func() {
		metrics.CacheRequestDuration.WithLabelValues("ListProducts", "get").Observe(time.Since(cacheGetStartedAt).Seconds())
	}()

	span.SetAttributes(
		attribute.Int("repository.limit", limit),
		attribute.Int("repository.offset", offset),
	)

	r.logger.Debug("cached list started",
		slog.String("op", op),
		slog.Int("limit", limit),
		slog.Int("offset", offset),
	)

	products, hit, err := r.cache.GetProductList(ctx, limit, offset)
	if err != nil {
		r.logger.Error("cache get product list failed",
			slog.String("op", op),
			slog.Int("limit", limit),
			slog.Int("offset", offset),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		metrics.CacheRequestsTotal.WithLabelValues("ListProducts", "error").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if hit {
		r.logger.Info("product list cache hit",
			slog.String("op", op),
			slog.Int("limit", limit),
			slog.Int("offset", offset),
			slog.Int("result_count", len(products)),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)

		metrics.CacheRequestsTotal.WithLabelValues("ListProducts", "hit").Inc()
		span.SetAttributes(
			attribute.Bool("cache.hit", true),
			attribute.Int("repository.result_count", len(products)),
		)
		span.SetStatus(codes.Ok, "cache_hit")

		return products, nil
	}

	metrics.CacheRequestsTotal.WithLabelValues("ListProducts", "miss").Inc()

	r.logger.Info("product list cache miss",
		slog.String("op", op),
		slog.Int("limit", limit),
		slog.Int("offset", offset),
	)

	span.SetAttributes(attribute.Bool("cache.hit", false))

	products, err = r.next.List(ctx, limit, offset)
	if err != nil {
		r.logger.Error("fallback repository list failed",
			slog.String("op", op),
			slog.Int("limit", limit),
			slog.Int("offset", offset),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)

		metrics.CacheRequestsTotal.WithLabelValues("List", "error").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	cacheSetStartedAt := time.Now()
	if err := r.cache.SetProductList(ctx, limit, offset, products); err != nil {
		r.logger.Warn("cache set product list failed after fallback success",
			slog.String("op", op),
			slog.Int("limit", limit),
			slog.Int("offset", offset),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		metrics.CacheRequestsTotal.WithLabelValues("GetProductList", "set_error").Inc()
		span.AddEvent("cache_set_product_list_failed")
		span.SetAttributes(attribute.String("cache.set_error", err.Error()))
	} else {
		metrics.CacheRequestDuration.WithLabelValues("ListProducts", "set").Observe(time.Since(cacheSetStartedAt).Seconds())
		r.logger.Debug("cache set product list completed after fallback success",
			slog.String("op", op),
			slog.Int("limit", limit),
			slog.Int("offset", offset),
			slog.Int("result_count", len(products)),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
	}

	r.logger.Debug("cached list completed",
		slog.String("op", op),
		slog.Int("limit", limit),
		slog.Int("offset", offset),
		slog.Int("result_count", len(products)),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	span.SetAttributes(attribute.Int("repository.result_count", len(products)))
	span.SetStatus(codes.Ok, "success")

	return products, nil
}

func (r *ProductRepository) Create(ctx context.Context, cmd domain.CreateProductCommand) (domain.Product, error) {
	const op = "repository.cached.ProductRepository.Create"

	// Пишем в основное хранилище
	product, err := r.next.Create(ctx, cmd)
	if err != nil {
		return domain.Product{}, err
	}

	// Инвалидируем кеш списков — данные устарели
	// Простая стратегия: удаляем все ключи списков по паттерну
	if err := r.invalidateListCache(ctx); err != nil {
		// Не падаем — данные записаны, кеш инвалидируется по TTL
		r.logger.Warn("failed to invalidate list cache after create",
			slog.String("op", op),
			slog.String("product_id", product.ID),
			slog.String("error", err.Error()),
		)
	}

	r.logger.Info("cached create product completed",
		slog.String("op", op),
		slog.String("product_id", product.ID),
	)

	return product, nil
}

func (r *ProductRepository) Update(ctx context.Context, cmd domain.UpdateProductCommand) (domain.Product, error) {
	const op = "repository.cached.ProductRepository.Update"

	product, err := r.next.Update(ctx, cmd)
	if err != nil {
		return domain.Product{}, err
	}

	// Инвалидируем кеш конкретного товара
	if err := r.cache.DeleteProduct(ctx, product.ID); err != nil {
		r.logger.Warn("failed to invalidate product cache after update",
			slog.String("op", op),
			slog.String("product_id", product.ID),
			slog.String("error", err.Error()),
		)
	}

	// Инвалидируем кеш списков
	if err := r.invalidateListCache(ctx); err != nil {
		r.logger.Warn("failed to invalidate list cache after update",
			slog.String("op", op),
			slog.String("product_id", product.ID),
			slog.String("error", err.Error()),
		)
	}

	r.logger.Info("cached update product completed",
		slog.String("op", op),
		slog.String("product_id", product.ID),
	)

	return product, nil
}

func (r *ProductRepository) UpdateStock(ctx context.Context, productID string, delta int32) (domain.Product, error) {
	const op = "repository.cached.ProductRepository.UpdateStock"

	product, err := r.next.UpdateStock(ctx, productID, delta)
	if err != nil {
		return domain.Product{}, err
	}

	// Инвалидируем кеш товара — остаток изменился
	if err := r.cache.DeleteProduct(ctx, productID); err != nil {
		r.logger.Warn("failed to invalidate product cache after stock update",
			slog.String("op", op),
			slog.String("product_id", productID),
			slog.String("error", err.Error()),
		)
	}

	// Инвалидируем кеш списков
	if err := r.invalidateListCache(ctx); err != nil {
		r.logger.Warn("failed to invalidate list cache after stock update",
			slog.String("op", op),
			slog.String("product_id", productID),
			slog.String("error", err.Error()),
		)
	}

	r.logger.Info("cached update stock completed",
		slog.String("op", op),
		slog.String("product_id", productID),
		slog.Int("delta", int(delta)),
		slog.Int("new_stock", int(product.Stock)),
	)

	return product, nil
}

// invalidateListCache удаляет все закешированные списки товаров
// используем SCAN + DEL — не блокирует Redis в отличие от KEYS
func (r *ProductRepository) invalidateListCache(ctx context.Context) error {
	// Получаем redis клиент через cache
	// Паттерн ключей списков: catalog:list:v1:*
	return r.cache.InvalidateListCache(ctx)
}

var _ catalogsvc.ProductRepository = (*ProductRepository)(nil)

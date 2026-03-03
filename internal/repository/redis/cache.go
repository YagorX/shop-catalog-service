package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/YagorX/shop-catalog-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type Cache struct {
	logger *slog.Logger
	client *redis.Client
	ttl    time.Duration
}

func NewCache(logger *slog.Logger, client *redis.Client, ttl time.Duration) (*Cache, error) {
	if client == nil {
		return nil, errors.New("redis client is empty")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if ttl <= 0 {
		return nil, errors.New("redis ttl must be > 0")
	}

	return &Cache{
		logger: logger,
		client: client,
		ttl:    ttl,
	}, nil
}

func (c *Cache) GetProduct(ctx context.Context, id string) (domain.Product, bool, error) {
	const op = "repository.redis.Cache.GetProduct"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/redis").Start(ctx, op)
	defer span.End()

	span.SetAttributes(attribute.String("redis.product_id", id))

	key := productKey(id)

	c.logger.Debug("redis get product started",
		slog.String("op", op),
		slog.String("product_id", id),
		slog.String("key", key),
	)

	raw, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("redis cache miss",
				slog.String("op", op),
				slog.String("product_id", id),
				slog.String("key", key),
				slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
			)
			span.SetAttributes(attribute.Bool("redis.hit", false))
			span.SetStatus(codes.Ok, "miss")
			return domain.Product{}, false, nil
		}

		c.logger.Error("redis get failed",
			slog.String("op", op),
			slog.String("product_id", id),
			slog.String("key", key),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domain.Product{}, false, err
	}

	var cached productCacheValue
	if err := json.Unmarshal([]byte(raw), &cached); err != nil {
		c.logger.Error("redis unmarshal failed",
			slog.String("op", op),
			slog.String("product_id", id),
			slog.String("key", key),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domain.Product{}, false, err
	}

	product := domain.Product{
		ID:          cached.ID,
		SKU:         cached.SKU,
		Name:        cached.Name,
		Description: cached.Description,
		PriceCents:  cached.PriceCents,
		Currency:    cached.Currency,
		Stock:       cached.Stock,
		Active:      cached.Active,
	}

	c.logger.Debug("redis cache hit",
		slog.String("op", op),
		slog.String("product_id", id),
		slog.String("key", key),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	span.SetAttributes(
		attribute.Bool("redis.hit", true),
		attribute.String("redis.key", key),
	)
	span.SetStatus(codes.Ok, "hit")

	return product, true, nil
}

func (c *Cache) SetProduct(ctx context.Context, product domain.Product) error {
	const op = "repository.redis.Cache.SetProduct"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/redis").Start(ctx, op)
	defer span.End()

	key := productKey(product.ID)

	span.SetAttributes(
		attribute.String("redis.product_id", product.ID),
		attribute.String("redis.key", key),
	)

	c.logger.Debug("redis set product started",
		slog.String("op", op),
		slog.String("product_id", product.ID),
		slog.String("key", key),
	)

	payload, err := json.Marshal(productCacheValue{
		ID:          product.ID,
		SKU:         product.SKU,
		Name:        product.Name,
		Description: product.Description,
		PriceCents:  product.PriceCents,
		Currency:    product.Currency,
		Stock:       product.Stock,
		Active:      product.Active,
	})
	if err != nil {
		c.logger.Error("redis marshal failed",
			slog.String("op", op),
			slog.String("product_id", product.ID),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if err := c.client.Set(ctx, key, payload, c.ttl).Err(); err != nil {
		c.logger.Error("redis set failed",
			slog.String("op", op),
			slog.String("product_id", product.ID),
			slog.String("key", key),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	c.logger.Debug("redis set product completed",
		slog.String("op", op),
		slog.String("product_id", product.ID),
		slog.String("key", key),
		slog.Duration("ttl", c.ttl),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	span.SetAttributes(attribute.String("redis.ttl", c.ttl.String()))
	span.SetStatus(codes.Ok, "success")

	return nil
}

func (c *Cache) DeleteProduct(ctx context.Context, id string) error {
	const op = "repository.redis.Cache.DeleteProduct"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/redis").Start(ctx, op)
	defer span.End()

	key := productKey(id)

	span.SetAttributes(
		attribute.String("redis.product_id", id),
		attribute.String("redis.key", key),
	)

	c.logger.Debug("redis delete product started",
		slog.String("op", op),
		slog.String("product_id", id),
		slog.String("key", key),
	)

	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.logger.Error("redis delete failed",
			slog.String("op", op),
			slog.String("product_id", id),
			slog.String("key", key),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	c.logger.Debug("redis delete product completed",
		slog.String("op", op),
		slog.String("product_id", id),
		slog.String("key", key),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	span.SetStatus(codes.Ok, "success")
	return nil
}

func (c *Cache) GetProductList(ctx context.Context, limit, offset int) ([]domain.Product, bool, error) {
	const op = "repository.redis.Cache.GetProductList"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/redis").Start(ctx, op)
	defer span.End()

	key := productListKey(limit, offset)

	span.SetAttributes(
		attribute.Int("redis.limit", limit),
		attribute.Int("redis.offset", offset),
		attribute.String("redis.key", key),
	)

	c.logger.Debug("redis get product list started",
		slog.String("op", op),
		slog.Int("limit", limit),
		slog.Int("offset", offset),
		slog.String("key", key),
	)

	raw, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("redis product list cache miss",
				slog.String("op", op),
				slog.Int("limit", limit),
				slog.Int("offset", offset),
				slog.String("key", key),
				slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
			)
			span.SetAttributes(attribute.Bool("redis.hit", false))
			span.SetStatus(codes.Ok, "miss")
			return nil, false, nil
		}

		c.logger.Error("redis get product list failed",
			slog.String("op", op),
			slog.Int("limit", limit),
			slog.Int("offset", offset),
			slog.String("key", key),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, false, err
	}

	var cached productListCacheValue
	if err := json.Unmarshal([]byte(raw), &cached); err != nil {
		c.logger.Error("redis unmarshal failed",
			slog.String("op", op),
			slog.Int("limit", limit),
			slog.Int("offset", offset),
			slog.String("key", key),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, false, err
	}

	products := make([]domain.Product, 0, len(cached.Items))

	for _, item := range cached.Items {
		products = append(products, domain.Product{
			ID:          item.ID,
			SKU:         item.SKU,
			Name:        item.Name,
			Description: item.Description,
			PriceCents:  item.PriceCents,
			Currency:    item.Currency,
			Stock:       item.Stock,
			Active:      item.Active,
		})
	}

	c.logger.Debug("redis product list cache hit",
		slog.String("op", op),
		slog.Int("limit", limit),
		slog.Int("offset", offset),
		slog.String("key", key),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	span.SetAttributes(
		attribute.Bool("redis.hit", true),
		attribute.String("redis.key", key),
	)
	span.SetStatus(codes.Ok, "hit")

	return products, true, nil
}

func (c *Cache) SetProductList(ctx context.Context, limit, offset int, products []domain.Product) error {
	const op = "repository.redis.Cache.SetProductList"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/redis").Start(ctx, op)
	defer span.End()

	key := productListKey(limit, offset)

	span.SetAttributes(
		attribute.Int("redis.limit", limit),
		attribute.Int("redis.offset", offset),
		attribute.String("redis.key", key),
		attribute.Int("redis.result_count", len(products)),
	)

	c.logger.Debug("redis set product list started",
		slog.String("op", op),
		slog.Int("limit", limit),
		slog.Int("offset", offset),
		slog.String("key", key),
		slog.Int("redis.result_count", len(products)),
	)

	items := make([]productCacheValue, 0, len(products))
	for _, product := range products {
		items = append(items, productCacheValue{
			ID:          product.ID,
			SKU:         product.SKU,
			Name:        product.Name,
			Description: product.Description,
			PriceCents:  product.PriceCents,
			Currency:    product.Currency,
			Stock:       product.Stock,
			Active:      product.Active,
		})
	}

	payload, err := json.Marshal(productListCacheValue{
		Items: items,
	})
	if err != nil {
		c.logger.Error("redis marshal product list failed",
			slog.String("op", op),
			slog.Int("limit", limit),
			slog.Int("offset", offset),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	if err := c.client.Set(ctx, key, payload, c.ttl).Err(); err != nil {
		c.logger.Error("redis set product list failed",
			slog.String("op", op),
			slog.Int("limit", limit),
			slog.Int("offset", offset),
			slog.String("key", key),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	c.logger.Debug("redis set product list completed",
		slog.String("op", op),
		slog.Int("limit", limit),
		slog.Int("offset", offset),
		slog.String("key", key),
		slog.Duration("ttl", c.ttl),
		slog.Int("result_count", len(products)),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	span.SetAttributes(attribute.String("redis.ttl", c.ttl.String()))
	span.SetStatus(codes.Ok, "success")

	return nil
}

func productKey(id string) string {
	return fmt.Sprintf("catalog:product:%s", id)
}

func productListKey(limit, offset int) string {
	return fmt.Sprintf("catalog:list:v1:limit=%d:offset=%d", limit, offset)
}

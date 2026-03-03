package postgres

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/YagorX/shop-catalog-service/internal/domain"
	catalogsvc "github.com/YagorX/shop-catalog-service/internal/service/catalog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type ProductRepository struct {
	logger *slog.Logger
	db     *sql.DB
}

func NewProductRepository(logger *slog.Logger, db *sql.DB) (*ProductRepository, error) {
	if db == nil {
		return nil, errors.New("db is empty")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &ProductRepository{
		logger: logger,
		db:     db,
	}, nil
}

func (r *ProductRepository) List(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	const op = "repository.postgres.ProductRepository.List"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/postgres").Start(ctx, op)
	defer span.End()

	span.SetAttributes(
		attribute.Int("repository.limit", limit),
		attribute.Int("repository.offset", offset),
	)

	r.logger.Debug("postgres list started",
		slog.String("op", op),
		slog.Int("limit", limit),
		slog.Int("offset", offset),
	)

	query := `
		SELECT id, sku, name, description, price_cents, currency, stock, active
		FROM products
		ORDER BY id
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		r.logger.Error("postgres list query failed",
			slog.String("op", op),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	defer rows.Close()

	products := make([]domain.Product, 0, limit)
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			r.logger.Error("postgres list scan failed",
				slog.String("op", op),
				slog.String("error", err.Error()),
				slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
			)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("postgres list rows iteration failed",
			slog.String("op", op),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	r.logger.Debug("postgres list completed",
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

func scanProduct(scanner interface {
	Scan(dest ...any) error
}) (domain.Product, error) {
	var product domain.Product

	err := scanner.Scan(
		&product.ID,
		&product.SKU,
		&product.Name,
		&product.Description,
		&product.PriceCents,
		&product.Currency,
		&product.Stock,
		&product.Active,
	)
	if err != nil {
		return domain.Product{}, err
	}

	return product, nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id string) (domain.Product, error) {
	const op = "repository.postgres.ProductRepository.GetByID"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/postgres").Start(ctx, op)
	defer span.End()

	span.SetAttributes(attribute.String("repository.product_id", id))

	r.logger.Debug("postgres get by id started",
		slog.String("op", op),
		slog.String("product_id", id),
	)

	query := `
		SELECT id, sku, name, description, price_cents, currency, stock, active
		FROM products
		WHERE id = $1
	`

	row := r.db.QueryRowContext(ctx, query, id)

	var product domain.Product
	err := row.Scan(
		&product.ID,
		&product.SKU,
		&product.Name,
		&product.Description,
		&product.PriceCents,
		&product.Currency,
		&product.Stock,
		&product.Active,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.logger.Warn("postgres product not found",
				slog.String("op", op),
				slog.String("product_id", id),
				slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
			)
			span.RecordError(domain.ErrProductNotFound)
			span.SetStatus(codes.Error, domain.ErrProductNotFound.Error())
			return domain.Product{}, domain.ErrProductNotFound
		}

		r.logger.Error("postgres get by id failed",
			slog.String("op", op),
			slog.String("product_id", id),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domain.Product{}, err
	}

	r.logger.Debug("postgres get by id completed",
		slog.String("op", op),
		slog.String("product_id", id),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	span.SetAttributes(attribute.String("repository.result_product_id", product.ID))
	span.SetStatus(codes.Ok, "success")

	return product, nil
}

var _ catalogsvc.ProductRepository = (*ProductRepository)(nil)

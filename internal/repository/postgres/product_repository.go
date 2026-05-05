package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/YagorX/shop-catalog-service/internal/domain"
	"github.com/YagorX/shop-catalog-service/internal/events"
	catalogsvc "github.com/YagorX/shop-catalog-service/internal/service/catalog"
	"github.com/google/uuid"
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

func (r *ProductRepository) Create(ctx context.Context, cmd domain.CreateProductCommand) (domain.Product, error) {
	const op = "repository.postgres.ProductRepository.Create"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/postgres").Start(ctx, op)
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Product{}, err
	}
	defer tx.Rollback()

	id := uuid.New().String()

	query := `
        INSERT INTO products (id, sku, name, description, price_cents, currency, stock, active)
        VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE)
        RETURNING id, sku, name, description, price_cents, currency, stock, active
    `

	row := tx.QueryRowContext(ctx, query,
		id,
		cmd.SKU,
		cmd.Name,
		cmd.Description,
		cmd.PriceCents,
		cmd.Currency,
		cmd.Stock,
	)

	product, err := scanProduct(row)
	if err != nil {
		// Нарушение уникального ключа (SKU уже существует)
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			span.RecordError(domain.ErrProductAlreadyExists)
			span.SetStatus(codes.Error, domain.ErrProductAlreadyExists.Error())
			return domain.Product{}, domain.ErrProductAlreadyExists
		}
		r.logger.Error("postgres create product failed",
			slog.String("op", op),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domain.Product{}, err
	}

	if err := r.insertOutboxEvent(ctx, tx, events.EventProductCreated, product); err != nil {
		return domain.Product{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Product{}, fmt.Errorf("commit create product transaction: %w", err)
	}

	r.logger.Info("postgres create product completed",
		slog.String("op", op),
		slog.String("product_id", product.ID),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)
	span.SetStatus(codes.Ok, "success")
	return product, nil
}

func (r *ProductRepository) Update(ctx context.Context, cmd domain.UpdateProductCommand) (domain.Product, error) {
	const op = "repository.postgres.ProductRepository.Update"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/postgres").Start(ctx, op)
	defer span.End()

	// Динамически строим запрос — обновляем только те поля которые переданы
	setClauses := []string{"updated_at = NOW()"}
	args := []any{}
	argIdx := 1

	if cmd.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *cmd.Name)
		argIdx++
	}
	if cmd.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *cmd.Description)
		argIdx++
	}
	if cmd.PriceCents != nil {
		setClauses = append(setClauses, fmt.Sprintf("price_cents = $%d", argIdx))
		args = append(args, *cmd.PriceCents)
		argIdx++
	}
	if cmd.Active != nil {
		setClauses = append(setClauses, fmt.Sprintf("active = $%d", argIdx))
		args = append(args, *cmd.Active)
		argIdx++
	}

	args = append(args, cmd.ID)
	query := fmt.Sprintf(`
        UPDATE products
        SET %s
        WHERE id = $%d
        RETURNING id, sku, name, description, price_cents, currency, stock, active
    `, strings.Join(setClauses, ", "), argIdx)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Product{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, query, args...)
	product, err := scanProduct(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.RecordError(domain.ErrProductNotFound)
			span.SetStatus(codes.Error, domain.ErrProductNotFound.Error())
			return domain.Product{}, domain.ErrProductNotFound
		}
		r.logger.Error("postgres update product failed",
			slog.String("op", op),
			slog.String("product_id", cmd.ID),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domain.Product{}, err
	}

	if err := r.insertOutboxEvent(ctx, tx, events.EventProductUpdated, product); err != nil {
		return domain.Product{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Product{}, fmt.Errorf("commit update product transaction: %w", err)
	}

	r.logger.Info("postgres update product completed",
		slog.String("op", op),
		slog.String("product_id", product.ID),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)
	span.SetStatus(codes.Ok, "success")
	return product, nil
}

func (r *ProductRepository) UpdateStock(ctx context.Context, productID string, delta int32) (domain.Product, error) {
	const op = "repository.postgres.ProductRepository.UpdateStock"
	startedAt := time.Now()

	ctx, span := otel.Tracer("catalog-service/internal/repository/postgres").Start(ctx, op)
	defer span.End()

	// delta может быть отрицательным (списание) или положительным (поступление)
	// CHECK constraint в БД не даст stock уйти в минус
	query := `
        UPDATE products
        SET stock = stock + $1,
            updated_at = NOW()
        WHERE id = $2
        RETURNING id, sku, name, description, price_cents, currency, stock, active
    `

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Product{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, query, delta, productID)
	product, err := scanProduct(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.RecordError(domain.ErrProductNotFound)
			span.SetStatus(codes.Error, domain.ErrProductNotFound.Error())
			return domain.Product{}, domain.ErrProductNotFound
		}
		// CHECK constraint сработал — stock ушёл бы в минус
		if strings.Contains(err.Error(), "check") || strings.Contains(err.Error(), "stock") {
			span.RecordError(domain.ErrInsufficientStock)
			span.SetStatus(codes.Error, domain.ErrInsufficientStock.Error())
			return domain.Product{}, domain.ErrInsufficientStock
		}
		r.logger.Error("postgres update stock failed",
			slog.String("op", op),
			slog.String("product_id", productID),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domain.Product{}, err
	}

	if err := r.insertOutboxEvent(ctx, tx, events.EventStockChanged, product); err != nil {
		return domain.Product{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Product{}, fmt.Errorf("commit update stock transaction: %w", err)
	}

	r.logger.Info("postgres update stock completed",
		slog.String("op", op),
		slog.String("product_id", productID),
		slog.Int64("delta", int64(delta)),
		slog.Int64("new_stock", int64(product.Stock)),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)
	span.SetStatus(codes.Ok, "success")

	return product, nil
}

func (r *ProductRepository) insertOutboxEvent(ctx context.Context, tx *sql.Tx, eventType string, product domain.Product) error {
	eventID := uuid.New().String()

	event := events.ProductEvent{
		EventID:    eventID,
		EventType:  eventType,
		ProductID:  product.ID,
		SKU:        product.SKU,
		Name:       product.Name,
		PriceCents: product.PriceCents,
		Currency:   product.Currency,
		Stock:      product.Stock,
		Active:     product.Active,
		OccuredAt:  time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		r.logger.Error("failed to marshal product event",
			slog.String("event_id", eventID),
			slog.String("event_type", eventType),
			slog.String("product_id", product.ID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("marshal outbox event: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
        INSERT INTO outbox_events (
            id,
            topic,
            message_key,
            event_type,
            payload
        )
        VALUES ($1, $2, $3, $4, $5)
    `,
		eventID,
		"catalog.products.v1",
		product.ID,
		eventType,
		payload,
	)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}

	return nil
}

var _ catalogsvc.ProductRepository = (*ProductRepository)(nil)

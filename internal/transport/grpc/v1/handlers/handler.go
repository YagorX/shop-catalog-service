package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/YagorX/shop-catalog-service/internal/domain"
	"github.com/YagorX/shop-catalog-service/internal/observability"
	catalogsvc "github.com/YagorX/shop-catalog-service/internal/service/catalog"
	catalogv1 "github.com/YagorX/shop-contracts/gen/go/proto/catalog/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	catalogv1.UnimplementedCatalogServiceServer
	svc *catalogsvc.CatalogService
}

const maxLogValueLen = 256

func NewHandler(svc *catalogsvc.CatalogService) (*Handler, error) {
	if svc == nil {
		return nil, errors.New("catalog service is nil")
	}
	return &Handler{svc: svc}, nil
}

func (h *Handler) ListProducts(ctx context.Context, req *catalogv1.ListProductsRequest) (*catalogv1.ListProductsResponse, error) {
	const op = "transport.grpc.catalog.v1.ListProducts"
	const method = "ListProducts"
	startedAt := time.Now()
	metrics := observability.MustMetrics()

	defer func() {
		metrics.GRPCRequestDuration.WithLabelValues(method).Observe(time.Since(startedAt).Seconds())
	}()

	slog.Debug("grpc request started",
		slog.String("op", op),
		slog.Uint64("limit", uint64(req.GetLimit())),
		slog.Uint64("offset", uint64(req.GetOffset())),
	)

	if err := validateListProductsRequest(req); err != nil {
		metrics.GRPCRequestsTotal.WithLabelValues(method, codes.InvalidArgument.String()).Inc()
		slog.Warn("grpc request validation failed",
			slog.String("op", op),
			slog.String("error", truncate(err.Error(), maxLogValueLen)),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		return nil, err
	}

	products, err := h.svc.ListProducts(ctx, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		mapped := mapError(err)
		code := status.Code(mapped)
		metrics.GRPCRequestsTotal.WithLabelValues(method, code.String()).Inc()

		level := slog.LevelError
		if code == codes.NotFound || code == codes.InvalidArgument {
			level = slog.LevelWarn
		}

		slog.Log(ctx, level, "grpc request failed",
			slog.String("op", op),
			slog.String("error", truncate(mapped.Error(), maxLogValueLen)),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		return nil, mapped
	}

	items := make([]*catalogv1.Product, 0, len(products))
	for _, product := range products {
		items = append(items, toProtoProduct(product))
	}

	resp := &catalogv1.ListProductsResponse{
		Items: items,
		Total: uint32(len(items)),
	}

	metrics.GRPCRequestsTotal.WithLabelValues(method, codes.OK.String()).Inc()
	slog.Info("grpc request completed",
		slog.String("op", op),
		slog.Int("result_count", len(items)),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	return resp, nil
}

func (h *Handler) GetProduct(ctx context.Context, req *catalogv1.GetProductRequest) (*catalogv1.GetProductResponse, error) {
	const op = "transport.grpc.catalog.v1.GetProduct"
	const method = "GetProduct"
	startedAt := time.Now()
	metrics := observability.MustMetrics()

	defer func() {
		metrics.GRPCRequestDuration.WithLabelValues(method).Observe(time.Since(startedAt).Seconds())
	}()

	slog.Debug("grpc request started",
		slog.String("op", op),
		slog.String("product_id", truncate(req.GetId(), maxLogValueLen)),
	)

	if err := validateGetProductRequest(req); err != nil {
		metrics.GRPCRequestsTotal.WithLabelValues(method, codes.InvalidArgument.String()).Inc()
		slog.Warn("grpc request validation failed",
			slog.String("op", op),
			slog.String("error", truncate(err.Error(), maxLogValueLen)),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		return nil, err
	}

	product, err := h.svc.GetProduct(ctx, req.GetId())
	if err != nil {
		mapped := mapError(err)
		code := status.Code(mapped)
		metrics.GRPCRequestsTotal.WithLabelValues(method, code.String()).Inc()

		level := slog.LevelError
		if code == codes.NotFound || code == codes.InvalidArgument {
			level = slog.LevelWarn
		}
		slog.Log(ctx, level, "grpc request failed",
			slog.String("op", op),
			slog.String("product_id", truncate(req.GetId(), maxLogValueLen)),
			slog.String("error", truncate(mapped.Error(), maxLogValueLen)),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
		return nil, mapped
	}

	resp := &catalogv1.GetProductResponse{
		Product: toProtoProduct(product),
	}

	metrics.GRPCRequestsTotal.WithLabelValues(method, codes.OK.String()).Inc()
	slog.Info("grpc request completed",
		slog.String("op", op),
		slog.String("product_id", truncate(req.GetId(), maxLogValueLen)),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	return resp, nil
}

func validateListProductsRequest(req *catalogv1.ListProductsRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}

	if req.GetLimit() > 1000 {
		return status.Error(codes.InvalidArgument, "limit must be <= 1000")
	}

	return nil
}

func validateGetProductRequest(req *catalogv1.GetProductRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if strings.TrimSpace(req.GetId()) == "" {
		return status.Error(codes.InvalidArgument, "id is required")
	}
	return nil
}

func toProtoProduct(p domain.Product) *catalogv1.Product {
	return &catalogv1.Product{
		Id:          p.ID,
		Sku:         p.SKU,
		Name:        p.Name,
		Description: p.Description,
		Price:       float64(p.PriceCents) / 100.0,
		Currency:    p.Currency,
		Stock:       uint32(p.Stock),
		Active:      p.Active,
		UpdatedAt:   "",
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrProductNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidPagination):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func truncate(value string, maxLen int) string {
	if maxLen <= 0 {
		return value
	}
	if len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	return value[:maxLen-3] + "..."
}

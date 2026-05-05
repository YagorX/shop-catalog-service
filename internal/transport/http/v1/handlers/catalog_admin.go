package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/YagorX/shop-catalog-service/internal/domain"
	"github.com/YagorX/shop-catalog-service/internal/transport/http/v1/contracts"
)

type CatalogAdminHandler struct {
	svc    contracts.CatalogService
	logger *slog.Logger
}

func NewCatalogAdminHandler(svc contracts.CatalogService, logger *slog.Logger) *CatalogAdminHandler {
	return &CatalogAdminHandler{svc: svc, logger: logger}
}

// --- Request/Response ---

type createProductRequest struct {
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Currency    string `json:"currency"`
	Stock       int32  `json:"stock"`
}

type updateProductRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	PriceCents  *int64  `json:"price_cents"`
	Active      *bool   `json:"active"`
}

type updateStockRequest struct {
	Delta int32 `json:"delta"`
}

type productResponse struct {
	ID          string `json:"id"`
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Currency    string `json:"currency"`
	Stock       int32  `json:"stock"`
	Active      bool   `json:"active"`
}

func toProductResponse(p domain.Product) productResponse {
	return productResponse{
		ID:          p.ID,
		SKU:         p.SKU,
		Name:        p.Name,
		Description: p.Description,
		PriceCents:  p.PriceCents,
		Currency:    p.Currency,
		Stock:       p.Stock,
		Active:      p.Active,
	}
}

// POST /admin/products
func (h *CatalogAdminHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req createProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	product, err := h.svc.CreateProduct(r.Context(), domain.CreateProductCommand{
		SKU:         req.SKU,
		Name:        req.Name,
		Description: req.Description,
		PriceCents:  req.PriceCents,
		Currency:    req.Currency,
		Stock:       req.Stock,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrProductAlreadyExists):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, domain.ErrInvalidProduct):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			h.logger.Error("create product failed", slog.String("error", err.Error()))
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, toProductResponse(product))
}

// PATCH /admin/products/{id}
func (h *CatalogAdminHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	productID := extractProductID(r.URL.Path)
	if productID == "" {
		writeError(w, http.StatusBadRequest, "product id is required")
		return
	}

	var req updateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	product, err := h.svc.UpdateProduct(r.Context(), domain.UpdateProductCommand{
		ID:          productID,
		Name:        req.Name,
		Description: req.Description,
		PriceCents:  req.PriceCents,
		Active:      req.Active,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrProductNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, domain.ErrInvalidProduct):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			h.logger.Error("update product failed", slog.String("error", err.Error()))
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, toProductResponse(product))
}

// POST /admin/products/{id}/stock
func (h *CatalogAdminHandler) UpdateStock(w http.ResponseWriter, r *http.Request) {
	// URL: /admin/products/prod-001/stock
	path := strings.TrimPrefix(r.URL.Path, "/admin/products/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "product id is required")
		return
	}
	productID := parts[0]

	var req updateStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	product, err := h.svc.UpdateStock(r.Context(), productID, req.Delta)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrProductNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, domain.ErrInsufficientStock):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			h.logger.Error("update stock failed", slog.String("error", err.Error()))
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, toProductResponse(product))
}

func extractProductID(path string) string {
	trimmed := strings.TrimPrefix(path, "/admin/products/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

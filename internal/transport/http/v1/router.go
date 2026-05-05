package v1

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/YagorX/shop-catalog-service/internal/transport/http/v1/contracts"
	"github.com/YagorX/shop-catalog-service/internal/transport/http/v1/handlers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type RouterDeps struct {
	LogLevelController contracts.LogLevelController
	ReadinessChecker   contracts.ReadinessChecker
	CatalogService     contracts.CatalogService
	Logger             *slog.Logger
}

func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	healthHandler := handlers.NewHealthHandler(deps.ReadinessChecker)
	adminHandler := handlers.NewAdminHandler(deps.LogLevelController)
	catalogAdmin := handlers.NewCatalogAdminHandler(deps.CatalogService, deps.Logger)

	// System
	mux.HandleFunc("/health", healthHandler.Health)
	mux.HandleFunc("/ready", healthHandler.Ready)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/admin/log-level", adminHandler.LogLevel)

	// POST /admin/products — создать товар
	mux.HandleFunc("/admin/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			catalogAdmin.CreateProduct(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// /admin/products/{id} и /admin/products/{id}/stock
	mux.HandleFunc("/admin/products/", func(w http.ResponseWriter, r *http.Request) {
		// POST /admin/products/{id}/stock — изменить остаток
		if strings.HasSuffix(r.URL.Path, "/stock") && r.Method == http.MethodPost {
			catalogAdmin.UpdateStock(w, r)
			return
		}
		// PATCH /admin/products/{id} — обновить товар
		switch r.Method {
		case http.MethodPatch:
			catalogAdmin.UpdateProduct(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return mux
}

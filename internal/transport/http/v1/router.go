package v1

import (
	"net/http"

	"github.com/YagorX/shop-catalog-service/internal/transport/http/v1/contracts"
	"github.com/YagorX/shop-catalog-service/internal/transport/http/v1/handlers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type RouterDeps struct {
	LogLevelController contracts.LogLevelController
	ReadinessChecker   contracts.ReadinessChecker
}

func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	healthHandler := handlers.NewHealthHandler(deps.ReadinessChecker)
	adminHandler := handlers.NewAdminHandler(deps.LogLevelController)

	mux.HandleFunc("/health", healthHandler.Health)
	mux.HandleFunc("/ready", healthHandler.Ready)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/admin/log-level", adminHandler.LogLevel)

	return mux
}

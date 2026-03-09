# shop-catalog-service

`shop-catalog-service` — gRPC-сервис каталога товаров в observability-проекте.

## Что реализовано

1. gRPC методы:
   - `ListProducts`
   - `GetProduct`
2. HTTP operational endpoints:
   - `GET /health`
   - `GET /ready`
   - `GET /metrics`
   - `GET/POST /admin/log-level`
3. Репозитории:
   - PostgreSQL (source of truth)
   - Redis cache
   - cached wrapper (cache-aside)
4. Observability:
   - JSON-логирование (`slog`)
   - Prometheus-метрики (`catalog_*`)
   - OpenTelemetry-трейсинг
5. gRPC health service зарегистрирован (`grpc.health.v1.Health/Check`)

## Архитектура

1. `transport/grpc` — валидация и маппинг grpc-кодов.
2. `service/catalog` — бизнес-логика и правила.
3. `repository/*` — доступ к Postgres/Redis.
4. `app/*` — сборка зависимостей и lifecycle.

## Конфигурация

Файлы:

1. `config/config.local.yaml`
2. `config/config.docker.yaml`

## Запуск

Локально:

```bash
go run ./cmd/catalog --config config/config.local.yaml
```

Через compose (из `shop-platform/deploy`):

```bash
docker compose up -d --build catalog-service
```

## Проверка

HTTP:

```bash
curl http://localhost:8081/health
curl http://localhost:8081/ready
curl http://localhost:8081/metrics
```

gRPC:

```bash
grpcurl -plaintext -d '{"limit":3,"offset":0}' localhost:9091 proto.catalog.v1.CatalogService/ListProducts
grpcurl -plaintext -d '{"id":"prod-001"}' localhost:9091 proto.catalog.v1.CatalogService/GetProduct
```

gRPC health:

```bash
grpcurl -plaintext -d '{"service":"proto.catalog.v1.CatalogService"}' localhost:9091 grpc.health.v1.Health/Check
```

## Метрики

1. `catalog_service_requests_total{method,status}`
2. `catalog_service_request_duration_seconds{method}`
3. `catalog_grpc_requests_total{method,code}`
4. `catalog_grpc_request_duration_seconds{method}`
5. `catalog_cache_requests_total{method,result}`
6. `catalog_cache_request_duration_seconds{method,operation}`

## Tracing

Ожидаемая цепочка:

1. gRPC transport span
2. `service.catalog.*`
3. repository spans (`cached`, `redis`, `postgres`)

## Текущее состояние

Сервис production-like для чтения каталога:

1. реализованы health/readiness/metrics
2. есть gRPC health check для интеграции с gateway
3. подключены Postgres, Redis, logs/metrics/traces

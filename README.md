# shop-catalog-service

`shop-catalog-service` — gRPC-сервис каталога товаров в учебном observability-проекте.

Это первый сервис, на котором собраны вместе:

1. нормальная слоистая архитектура,
2. PostgreSQL как source of truth,
3. Redis как cache,
4. structured logging,
5. Prometheus metrics,
6. OpenTelemetry traces,
7. Jaeger,
8. Docker runtime,
9. migrations и seed.

Этот сервис нужен не только как «каталог товаров», но и как эталонный пример того, как должен быть устроен реальный Go-сервис с observability и с понятными границами между transport, business logic и data access.

## Зачем нужен этот сервис

Сервис отвечает за чтение каталога товаров.

Сейчас он умеет:

1. вернуть список товаров,
2. вернуть один товар по `id`.

Сервис не занимается:

1. созданием товаров,
2. изменением товаров,
3. фильтрацией по категориям,
4. полнотекстовым поиском,
5. бизнес-флоу заказа.

Это сделано специально: сервис пока сфокусирован на чтении и observability, чтобы на одном примере хорошо отработать:

1. transport,
2. service layer,
3. repository layer,
4. caching,
5. метрики,
6. трейсы,
7. логи,
8. readiness/liveness,
9. docker deployment.

## Что сервис делает сейчас

### Бизнес-методы

Через gRPC сервис предоставляет:

1. `ListProducts`
2. `GetProduct`

### Operational endpoints

Через HTTP сервис предоставляет:

1. `GET /health`
2. `GET /ready`
3. `GET /metrics`
4. `GET /admin/log-level`
5. `POST /admin/log-level?level=debug|info|warn|error`

### Что уже встроено

1. gRPC transport для бизнес-методов
2. HTTP operational endpoints
3. JSON logging через `slog`
4. runtime-смена уровня логирования
5. Prometheus metrics
6. OpenTelemetry tracing
7. Jaeger integration
8. PostgreSQL repository
9. Redis cache
10. cache-aside wrapper
11. SQL migrations
12. seed данных
13. dockerized runtime через `shop-platform`
14. readiness checks для реальных зависимостей
15. graceful shutdown

## Как сервис устроен

Сервис разделён по слоям.

## 1. Transport layer

Папка: `internal/transport`

Здесь живёт код, который принимает внешние запросы и преобразует их в вызовы бизнес-логики.

### gRPC transport

Путь: `internal/transport/grpc/v1/handlers`

Что делает:

1. валидирует входной protobuf request,
2. вызывает `CatalogService`,
3. преобразует ошибки в gRPC status codes,
4. пишет transport-level logs,
5. обновляет transport-level gRPC metrics,
6. участвует в transport-level tracing.

Именно этот слой знает про:

1. `catalogv1.ListProductsRequest`,
2. `catalogv1.GetProductRequest`,
3. `codes.NotFound`, `codes.InvalidArgument`, `codes.Internal`.

### HTTP transport

Путь: `internal/transport/http/v1`

Этот слой не реализует бизнес-методы каталога.
Он нужен только для operational/runtime API.

Что здесь есть:

1. `/health` — liveness,
2. `/ready` — readiness,
3. `/metrics` — Prometheus endpoint,
4. `/admin/log-level` — смена уровня логирования.

## 2. Service layer

Путь: `internal/service/catalog`

Это business/usecase слой.

Он не знает про:

1. gRPC protobuf,
2. HTTP,
3. SQL,
4. Redis protocol.

Он знает только:

1. доменную модель `domain.Product`,
2. интерфейс `ProductRepository`,
3. бизнес-правила чтения каталога.

Что делает `CatalogService`:

1. валидирует `limit` и `offset`,
2. нормализует пагинацию,
3. валидирует `id` у `GetProduct`,
4. пишет service-level logs,
5. пишет service-level metrics,
6. создаёт service-level spans.

Именно здесь реализованы usecase-методы:

1. `ListProducts(ctx, limit, offset)`
2. `GetProduct(ctx, id)`

## 3. Repository layer

Папка: `internal/repository`

Этот слой отвечает за получение данных.

Сейчас в сервисе есть несколько реализаций.

### `postgres`

Путь: `internal/repository/postgres`

Это source of truth.

Именно этот repository читает реальные товары из PostgreSQL.

Что делает:

1. `GetByID` через SQL query,
2. `List` через SQL query,
3. пишет repo-level logs,
4. создаёт spans для DB access.

### `redis`

Путь: `internal/repository/redis`

Это низкоуровневый cache adapter.

Он не реализует `ProductRepository` напрямую.
Он умеет:

1. читать товар из Redis,
2. писать товар в Redis,
3. читать список товаров из Redis,
4. писать список товаров в Redis.

То есть это infrastructure layer для cache storage.

### `cached`

Путь: `internal/repository/cached`

Это cache-aside wrapper.

Именно он реализует `ProductRepository` и внутри комбинирует:

1. Redis cache,
2. fallback в PostgreSQL.

Это главный repository, который реально используется сервисом.

### `in_memory`

Путь: `internal/repository/in_memory`

Оставлен для:

1. быстрых локальных тестов,
2. dev режима,
3. unit tests,
4. сравнения поведения без внешних зависимостей.

В текущем production-like runtime он уже не основной.

## 4. Observability layer

Папка: `internal/observability`

Тут находятся общие компоненты observability:

1. logger,
2. metrics,
3. tracing.

### Logger

Используется `slog` в JSON-режиме.

Базовые поля в логах:

1. `service`
2. `env`
3. `version`

Дополнительные поля:

1. `op`
2. `duration_ms`
3. `error`
4. service/repository-specific attributes

### Metrics

В сервисе есть несколько уровней метрик.

#### Service-level

1. `catalog_service_requests_total{method,status}`
2. `catalog_service_request_duration_seconds{method}`

#### gRPC transport-level

1. `catalog_grpc_requests_total{method,code}`
2. `catalog_grpc_request_duration_seconds{method}`

#### Cache-level

1. `catalog_cache_requests_total{method,result}`
2. `catalog_cache_request_duration_seconds{method,operation}`

Плюс автоматически доступны Go runtime/process metrics через `/metrics`.

### Tracing

Используется OpenTelemetry.

Root transport span создаётся автоматически на gRPC server уровне.

Дальше строятся child spans по слоям:

1. transport span,
2. service span,
3. cached repository span,
4. redis span,
5. postgres span.

Это позволяет видеть не просто одну длинную «колбасу», а реальную структуру обработки запроса.

## 5. App/runtime layer

Папка: `internal/app`

Это composition root и lifecycle orchestration.

Сервис сейчас собран через:

1. `internal/app/app.go`
2. `internal/app/grpcapp`
3. `internal/app/httpapp`

Что делает `app` слой:

1. загружает logger,
2. инициализирует tracing,
3. открывает PostgreSQL,
4. открывает Redis,
5. собирает repository chain,
6. собирает `CatalogService`,
7. собирает gRPC handler,
8. регистрирует сервис в gRPC server,
9. поднимает HTTP server,
10. делает graceful shutdown.

## Data flow: как проходит запрос

## `GetProduct`

Полный путь сейчас такой:

1. gRPC client вызывает `GetProduct`
2. gRPC transport handler валидирует request
3. handler вызывает `CatalogService.GetProduct`
4. `CatalogService` вызывает `cached repository`
5. `cached repository` сначала идёт в Redis
6. если `hit`, товар возвращается сразу
7. если `miss`, `cached repository` идёт в PostgreSQL
8. если PostgreSQL нашёл товар, он записывается в Redis
9. результат возвращается обратно вверх по слоям

Таким образом, при первом запросе обычно путь длиннее, а при повторном короче.

## `ListProducts`

Путь похожий:

1. gRPC client вызывает `ListProducts`
2. transport handler валидирует request
3. handler вызывает `CatalogService.ListProducts`
4. `CatalogService` нормализует `limit/offset`
5. `cached repository` пытается взять список из Redis
6. если `miss`, идёт в PostgreSQL
7. успешный результат пишет в Redis
8. результат возвращается клиенту

## PostgreSQL

PostgreSQL — источник истины.

Таблица: `products`

Поля:

1. `id`
2. `sku`
3. `name`
4. `description`
5. `price_cents`
6. `currency`
7. `stock`
8. `active`
9. `created_at`
10. `updated_at`

`price_cents` хранится как integer, а не float.

Это сделано правильно для money values.

## Redis

Redis используется как cache layer.

Сейчас кэшируются:

1. `GetProduct(id)`
2. `ListProducts(limit, offset)`

Паттерн:

1. cache-aside
2. read-through вручную через cached repository
3. TTL-based freshness

### Примеры cache keys

Карточка товара:

```text
catalog:product:prod-001
```

Список:

```text
catalog:list:v1:limit=10:offset=0
```

## Health and Readiness

### `/health`

`/health` — это liveness endpoint.

Он отвечает только на вопрос:

«жив ли процесс настолько, чтобы ответить?»

### `/ready`

`/ready` — это readiness endpoint.

Он отвечает на вопрос:

«готов ли сервис реально принимать трафик?»

Сейчас readiness проверяет реальные зависимости:

1. PostgreSQL через `PingContext`
2. Redis через `Ping`

Поведение:

1. `200` — сервис готов
2. `503` — хотя бы одна зависимость недоступна

Это уже production-like readiness, а не декоративный endpoint.

## Структура репозитория

```text
cmd/
  catalog/
  catalog-client/
  migrator/

config/
  config.local.yaml
  config.docker.yaml

internal/
  app/
    app.go
    grpcapp/
    httpapp/
  config/
  domain/
  observability/
  repository/
    cached/
    in_memory/
    postgres/
    redis/
  service/
    catalog/
  transport/
    grpc/
    http/

migrations/
```

## Конфигурация

### `config/config.local.yaml`

Используется для локального запуска вне Docker.

Типичный сценарий:

1. сервис запускается через `go run`,
2. observability stack уже поднят в `shop-platform`,
3. PostgreSQL и Redis доступны по host ports.

### `config/config.docker.yaml`

Используется для запуска внутри Docker compose.

Типичный сценарий:

1. сервис работает как контейнер,
2. PostgreSQL доступен как `postgres:5432`,
3. Redis доступен как `redis:6379`,
4. Jaeger OTLP endpoint доступен как `jaeger:4317`.

## Миграции

Миграции лежат в `migrations/`.

Сейчас есть:

1. миграция создания таблицы `products`,
2. миграция seed товаров.

Команды:

```bash
make migrate-up
make migrate-down
make migrate-version
```

Важно для локальной среды:

1. если на Windows уже стоит локальный Postgres, не надо случайно подключаться к нему вместо docker Postgres,
2. лучше использовать отдельный host port для docker Postgres, например `5433`,
3. DSN в `Makefile` и `config.local.yaml` должен смотреть именно туда.

## Запуск

### Локально

```bash
make run
```

### gRPC client

```bash
make client
```

### Миграции

```bash
make migrate-up
```

### Через Docker

Сервис запускается внутри `shop-platform` compose вместе с:

1. PostgreSQL
2. Redis
3. Kafka
4. ELK stack
5. Prometheus
6. Grafana
7. Jaeger

Типичный запуск:

```bash
docker compose -f ../shop-platform/deploy/docker-compose.yml up -d --build catalog-service
```

## Проверка сервиса

### Проверка health и readiness

```bash
curl http://localhost:8081/health
curl http://localhost:8081/ready
```

### Проверка metrics

```bash
curl http://localhost:8081/metrics
```

### Проверка gRPC reflection

```bash
grpcurl -plaintext localhost:9091 list
```

### Проверка `ListProducts`

```bash
grpcurl -plaintext -d '{"limit":3,"offset":0}' localhost:9091 proto.catalog.v1.CatalogService/ListProducts
```

### Проверка `GetProduct`

```bash
grpcurl -plaintext -d '{"id":"prod-001"}' localhost:9091 proto.catalog.v1.CatalogService/GetProduct
```

## Как проверять Redis cache

### `GetProduct`

Первый вызов:

1. ожидается `cache miss`,
2. ожидается PostgreSQL lookup,
3. ожидается Redis write.

Второй вызов тем же `id`:

1. ожидается `cache hit`,
2. PostgreSQL уже не должен участвовать.

### `ListProducts`

Первый вызов с конкретным `limit/offset`:

1. ожидается `cache miss`,
2. ожидается PostgreSQL lookup,
3. ожидается Redis write.

Второй вызов с теми же `limit/offset`:

1. ожидается `cache hit`.

## Как проверять observability

### Kibana

Полезно фильтровать по:

1. `service = catalog-service`
2. `op`
3. `level`

### Prometheus

Полезные запросы:

```promql
catalog_service_requests_total
catalog_grpc_requests_total
catalog_cache_requests_total
```

### Jaeger

Ожидаемая трасса для `GetProduct`:

1. gRPC transport span
2. `service.catalog.GetProduct`
3. `repository.cached.ProductRepository.GetByID`
4. `repository.redis.Cache.GetProduct`
5. при `miss`:
   - `repository.postgres.ProductRepository.GetByID`
   - `repository.redis.Cache.SetProduct`

Ожидаемая трасса для `ListProducts`:

1. gRPC transport span
2. `service.catalog.ListProducts`
3. `repository.cached.ProductRepository.List`
4. `repository.redis.Cache.GetProductList`
5. при `miss`:
   - `repository.postgres.ProductRepository.List`
   - `repository.redis.Cache.SetProductList`

## Чем этот сервис полезен для проекта целиком

`shop-catalog-service` сейчас — это опорный сервис проекта.

На нём уже можно увидеть:

1. как строить service composition,
2. как разделять transport/service/repository,
3. как внедрять observability на каждом слое,
4. как строить cache-aside поверх Postgres,
5. как делать readiness не фиктивным,
6. как организовать runtime lifecycle и graceful shutdown.

То есть это уже не «черновик сервиса», а полноценный базовый шаблон для остальных сервисов проекта.

## Текущее состояние

Сервис уже покрывает полный минимальный observability cycle:

1. logs,
2. metrics,
3. traces,
4. PostgreSQL source of truth,
5. Redis cache,
6. cache-aside pattern,
7. HTTP + gRPC runtime decomposition,
8. migrations and seed,
9. readiness checks для реальных зависимостей.

## Что логично делать дальше

1. добавить gRPC health service,
2. написать tests для `service`, `cached repo`, `postgres repo`,
3. продумать cache invalidation strategy для будущих write-методов,
4. интегрировать `shop-gateway`,
5. построить end-to-end trace между сервисами,
6. собрать Grafana dashboard для service/gRPC/cache метрик.

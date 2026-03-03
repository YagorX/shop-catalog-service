APP_NAME=catalog-service
CONFIG_PATH=./config/config.local.yaml

POSTGRES_DSN=postgres://catalog:catalog@localhost:5433/catalog?sslmode=disable
MIGRATIONS_PATH=./migrations
MIGRATIONS_TABLE=schema_migrations

.PHONY: run client migrate-up migrate-down migrate-version

run:
	go run ./cmd/catalog/main.go -config $(CONFIG_PATH)

client:
	go run ./cmd/catalog-client/main.go -addr 127.0.0.1:9091 -id prod-001

migrate-up:
	go run ./cmd/migrator/main.go \
		-dsn "$(POSTGRES_DSN)" \
		-migrations-path "$(MIGRATIONS_PATH)" \
		-migrations-table "$(MIGRATIONS_TABLE)" \
		-command up

migrate-down:
	go run ./cmd/migrator/main.go \
		-dsn "$(POSTGRES_DSN)" \
		-migrations-path "$(MIGRATIONS_PATH)" \
		-migrations-table "$(MIGRATIONS_TABLE)" \
		-command down

migrate-version:
	go run ./cmd/migrator/main.go \
		-dsn "$(POSTGRES_DSN)" \
		-migrations-path "$(MIGRATIONS_PATH)" \
		-migrations-table "$(MIGRATIONS_TABLE)" \
		-command version

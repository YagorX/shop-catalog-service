FROM golang:1.24.1 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/catalog-service ./cmd/catalog

FROM alpine:3.20
WORKDIR /app

RUN addgroup -S app && adduser -S app -G app

COPY --from=builder /out/catalog-service /app/catalog-service
COPY config/config.docker.yaml /app/config/config.docker.yaml

USER app

EXPOSE 8081 9091

ENV CONFIG_PATH=/app/config/config.docker.yaml

ENTRYPOINT ["/app/catalog-service"]

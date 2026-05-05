package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// Типы событий
const (
	EventProductCreated = "product.created"
	EventProductUpdated = "product.updated"
	EventStockChanged   = "stock.changed"
)

// ProductEvent — единая структура для всех событий каталога
type ProductEvent struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	ProductID  string    `json:"product_id"`
	SKU        string    `json:"sku"`
	Name       string    `json:"name"`
	PriceCents int64     `json:"price_cents"`
	Currency   string    `json:"currency"`
	Stock      int32     `json:"stock"`
	Active     bool      `json:"active"`
	OccuredAt  time.Time `json:"occured_at"`
}

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.Hash{}, // по ключу → один продукт всегда в одной партиции
			RequiredAcks: kafka.RequireAll,
			MaxAttempts:  5,
			BatchSize:    100,
			BatchTimeout: 10 * time.Millisecond,
		},
	}
}

func (p *Producer) Publish(ctx context.Context, event ProductEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: "catalog.products.v1",
		Key:   []byte(event.ProductID), // гарантирует порядок событий одного товара
		Value: payload,
		Headers: []kafka.Header{
			{Key: "event-type", Value: []byte(event.EventType)},
			{Key: "event-id", Value: []byte(event.EventID)},
			{Key: "source", Value: []byte("catalog-service")},
		},
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func (p *Producer) PublishRaw(ctx context.Context, topic, key, eventType, eventID string, payload []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
		Headers: []kafka.Header{
			{Key: "event-type", Value: []byte(eventType)},
			{Key: "event-id", Value: []byte(eventID)},
			{Key: "source", Value: []byte("catalog-service")},
		},
	})
}

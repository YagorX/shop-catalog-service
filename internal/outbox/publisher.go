package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/YagorX/shop-catalog-service/internal/events"
)

type Event struct {
	ID         string
	Topic      string
	MessageKey string
	EventType  string
	Payload    []byte
}

type Publisher struct {
	db       *sql.DB
	producer *events.Producer
	logger   *slog.Logger

	batchSize int
	interval  time.Duration
}

func NewPublisher(db *sql.DB, producer *events.Producer, logger *slog.Logger) *Publisher {
	if logger == nil {
		logger = slog.Default()
	}

	return &Publisher{
		db:        db,
		producer:  producer,
		logger:    logger,
		batchSize: 100,
		interval:  time.Second,
	}
}

func (p *Publisher) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("outbox publisher stopped")
			return nil

		case <-ticker.C:
			if err := p.publishBatch(ctx); err != nil {
				p.logger.Error("publish outbox batch failed",
					slog.String("error", err.Error()),
				)
			}
		}
	}
}

func (p *Publisher) Close() error {
	if p == nil || p.producer == nil {
		return nil
	}
	if err := p.producer.Close(); err != nil {
		p.logger.Error("close kafka producer failed",
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}

func (p *Publisher) publishBatch(ctx context.Context) error {
	events, err := p.fetchPending(ctx)
	if err != nil {
		return err
	}

	for _, event := range events {
		err := p.producer.PublishRaw(
			ctx,
			event.Topic,
			event.MessageKey,
			event.EventType,
			event.ID,
			event.Payload,
		)

		if err != nil {
			p.logger.Error("publish outbox event failed",
				slog.String("event_id", event.ID),
				slog.String("error", err.Error()),
			)
			p.markFailed(ctx, event.ID, err)
			continue
		}

		p.markSent(ctx, event.ID)
	}

	return nil
}

func (p *Publisher) markFailed(ctx context.Context, eventID string, err error) {
	_, updateErr := p.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET attempts = attempts + 1,
		    last_error = $1
		WHERE id = $2
	`, err.Error(), eventID)
	if updateErr != nil {
		p.logger.Error("update outbox event failed",
			slog.String("event_id", eventID),
			slog.String("error", updateErr.Error()),
		)
	}
}

func (p *Publisher) markSent(ctx context.Context, eventID string) {
	_, updateErr := p.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET status = 'sent', sent_at = NOW()
		WHERE id = $1
	`, eventID)
	if updateErr != nil {
		p.logger.Error("update outbox event failed",
			slog.String("event_id", eventID),
			slog.String("error", updateErr.Error()),
		)
	}
}

func (p *Publisher) fetchPending(ctx context.Context) ([]Event, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, topic, message_key, event_type, payload
		FROM outbox_events
		WHERE status = 'pending'
		ORDER BY created_at
		LIMIT $1
	`, p.batchSize)
	if err != nil {
		return nil, fmt.Errorf("query pending outbox events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0, p.batchSize)

	for rows.Next() {
		var event Event

		if err := rows.Scan(
			&event.ID,
			&event.Topic,
			&event.MessageKey,
			&event.EventType,
			&event.Payload,
		); err != nil {
			return nil, fmt.Errorf("scan pending outbox event: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending outbox events: %w", err)
	}

	return events, nil
}

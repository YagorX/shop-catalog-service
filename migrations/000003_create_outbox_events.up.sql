CREATE TABLE outbox_events (
    id TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    message_key TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);

CREATE INDEX idx_outbox_events_pending
ON outbox_events (created_at)
WHERE status = 'pending';

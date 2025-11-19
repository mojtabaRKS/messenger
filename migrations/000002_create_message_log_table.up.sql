CREATE TABLE messages
(
    id                  BIGSERIAL PRIMARY KEY,
    user_id         TEXT        NOT NULL,
    to_number           TEXT        NOT NULL,
    body                TEXT        NOT NULL,
    status              TEXT        NOT NULL DEFAULT 'queued',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    provider_message_id TEXT NULL
);
CREATE INDEX ON messages (user_id, created_at);

CREATE TABLE messages
(
    id         BIGSERIAL PRIMARY KEY,
    user_id    TEXT        NOT NULL,
    to_number  TEXT        NOT NULL,
    body       TEXT        NOT NULL,
    status     ENUM('queued', 'success', 'failure', 'processing')        NOT NULL DEFAULT 'queued',
    hash       TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
);
CREATE INDEX ON messages (user_id, created_at);

CREATE TABLE kafka_dlq
(
    id              BIGSERIAL PRIMARY KEY,
    topic           TEXT        NOT NULL,
    key             TEXT,
    payload         JSONB       NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempt_count   INT         NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ NULL
);

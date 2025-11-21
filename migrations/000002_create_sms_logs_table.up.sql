CREATE TABLE sms_logs
(
    message_id           BIGSERIAL,
    customer_id          BIGINT      NOT NULL,
    to_number            TEXT        NOT NULL,
    body                 TEXT        NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE sms_status_events
(
    id          BIGSERIAL PRIMARY KEY,
    message_id  UUID        NOT NULL,
    status      TEXT        NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    operator_id TEXT NULL
);

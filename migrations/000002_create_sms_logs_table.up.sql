CREATE TABLE sms_logs
(
    message_id           UUID        NOT NULL,
    customer_id          BIGINT      NOT NULL,
    to_number            TEXT        NOT NULL,
    body                 TEXT        NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, created_at)
) PARTITION BY RANGE (created_at);

-- create example partitions for the next 7 days
CREATE TABLE sms_logs_2025_11_21 PARTITION OF sms_logs FOR VALUES FROM ('2025-11-21') TO ('2025-11-22');
CREATE TABLE sms_logs_2025_11_22 PARTITION OF sms_logs FOR VALUES FROM ('2025-11-22') TO ('2025-11-23');
CREATE TABLE sms_logs_2025_11_23 PARTITION OF sms_logs FOR VALUES FROM ('2025-11-23') TO ('2025-11-24');
CREATE TABLE sms_logs_2025_11_24 PARTITION OF sms_logs FOR VALUES FROM ('2025-11-24') TO ('2025-11-25');
CREATE TABLE sms_logs_2025_11_25 PARTITION OF sms_logs FOR VALUES FROM ('2025-11-25') TO ('2025-11-26');
CREATE TABLE sms_logs_2025_11_26 PARTITION OF sms_logs FOR VALUES FROM ('2025-11-26') TO ('2025-11-27');
CREATE TABLE sms_logs_2025_11_27 PARTITION OF sms_logs FOR VALUES FROM ('2025-11-27') TO ('2025-11-28');
CREATE TABLE sms_logs_2025_11_28 PARTITION OF sms_logs FOR VALUES FROM ('2025-11-28') TO ('2025-11-29');

CREATE TABLE sms_status_events
(
    id          BIGSERIAL PRIMARY KEY,
    message_id  UUID        NOT NULL,
    status      TEXT        NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    operator_id TEXT NULL
);

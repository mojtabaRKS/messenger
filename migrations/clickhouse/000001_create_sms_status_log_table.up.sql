CREATE TABLE IF NOT EXISTS sms_status_log
(
    job_id String,
    customer_id String,
    phone String,
    message String,
    status String,
    priority Int32,
    created_at DateTime,
    timestamp DateTime
) ENGINE = MergeTree()
    ORDER BY (timestamp, customer_id)
    PARTITION BY toYYYYMM (timestamp)
    TTL timestamp + INTERVAL 90 DAY
# Database Schema - SMS Gateway

## PostgreSQL Schema (OLTP)

### Design Principles
1. **Write-Optimized**: Minimal indexes, append-only logs
2. **Partitioned**: Daily partitions for SMS logs
3. **Atomic Balance**: Row-level locking or advisory locks
4. **Fast Reads**: Indexes only on critical query paths

---

## Table Definitions

### 1. Customers Table

```sql
CREATE TABLE customers (
    customer_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT,
    balance BIGINT NOT NULL DEFAULT 0,  -- Balance in cents
    status TEXT NOT NULL DEFAULT 'active',  -- active, suspended, deleted
    rate_limit INT NOT NULL DEFAULT 1000,  -- Max SMS per minute
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT positive_balance CHECK (balance >= 0)
);

-- Index for balance queries
CREATE INDEX idx_customers_balance ON customers(balance) WHERE balance > 0;

-- Index for status filtering
CREATE INDEX idx_customers_status ON customers(status) WHERE status = 'active';

-- Trigger to update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_customers_updated_at
    BEFORE UPDATE ON customers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

**Estimated Rows**: 10,000 - 100,000 customers  
**Size**: ~10-100 MB  
**Write Pattern**: Low frequency (balance updates)  
**Read Pattern**: High frequency (balance checks)

---

### 2. SMS Logs (Partitioned)

```sql
-- Parent table (partitioned by created_at)
CREATE TABLE sms_logs (
    id BIGSERIAL,
    customer_id TEXT NOT NULL,
    phone TEXT NOT NULL,
    message TEXT NOT NULL,
    cost BIGINT NOT NULL,  -- Cost in cents
    status TEXT NOT NULL DEFAULT 'accepted',  -- accepted, queued, sent, failed, delivered
    idempotency_key TEXT,  -- Optional: for duplicate prevention
    metadata JSONB,  -- Optional: for extensibility
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Index on customer_id for queries (created on each partition)
CREATE INDEX idx_sms_logs_customer ON sms_logs(customer_id, created_at DESC);

-- Unique constraint on idempotency_key (per customer)
CREATE UNIQUE INDEX idx_sms_logs_idempotency 
    ON sms_logs(customer_id, idempotency_key) 
    WHERE idempotency_key IS NOT NULL;

-- Optional: Index on phone for debugging
-- CREATE INDEX idx_sms_logs_phone ON sms_logs(phone) WHERE status = 'failed';
```

**Daily Partitions** (auto-created):
```sql
-- Example: Partition for 2025-11-20
CREATE TABLE sms_logs_2025_11_20 PARTITION OF sms_logs
    FOR VALUES FROM ('2025-11-20 00:00:00+00') TO ('2025-11-21 00:00:00+00');

-- Partition for 2025-11-21
CREATE TABLE sms_logs_2025_11_21 PARTITION OF sms_logs
    FOR VALUES FROM ('2025-11-21 00:00:00+00') TO ('2025-11-22 00:00:00+00');
```

**Partition Management Function**:
```sql
CREATE OR REPLACE FUNCTION create_sms_logs_partition(partition_date DATE)
RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_date TIMESTAMPTZ;
    end_date TIMESTAMPTZ;
BEGIN
    partition_name := 'sms_logs_' || to_char(partition_date, 'YYYY_MM_DD');
    start_date := partition_date::TIMESTAMPTZ;
    end_date := (partition_date + INTERVAL '1 day')::TIMESTAMPTZ;
    
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF sms_logs
         FOR VALUES FROM (%L) TO (%L)',
        partition_name, start_date, end_date
    );
    
    RAISE NOTICE 'Created partition: %', partition_name;
END;
$$ LANGUAGE plpgsql;

-- Pre-create partitions for the next 7 days
DO $$
DECLARE
    i INT;
BEGIN
    FOR i IN 0..6 LOOP
        PERFORM create_sms_logs_partition(CURRENT_DATE + i);
    END LOOP;
END $$;
```

**Automated Partition Creation** (via cron job or pg_cron):
```sql
-- Install pg_cron extension
CREATE EXTENSION IF NOT EXISTS pg_cron;

-- Schedule daily partition creation at 2 AM
SELECT cron.schedule(
    'create_future_partitions',
    '0 2 * * *',  -- Daily at 2 AM
    $$SELECT create_sms_logs_partition(CURRENT_DATE + 7)$$
);
```

**Partition Archival Strategy**:
```sql
-- Detach old partitions (run monthly)
CREATE OR REPLACE FUNCTION archive_old_sms_partitions(days_to_keep INT DEFAULT 30)
RETURNS VOID AS $$
DECLARE
    partition_record RECORD;
    cutoff_date DATE;
BEGIN
    cutoff_date := CURRENT_DATE - days_to_keep;
    
    FOR partition_record IN
        SELECT tablename FROM pg_tables
        WHERE schemaname = 'public'
          AND tablename LIKE 'sms_logs_20%'
          AND to_date(substring(tablename from 10), 'YYYY_MM_DD') < cutoff_date
    LOOP
        EXECUTE format('ALTER TABLE sms_logs DETACH PARTITION %I', partition_record.tablename);
        RAISE NOTICE 'Detached partition: %', partition_record.tablename;
        
        -- Optional: Move to archive schema
        -- EXECUTE format('ALTER TABLE %I SET SCHEMA archive', partition_record.tablename);
    END LOOP;
END;
$$ LANGUAGE plpgsql;
```

**Estimated Data Volume**:
- 100M SMS/day
- Avg 200 bytes per row (including indexes)
- Daily partition: ~20 GB/day
- 30-day retention: ~600 GB
- With compression: ~300 GB

---

### 3. Balance Transactions (Audit Trail)

```sql
CREATE TABLE balance_transactions (
    id BIGSERIAL PRIMARY KEY,
    customer_id TEXT NOT NULL,
    amount BIGINT NOT NULL,  -- Positive for credit, negative for debit
    balance_before BIGINT NOT NULL,
    balance_after BIGINT NOT NULL,
    transaction_type TEXT NOT NULL,  -- debit_sms, credit_topup, refund
    reference_id TEXT,  -- SMS ID or payment ID
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (created_at);

CREATE INDEX idx_balance_txn_customer ON balance_transactions(customer_id, created_at DESC);

-- Daily partitions (similar to sms_logs)
CREATE TABLE balance_transactions_2025_11_20 PARTITION OF balance_transactions
    FOR VALUES FROM ('2025-11-20 00:00:00+00') TO ('2025-11-21 00:00:00+00');
```

**Purpose**: 
- Audit trail for all balance changes
- Debugging balance discrepancies
- Regulatory compliance

---

### 4. Idempotency Cache (Optional)

```sql
CREATE TABLE idempotency_cache (
    idempotency_key TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL,
    response_status INT NOT NULL,
    response_body JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- TTL: Delete entries older than 24 hours
CREATE INDEX idx_idempotency_created ON idempotency_cache(created_at);

-- Cleanup function (run hourly)
CREATE OR REPLACE FUNCTION cleanup_idempotency_cache()
RETURNS VOID AS $$
BEGIN
    DELETE FROM idempotency_cache
    WHERE created_at < NOW() - INTERVAL '24 hours';
END;
$$ LANGUAGE plpgsql;
```

---

## Atomic Balance Deduction Patterns

### Pattern 1: Row-Level Locking (Simple, High Contention)

```sql
BEGIN;

-- Lock the customer row
SELECT customer_id, balance 
FROM customers 
WHERE customer_id = $1 
FOR UPDATE;

-- Check balance
IF (balance < $2) THEN
    ROLLBACK;
    RETURN 'insufficient_balance';
END IF;

-- Deduct balance
UPDATE customers 
SET balance = balance - $2 
WHERE customer_id = $1;

-- Insert SMS log
INSERT INTO sms_logs (customer_id, phone, message, cost)
VALUES ($1, $3, $4, $2);

COMMIT;
```

**Pros**: Simple, ACID guarantees  
**Cons**: High contention for popular customers  
**Use Case**: Low-to-medium throughput customers (<100 TPS per customer)

---

### Pattern 2: Advisory Locks (Better for High Contention)

```sql
BEGIN;

-- Advisory lock on customer_id hash (numeric)
-- This is more efficient than row locks
SELECT pg_advisory_xact_lock(('x' || md5($1))::bit(64)::bigint);

SELECT balance FROM customers WHERE customer_id = $1;

IF (balance < $2) THEN
    ROLLBACK;
    RETURN 'insufficient_balance';
END IF;

UPDATE customers SET balance = balance - $2 WHERE customer_id = $1;

INSERT INTO sms_logs (customer_id, phone, message, cost)
VALUES ($1, $3, $4, $2);

COMMIT;
```

**Pros**: Less lock overhead, better performance  
**Cons**: Slightly more complex  
**Use Case**: High-throughput customers (100-1000 TPS per customer)

---

### Pattern 3: Optimistic Locking (Lowest Latency)

```sql
-- No explicit transaction needed
UPDATE customers 
SET balance = balance - $2 
WHERE customer_id = $1 
  AND balance >= $2
RETURNING balance;

-- If RETURNING yields a row, balance was sufficient
-- If no rows returned, balance was insufficient

IF (rows_affected = 0) THEN
    RETURN 'insufficient_balance';
END IF;

-- Insert SMS log (separate transaction is OK)
INSERT INTO sms_logs (customer_id, phone, message, cost)
VALUES ($1, $3, $4, $2);
```

**Pros**: Fastest, no locks  
**Cons**: Retry on failure, no transaction across tables  
**Use Case**: Ultra-high throughput, acceptable to have SMS log slightly decoupled

**Important**: This pattern is acceptable ONLY if you're OK with rare edge cases where the balance is deducted but SMS log insert fails (then rely on balance_transactions for reconciliation).

---

## PostgreSQL Configuration for High Throughput

```ini
# postgresql.conf optimizations

# Memory
shared_buffers = 16GB
effective_cache_size = 48GB
work_mem = 32MB
maintenance_work_mem = 2GB

# Checkpoints (reduce I/O spikes)
checkpoint_timeout = 15min
checkpoint_completion_target = 0.9
max_wal_size = 10GB
min_wal_size = 2GB

# Connections
max_connections = 500
superuser_reserved_connections = 5

# Write performance
synchronous_commit = off  # For sms_logs only (acceptable data loss: <1 sec)
wal_compression = on
wal_buffers = 16MB

# Parallel query (for reporting queries)
max_parallel_workers_per_gather = 4
max_parallel_workers = 8

# Autovacuum (critical for partitioned tables)
autovacuum_max_workers = 6
autovacuum_naptime = 10s
autovacuum_vacuum_scale_factor = 0.05
autovacuum_analyze_scale_factor = 0.02

# Query planner
random_page_cost = 1.1  # For SSD
effective_io_concurrency = 200
```

---

## ClickHouse Schema (OLAP)

### Design Principles
1. **Column-Oriented**: Fast aggregations
2. **Partitioned by Date**: Easy TTL management
3. **Pre-Aggregated**: Minute/hour level summaries
4. **Kafka Ingestion**: Real-time streaming from Kafka

---

### 1. Kafka Engine (Ingest from Kafka)

```sql
CREATE TABLE sms_raw_queue (
    event_id String,
    event_type String,
    timestamp DateTime64(3),
    customer_id String,
    sms_id UInt64,
    phone String,
    message String,
    cost UInt64,
    status String,
    idempotency_key String
) ENGINE = Kafka()
SETTINGS
    kafka_broker_list = 'kafka-broker-1:9092,kafka-broker-2:9092,kafka-broker-3:9092',
    kafka_topic_list = 'sms.accepted',
    kafka_group_name = 'clickhouse_ingestion',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 10,
    kafka_skip_broken_messages = 100;  -- Skip up to 100 malformed messages
```

---

### 2. Raw SMS Logs (Target Table)

```sql
CREATE TABLE sms_logs_ch (
    event_id String,
    timestamp DateTime64(3),
    customer_id String,
    sms_id UInt64,
    phone String,
    message String,
    cost UInt64,
    status String,
    date Date MATERIALIZED toDate(timestamp),
    hour UInt8 MATERIALIZED toHour(timestamp),
    minute UInt16 MATERIALIZED toStartOfMinute(timestamp)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(date)
ORDER BY (customer_id, timestamp)
TTL date + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- Materialized view to populate from Kafka
CREATE MATERIALIZED VIEW sms_logs_mv TO sms_logs_ch AS
SELECT 
    event_id,
    timestamp,
    customer_id,
    sms_id,
    phone,
    message,
    cost,
    status
FROM sms_raw_queue;
```

**Estimated Size**: 
- 100M rows/day × 150 bytes = 15 GB/day raw
- With compression (3-5x): ~3-5 GB/day
- 90-day retention: ~270-450 GB

---

### 3. Aggregated Tables

#### Minute-Level Aggregations (Real-Time)

```sql
CREATE TABLE sms_stats_minute (
    customer_id String,
    minute DateTime,
    sms_count UInt64,
    total_cost UInt64,
    failed_count UInt64,
    date Date MATERIALIZED toDate(minute)
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (customer_id, minute)
TTL date + INTERVAL 90 DAY;

-- Materialized view (real-time aggregation)
CREATE MATERIALIZED VIEW sms_stats_minute_mv TO sms_stats_minute AS
SELECT
    customer_id,
    toStartOfMinute(timestamp) as minute,
    count() as sms_count,
    sum(cost) as total_cost,
    countIf(status = 'failed') as failed_count
FROM sms_logs_ch
GROUP BY customer_id, minute;
```

**Query Example**:
```sql
-- Get customer SMS count for last hour
SELECT 
    customer_id,
    sum(sms_count) as total_sms,
    sum(total_cost) as total_cost,
    sum(failed_count) as failures
FROM sms_stats_minute
WHERE minute >= now() - INTERVAL 1 HOUR
  AND customer_id = 'cust_12345'
GROUP BY customer_id;
```

---

#### Hourly Aggregations (Batch)

```sql
CREATE TABLE sms_stats_hourly (
    customer_id String,
    hour DateTime,
    sms_count UInt64,
    total_cost UInt64,
    unique_phones UInt64,
    failed_count UInt64,
    date Date MATERIALIZED toDate(hour)
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (customer_id, hour)
TTL date + INTERVAL 180 DAY;

-- No materialized view - populated by batch INSERT
```

**Batch Aggregation Job** (run every hour):
```sql
INSERT INTO sms_stats_hourly (customer_id, hour, sms_count, total_cost, unique_phones, failed_count)
SELECT
    customer_id,
    toStartOfHour(timestamp) as hour,
    count() as sms_count,
    sum(cost) as total_cost,
    uniqExact(phone) as unique_phones,
    countIf(status = 'failed') as failed_count
FROM sms_logs_ch
WHERE timestamp >= now() - INTERVAL 2 HOUR  -- Process last 2 hours (overlap for safety)
  AND timestamp < now() - INTERVAL 1 HOUR   -- Don't process current hour (incomplete)
GROUP BY customer_id, hour;
```

---

#### Daily Aggregations (Batch)

```sql
CREATE TABLE sms_stats_daily (
    customer_id String,
    date Date,
    sms_count UInt64,
    total_cost UInt64,
    unique_phones UInt64,
    failed_count UInt64,
    avg_message_length Float32
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (customer_id, date)
TTL date + INTERVAL 365 DAY;
```

**Batch Job** (run daily at 2 AM):
```sql
INSERT INTO sms_stats_daily (customer_id, date, sms_count, total_cost, unique_phones, failed_count, avg_message_length)
SELECT
    customer_id,
    toDate(timestamp) as date,
    count() as sms_count,
    sum(cost) as total_cost,
    uniqExact(phone) as unique_phones,
    countIf(status = 'failed') as failed_count,
    avg(length(message)) as avg_message_length
FROM sms_logs_ch
WHERE date = yesterday()
GROUP BY customer_id, date;
```

---

### 4. Top-N Tables (Leaderboards)

```sql
CREATE TABLE top_customers_daily (
    date Date,
    rank UInt32,
    customer_id String,
    sms_count UInt64,
    total_cost UInt64
) ENGINE = MergeTree()
ORDER BY (date, rank);

-- Populated daily
INSERT INTO top_customers_daily
SELECT
    yesterday() as date,
    row_number() OVER (ORDER BY sms_count DESC) as rank,
    customer_id,
    sms_count,
    total_cost
FROM sms_stats_daily
WHERE date = yesterday()
ORDER BY sms_count DESC
LIMIT 1000;
```

---

## Query Performance Examples

### Query 1: Customer Activity (Last 24 Hours)
```sql
SELECT 
    customer_id,
    sum(sms_count) as total_sms,
    sum(total_cost) as total_cost
FROM sms_stats_minute
WHERE minute >= now() - INTERVAL 24 HOUR
  AND customer_id = 'cust_12345'
GROUP BY customer_id;
```
**Expected Latency**: <10ms  
**Rows Scanned**: 1,440 (24 hours × 60 minutes)

---

### Query 2: System-Wide Throughput (Last Hour)
```sql
SELECT 
    toStartOfFiveMinutes(minute) as bucket,
    sum(sms_count) as sms_per_5min
FROM sms_stats_minute
WHERE minute >= now() - INTERVAL 1 HOUR
GROUP BY bucket
ORDER BY bucket;
```
**Expected Latency**: <50ms  
**Rows Scanned**: ~60 minutes × 10,000 customers = 600K rows

---

### Query 3: Daily Report for Customer
```sql
SELECT 
    date,
    sms_count,
    total_cost,
    unique_phones,
    failed_count,
    (failed_count * 100.0 / sms_count) as failure_rate
FROM sms_stats_daily
WHERE customer_id = 'cust_12345'
  AND date >= today() - INTERVAL 30 DAY
ORDER BY date DESC;
```
**Expected Latency**: <5ms  
**Rows Scanned**: 30 rows

---

## Backup & Recovery

### PostgreSQL Backups
```bash
# Daily full backup (using pg_basebackup)
pg_basebackup -h localhost -U postgres -D /backup/$(date +%Y%m%d) -Ft -z -P

# Continuous WAL archiving (postgresql.conf)
archive_mode = on
archive_command = 'cp %p /wal_archive/%f'
```

### ClickHouse Backups
```sql
-- Backup to S3
BACKUP TABLE sms_logs_ch TO S3('s3://clickhouse-backups/sms_logs_ch/', 'access_key', 'secret_key');

-- Restore from S3
RESTORE TABLE sms_logs_ch FROM S3('s3://clickhouse-backups/sms_logs_ch/', 'access_key', 'secret_key');
```

---

## Summary

This schema achieves:
✅ Write throughput: 5,000-10,000 SMS/sec (PostgreSQL)  
✅ Atomic balance deduction with multiple patterns  
✅ Daily partitioning with auto-management  
✅ ClickHouse ingestion from Kafka (<5 sec latency)  
✅ Pre-aggregated tables for fast reporting (<50ms)  
✅ 90-day retention for raw logs, 1-year for aggregates  
✅ Minimal indexes on write path, optimized for reads  


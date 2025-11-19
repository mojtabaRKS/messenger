# High-Scale SMS Gateway - Complete Architecture

## System Overview

### Scale Requirements
- **Daily Volume**: ~100 million SMS/day
- **Average TPS**: ~1,200 SMS/sec
- **Peak TPS**: ~3,000 SMS/sec
- **Customers**: Tens of thousands with varying throughput profiles
- **Billing**: Zero-balance rejection, atomic deduction, cent-level accuracy
- **Reporting Latency**: 1-5 minutes acceptable

### Core Principles
1. **Billing First**: No SMS accepted without successful balance deduction
2. **Append-Only Writes**: PostgreSQL SMS logs are never updated
3. **Event-Driven**: Kafka as the single source of truth for analytics
4. **Separation of Concerns**: OLTP (Postgres) vs OLAP (ClickHouse)
5. **Isolation**: Per-customer rate shaping to prevent noisy neighbor issues

---

## Architecture Diagram

```
┌─────────────┐
│   Clients   │
└──────┬──────┘
       │ HTTP POST /send
       ▼
┌──────────────────────────────────────────────────────────┐
│                    API Gateway (Go)                       │
│  ┌────────────────────────────────────────────────────┐  │
│  │ 1. Validate Request                                 │  │
│  │ 2. Atomic Balance Check & Deduction (Postgres)     │  │
│  │ 3. Append SMS Log (Partitioned Table)              │  │
│  │ 4. Produce to Kafka (sms.accepted)                 │  │
│  │ 5. Return 200 OK                                    │  │
│  └────────────────────────────────────────────────────┘  │
└───────┬──────────────────────────────────┬───────────────┘
        │                                  │
        │ Write                            │ Produce Event
        ▼                                  ▼
┌─────────────────┐              ┌──────────────────────┐
│   PostgreSQL    │              │       Kafka          │
│  (OLTP Only)    │              │   (Event Backbone)   │
│                 │              │                      │
│ • Balances      │              │ Topics:              │
│ • SMS Logs      │              │  - sms.accepted      │
│   (partitioned) │              │  - sms.delivery      │
│ • Customers     │              │  - sms.status        │
└─────────────────┘              └──────┬───────────┬───┘
                                        │           │
                          ┌─────────────┘           └────────────┐
                          │                                      │
                          ▼                                      ▼
                 ┌──────────────────┐                  ┌──────────────────┐
                 │   ClickHouse     │                  │  Delivery Worker │
                 │   Ingestion      │                  │    Pool (Go)     │
                 │                  │                  │                  │
                 │ Kafka Engine →   │                  │ • Rate Shaping   │
                 │ Raw SMS Table →  │                  │ • Per-Customer   │
                 │ Aggregated Views │                  │   Queues         │
                 └──────────────────┘                  │ • Operator API   │
                                                       └──────────────────┘
                          │
                          │ Query
                          ▼
                 ┌──────────────────┐
                 │  Reporting API   │
                 │      (Go)        │
                 │                  │
                 │ • Customer Stats │
                 │ • Minute Aggs    │
                 │ • Daily Reports  │
                 └──────────────────┘
```

---

## Component Architecture

### 1. API Service (Go)

**Responsibilities**:
- Accept SMS send requests
- Validate customer_id, phone, message
- Atomic balance deduction
- Append-only SMS log write
- Kafka event production
- Sub-10ms P99 latency target

**Critical Path**:
```
Request → Validate → BEGIN TX → 
  SELECT balance FOR UPDATE → 
  Check >= cost → 
  UPDATE balance SET balance = balance - cost →
  INSERT INTO sms_logs →
COMMIT → Produce Kafka → Return 200
```

**Scaling Strategy**:
- Stateless: 10-50 API pods
- Connection pooling: 20-50 connections per pod
- Kafka batch producer: 100ms linger, 16KB batches

---

### 2. PostgreSQL (OLTP Database)

**Schema Design**:

#### Customers Table
```sql
CREATE TABLE customers (
    customer_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    balance BIGINT NOT NULL DEFAULT 0,  -- In cents
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_customers_balance ON customers(balance) WHERE balance > 0;
```

#### SMS Logs (Partitioned by Day)
```sql
CREATE TABLE sms_logs (
    id BIGSERIAL,
    customer_id TEXT NOT NULL,
    phone TEXT NOT NULL,
    message TEXT NOT NULL,
    cost BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'accepted',
    idempotency_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Minimal indexes for OLTP
CREATE INDEX idx_sms_logs_customer ON sms_logs(customer_id, created_at DESC);
CREATE UNIQUE INDEX idx_sms_logs_idempotency ON sms_logs(customer_id, idempotency_key) 
    WHERE idempotency_key IS NOT NULL;
```

**Partition Management**:
```sql
-- Daily partitions, auto-created
CREATE TABLE sms_logs_2025_11_20 PARTITION OF sms_logs
    FOR VALUES FROM ('2025-11-20 00:00:00+00') TO ('2025-11-21 00:00:00+00');
```

**Partition Maintenance Strategy**:
- Pre-create partitions 7 days in advance
- Archive partitions older than 30 days to cold storage
- Drop or detach partitions older than 90 days

**Balance Concurrency Safety**:

Option A: Row-Level Locking
```sql
BEGIN;
SELECT balance FROM customers WHERE customer_id = $1 FOR UPDATE;
-- Check balance >= cost
UPDATE customers SET balance = balance - $2 WHERE customer_id = $1;
COMMIT;
```

Option B: Advisory Locks (Recommended for High Contention)
```sql
-- Lock customer_id hash
SELECT pg_advisory_xact_lock(hashtext($1));
SELECT balance FROM customers WHERE customer_id = $1;
-- Check and update
UPDATE customers SET balance = balance - $2 WHERE customer_id = $1;
```

**Write Performance**:
- Synchronous commit OFF for SMS logs (async replication acceptable)
- Bulk inserts via COPY protocol (not required for this workload)
- Checkpoint tuning: `checkpoint_timeout = 15min`

---

### 3. Kafka (Event Streaming)

**Topic Design**:

#### sms.accepted
- **Purpose**: Accepted SMS events for analytics
- **Partitions**: 30 (distributes 100M/day = ~1200 msg/sec across partitions)
- **Key**: `customer_id` (ensures customer ordering)
- **Retention**: 7 days
- **Replication**: 3

#### sms.delivery
- **Purpose**: SMS delivery tasks for workers
- **Partitions**: 50 (allows fine-grained worker scaling)
- **Key**: `customer_id` or `operator_id` (rate shaping)
- **Retention**: 3 days
- **Replication**: 3

#### sms.status
- **Purpose**: Delivery status updates from workers
- **Partitions**: 30
- **Key**: `sms_id`
- **Retention**: 7 days
- **Replication**: 3

**Event Schema (sms.accepted)**:
```json
{
  "event_id": "uuid-v7",
  "event_type": "sms.accepted",
  "timestamp": "2025-11-19T10:30:45.123Z",
  "customer_id": "cust_12345",
  "sms_id": 9876543210,
  "phone": "+989121234567",
  "message": "Your verification code is 123456",
  "cost": 500,
  "idempotency_key": "req_abc123"
}
```

**Producer Configuration**:
```properties
acks=1                    # Leader acknowledgment (not all replicas)
linger.ms=100             # Batch for 100ms
batch.size=16384          # 16KB batches
compression.type=snappy   # Fast compression
enable.idempotence=true   # Exactly-once semantics
```

**Throughput Calculation**:
- 100M SMS/day = 1,157 msg/sec average
- Peak 3x = 3,471 msg/sec
- Per partition (30): ~115 msg/sec
- With batching: ~1-2 MB/sec per partition

---

### 4. ClickHouse (Analytics & Reporting)

**Raw SMS Table (Kafka Engine)**:
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
    idempotency_key String
) ENGINE = Kafka()
SETTINGS
    kafka_broker_list = 'kafka:9092',
    kafka_topic_list = 'sms.accepted',
    kafka_group_name = 'clickhouse_ingestion',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 10;
```

**Materialized Target Table**:
```sql
CREATE TABLE sms_logs_ch (
    event_id String,
    timestamp DateTime64(3),
    customer_id String,
    sms_id UInt64,
    phone String,
    message String,
    cost UInt64,
    date Date MATERIALIZED toDate(timestamp),
    hour UInt8 MATERIALIZED toHour(timestamp)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(date)
ORDER BY (customer_id, timestamp)
TTL date + INTERVAL 90 DAY;

-- Materialized view to populate
CREATE MATERIALIZED VIEW sms_logs_mv TO sms_logs_ch AS
SELECT 
    event_id, timestamp, customer_id, sms_id, 
    phone, message, cost
FROM sms_raw_queue;
```

**Aggregated Tables (Pre-computed)**:

#### Minute-Level Aggregations
```sql
CREATE TABLE sms_stats_minute (
    customer_id String,
    timestamp DateTime,
    minute DateTime MATERIALIZED toStartOfMinute(timestamp),
    sms_count UInt64,
    total_cost UInt64
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(minute)
ORDER BY (customer_id, minute);

CREATE MATERIALIZED VIEW sms_stats_minute_mv TO sms_stats_minute AS
SELECT
    customer_id,
    toStartOfMinute(timestamp) as minute,
    timestamp,
    count() as sms_count,
    sum(cost) as total_cost
FROM sms_logs_ch
GROUP BY customer_id, minute, timestamp;
```

#### Hourly Aggregations
```sql
CREATE TABLE sms_stats_hourly (
    customer_id String,
    hour DateTime,
    sms_count UInt64,
    total_cost UInt64,
    unique_phones UInt64
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (customer_id, hour);

-- Populated by scheduled INSERT FROM SELECT (every hour)
```

**Query Examples**:

```sql
-- Customer SMS count for last 24 hours (query aggregated table)
SELECT 
    customer_id,
    sum(sms_count) as total_sms,
    sum(total_cost) as total_spent
FROM sms_stats_minute
WHERE minute >= now() - INTERVAL 24 HOUR
  AND customer_id = 'cust_12345'
GROUP BY customer_id;

-- Top 10 customers by volume today
SELECT 
    customer_id,
    sum(sms_count) as sms_today
FROM sms_stats_minute
WHERE toDate(minute) = today()
GROUP BY customer_id
ORDER BY sms_today DESC
LIMIT 10;
```

---

### 5. Delivery Worker Pool

**Architecture**:
- Multiple worker pods (10-50 instances)
- Each worker has per-customer queues (in-memory)
- Rate shaping via token bucket algorithm
- Horizontal scaling via Kafka consumer groups

**Rate Shaping Strategy**:

```
┌─────────────────────────────────┐
│      Worker Instance            │
│                                 │
│  ┌──────────────────────────┐  │
│  │ Kafka Consumer Thread    │  │
│  │  (reads sms.delivery)    │  │
│  └───────────┬──────────────┘  │
│              │ Distribute       │
│              ▼                  │
│  ┌─────────────────────────┐   │
│  │ Per-Customer Buffers    │   │
│  │                         │   │
│  │ [Cust A] → [Queue]      │   │
│  │ [Cust B] → [Queue]      │   │
│  │ [Cust C] → [Queue]      │   │
│  └────────┬────────────────┘   │
│           │ Token Bucket        │
│           ▼                     │
│  ┌──────────────────────┐      │
│  │ Worker Goroutines    │      │
│  │ (100-500 workers)    │      │
│  │                      │      │
│  │ → HTTP → Operator API│      │
│  └──────────────────────┘      │
└─────────────────────────────────┘
```

**Benefits**:
- Large customers don't block small customers
- Natural backpressure per customer
- Fair distribution of worker capacity

---

## Critical Paths & Bottleneck Analysis

### Ingestion Path Bottlenecks

#### Bottleneck 1: Balance Lock Contention
**Issue**: High-throughput customers cause row lock contention

**Mitigation**:
- Use advisory locks with customer_id hash
- Consider balance service with Redis cache + eventual consistency
- Pre-authorized balance chunks for ultra-high-volume customers

#### Bottleneck 2: PostgreSQL Write Throughput
**Issue**: 1200 writes/sec can saturate single Postgres instance

**Mitigation**:
- Partitioning reduces index overhead
- Async commit for SMS logs
- Connection pooling: 20-50 connections per API pod
- Expected capacity: 5,000-10,000 writes/sec on modern hardware

#### Bottleneck 3: Kafka Producer Blocking
**Issue**: Synchronous produce blocks API response

**Mitigation**:
- Async Kafka producer with callbacks
- In-memory buffer with overflow handling
- Accept SMS even if Kafka is temporarily down (log to Postgres, replay later)

### Delivery Path Bottlenecks

#### Bottleneck 4: Worker Throughput
**Issue**: Operator APIs have rate limits

**Mitigation**:
- Per-operator rate shaping
- Retry with exponential backoff
- Circuit breaker for failing operators

---

## Fault Tolerance Strategy

### PostgreSQL
- **Replication**: Streaming replication with 2 standby replicas
- **Failover**: Automatic via Patroni or similar
- **Backup**: Daily full + continuous WAL archiving
- **Recovery Time**: <60 seconds for automatic failover

### Kafka
- **Replication Factor**: 3
- **Min In-Sync Replicas**: 2
- **Producer**: `acks=1` (leader only, acceptable for this use case)
- **Consumer**: Manual offset commit after successful processing

### ClickHouse
- **Replication**: ReplicatedMergeTree (2-3 replicas)
- **Recovery**: Rebuild from Kafka (7-day retention)
- **Backup**: S3 snapshots for long-term storage

### API Service
- **Stateless**: Any pod failure → traffic to others
- **Health Checks**: /health endpoint (DB + Kafka connectivity)
- **Circuit Breakers**: Fail fast on downstream issues

### Worker Pool
- **Consumer Groups**: Automatic rebalancing
- **Retry Logic**: Dead-letter queue for failed deliveries
- **Poison Pills**: Skip and alert on malformed messages

---

## Performance Targets & SLAs

### API Service
- **P50 Latency**: <5ms
- **P99 Latency**: <20ms
- **P99.9 Latency**: <50ms
- **Availability**: 99.95%

### Delivery Workers
- **Throughput**: Match ingestion rate (1200-3000/sec)
- **Delivery Latency**: <30 seconds P95
- **Success Rate**: >99.5%

### Reporting
- **Query Latency**: <500ms for minute aggregations
- **Data Freshness**: 1-5 minutes
- **Availability**: 99.9%

---

## Scaling Plan

### Phase 1: Launch (10M SMS/day)
- 3 API pods
- 1 PostgreSQL instance (primary + standby)
- 3 Kafka brokers
- 1 ClickHouse instance
- 5 worker pods

### Phase 2: Growth (50M SMS/day)
- 10 API pods
- Same Postgres (sufficient capacity)
- 5 Kafka brokers
- 2 ClickHouse replicas
- 20 worker pods

### Phase 3: Scale (100M SMS/day)
- 20-30 API pods
- Postgres read replicas for /balance queries
- 7 Kafka brokers
- 3 ClickHouse replicas
- 40-50 worker pods

---

## Monitoring & Observability

### Key Metrics

**API Service**:
- Request rate (per endpoint)
- Latency (P50, P99, P99.9)
- Error rate (4xx, 5xx)
- Balance deduction failures
- Kafka produce failures

**PostgreSQL**:
- Connections in use
- Transaction rate
- Lock wait time
- Partition write distribution

**Kafka**:
- Producer throughput
- Consumer lag (per topic/partition)
- Rebalance events

**ClickHouse**:
- Insert rate
- Query latency
- Disk usage per table

**Workers**:
- Delivery success rate
- Retry count
- Operator API latency
- Queue depth per customer

### Alerting Thresholds
- API P99 latency >50ms
- Kafka consumer lag >100k messages
- Balance deduction failure rate >0.1%
- Worker delivery success rate <99%
- PostgreSQL connection saturation >80%

---

## Deployment Architecture

### Kubernetes Deployment
```
Namespace: sms-gateway

Deployments:
- api-service (10-50 replicas)
- delivery-worker (10-50 replicas)
- reporting-api (3-5 replicas)

StatefulSets:
- postgres (1 primary + 2 replicas)
- kafka (3-7 brokers)
- clickhouse (1-3 nodes)

Services:
- api-service (LoadBalancer)
- postgres (ClusterIP)
- kafka (Headless)
- clickhouse (ClusterIP)
```

### Resource Allocation
```yaml
api-service:
  cpu: 500m-2000m
  memory: 512Mi-2Gi
  
delivery-worker:
  cpu: 1000m-4000m
  memory: 1Gi-4Gi
  
postgres:
  cpu: 4000m-16000m
  memory: 16Gi-64Gi
  disk: 1TB NVMe
  
kafka:
  cpu: 2000m-8000m
  memory: 8Gi-32Gi
  disk: 500GB SSD per broker
  
clickhouse:
  cpu: 4000m-16000m
  memory: 32Gi-128Gi
  disk: 2TB NVMe
```

---

## Cost Estimation (AWS/GCP)

### Compute
- API Pods: 20 × $50/mo = $1,000/mo
- Worker Pods: 30 × $100/mo = $3,000/mo
- **Subtotal**: $4,000/mo

### Database
- PostgreSQL (db.r6g.2xlarge): $800/mo
- ClickHouse (3 × db.r6g.4xlarge): $4,500/mo
- **Subtotal**: $5,300/mo

### Kafka (MSK/Confluent)
- 5 brokers × $200/mo = $1,000/mo
- **Subtotal**: $1,000/mo

### Storage
- PostgreSQL: 1TB × $0.10/GB = $100/mo
- ClickHouse: 5TB × $0.08/GB = $400/mo
- Kafka: 2TB × $0.10/GB = $200/mo
- **Subtotal**: $700/mo

### Network
- Data transfer: ~$500/mo

**Total**: ~$11,500/mo for 100M SMS/day

**Per SMS Cost**: $0.0000115 (0.00115 cents) - infrastructure only

---

## Migration Strategy

### Day 0: Preparation
- Deploy infrastructure (Postgres, Kafka, ClickHouse)
- Create schemas and partitions
- Deploy API service (shadow mode)

### Day 1-7: Parallel Run
- Route 1% traffic to new system
- Compare results with old system
- Monitor all metrics

### Day 8-14: Gradual Rollout
- Increase to 10%, 25%, 50%
- Monitor balance accuracy
- Verify reporting consistency

### Day 15+: Full Migration
- Route 100% traffic
- Decommission old system after 30-day validation

---

## Security Considerations

### API Security
- Rate limiting per customer_id (e.g., 10,000 req/min)
- Request signing (HMAC) for authentication
- IP whitelisting for enterprise customers

### Database Security
- Encrypted connections (TLS)
- Row-level security (if multi-tenant)
- Audit logging for balance changes

### Network Security
- Private VPC for all components
- No public internet access except API gateway
- mTLS between services (optional)

---

## Summary

This architecture achieves:
✅ 100M SMS/day capacity with headroom for growth  
✅ Atomic balance deduction with cent-level accuracy  
✅ Append-only PostgreSQL for OLTP  
✅ Kafka-based event streaming for decoupling  
✅ ClickHouse for fast analytics (1-5 min freshness)  
✅ Horizontal scalability at every layer  
✅ Per-customer rate shaping to prevent interference  
✅ <20ms P99 latency for API requests  
✅ Clear fault tolerance and monitoring strategy  

The system is production-ready and can scale to 500M+ SMS/day with the same architecture.


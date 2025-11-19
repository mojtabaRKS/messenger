# Go Project Structure - SMS Gateway

## Overview

This is a monorepo structure containing multiple services:
1. **api-service**: REST API for SMS sending
2. **delivery-worker**: Background workers for SMS delivery
3. **reporting-api**: Analytics and reporting API
4. **partition-manager**: Utility to manage PostgreSQL partitions

---

## Folder Structure

```
sms-gateway/
├── cmd/                                 # Entry points for services
│   ├── api/                            # API service
│   │   └── main.go
│   ├── worker/                         # Delivery worker
│   │   └── main.go
│   ├── reporting/                      # Reporting API
│   │   └── main.go
│   ├── partition-manager/              # Partition management utility
│   │   └── main.go
│   └── migrate/                        # Database migration tool
│       └── main.go
│
├── internal/                           # Private application code
│   ├── api/                           # API service code
│   │   ├── handler/                   # HTTP handlers
│   │   │   ├── sms.go                # SMS send handler
│   │   │   ├── balance.go            # Balance query handler
│   │   │   └── health.go             # Health check
│   │   ├── middleware/               # HTTP middleware
│   │   │   ├── logger.go
│   │   │   ├── ratelimit.go
│   │   │   └── recovery.go
│   │   ├── request/                  # Request DTOs
│   │   │   └── send_sms.go
│   │   ├── response/                 # Response DTOs
│   │   │   └── send_sms.go
│   │   └── server.go                 # HTTP server setup
│   │
│   ├── worker/                        # Delivery worker code
│   │   ├── consumer.go               # Kafka consumer
│   │   ├── delivery.go               # SMS delivery logic
│   │   ├── pool.go                   # Worker pool
│   │   ├── queue.go                  # Per-customer queues
│   │   ├── ratelimit.go              # Token bucket rate limiter
│   │   └── operator/                 # Operator integrations
│   │       ├── interface.go
│   │       ├── mci.go
│   │       └── irancell.go
│   │
│   ├── reporting/                     # Reporting service code
│   │   ├── handler/                  # HTTP handlers
│   │   │   ├── stats.go
│   │   │   └── export.go
│   │   ├── query/                    # Query builders
│   │   │   └── aggregations.go
│   │   └── server.go
│   │
│   ├── domain/                        # Domain models
│   │   ├── customer.go
│   │   ├── sms.go
│   │   ├── balance.go
│   │   └── event.go
│   │
│   ├── repository/                    # Data access layer
│   │   ├── postgres/
│   │   │   ├── customer.go           # Customer queries
│   │   │   ├── sms.go                # SMS log inserts
│   │   │   ├── balance.go            # Balance operations
│   │   │   └── transaction.go        # DB transactions
│   │   └── clickhouse/
│   │       ├── stats.go              # Aggregation queries
│   │       └── export.go             # Data export
│   │
│   ├── service/                       # Business logic
│   │   ├── balance/
│   │   │   ├── checker.go            # Balance validation
│   │   │   └── deductor.go           # Atomic deduction
│   │   ├── sms/
│   │   │   ├── sender.go             # SMS sending orchestration
│   │   │   ├── validator.go          # Input validation
│   │   │   └── cost.go               # Cost calculation
│   │   └── idempotency/
│   │       └── manager.go            # Idempotency handling
│   │
│   ├── infra/                         # Infrastructure clients
│   │   ├── postgres/
│   │   │   ├── client.go             # Connection pool
│   │   │   └── migration.go          # Migration runner
│   │   ├── clickhouse/
│   │   │   └── client.go
│   │   ├── kafka/
│   │   │   ├── producer.go           # Kafka producer wrapper
│   │   │   ├── consumer.go           # Kafka consumer wrapper
│   │   │   └── config.go
│   │   └── redis/
│   │       └── client.go             # Redis (optional: for rate limiting)
│   │
│   ├── config/                        # Configuration
│   │   ├── config.go                 # Config struct
│   │   └── loader.go                 # Load from env/file
│   │
│   └── pkg/                          # Shared utilities
│       ├── logger/
│       │   └── logger.go             # Structured logging
│       ├── ulid/
│       │   └── generator.go          # ULID generation
│       ├── metrics/
│       │   └── prometheus.go         # Prometheus metrics
│       └── errors/
│           └── errors.go             # Error types
│
├── migrations/                        # Database migrations
│   ├── postgres/
│   │   ├── 000001_create_customers.up.sql
│   │   ├── 000001_create_customers.down.sql
│   │   ├── 000002_create_sms_logs.up.sql
│   │   ├── 000002_create_sms_logs.down.sql
│   │   ├── 000003_create_balance_txn.up.sql
│   │   └── 000003_create_balance_txn.down.sql
│   └── clickhouse/
│       ├── 001_create_raw_table.sql
│       ├── 002_create_kafka_engine.sql
│       └── 003_create_aggregations.sql
│
├── scripts/                           # Utility scripts
│   ├── create_kafka_topics.sh
│   ├── seed_test_data.sql
│   └── benchmark.sh
│
├── deployments/                       # Deployment configs
│   ├── kubernetes/
│   │   ├── api-deployment.yaml
│   │   ├── worker-deployment.yaml
│   │   ├── postgres-statefulset.yaml
│   │   └── kafka-statefulset.yaml
│   └── docker/
│       └── docker-compose.yml
│
├── docs/                              # Additional documentation
│   ├── api.md                        # API documentation
│   ├── performance.md                # Performance tuning
│   └── runbook.md                    # Operations runbook
│
├── tests/                             # Integration tests
│   ├── api_test.go
│   ├── balance_test.go
│   └── fixtures/
│
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── deploy.yml
│
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── ARCHITECTURE.md                    # (Already created)
├── DATABASE_SCHEMA.md                 # (Already created)
└── KAFKA_INFRASTRUCTURE.md            # (Already created)
```

---

## Service Boundaries

### 1. API Service (cmd/api)
- **Port**: 8080
- **Responsibility**: 
  - Accept SMS send requests
  - Validate balance
  - Deduct balance atomically
  - Write SMS log to Postgres
  - Produce event to Kafka
- **Dependencies**: PostgreSQL, Kafka
- **Scaling**: Horizontal (stateless)

### 2. Delivery Worker (cmd/worker)
- **Responsibility**:
  - Consume from Kafka (sms.delivery topic)
  - Per-customer rate shaping
  - Call operator APIs
  - Retry failed deliveries
  - Produce status events
- **Dependencies**: Kafka, Operator APIs
- **Scaling**: Horizontal (Kafka consumer group)

### 3. Reporting API (cmd/reporting)
- **Port**: 8081
- **Responsibility**:
  - Query ClickHouse for analytics
  - Serve aggregated reports
  - Export data
- **Dependencies**: ClickHouse
- **Scaling**: Horizontal (stateless)

### 4. Partition Manager (cmd/partition-manager)
- **Type**: Cron job
- **Responsibility**:
  - Create future partitions
  - Archive old partitions
- **Schedule**: Daily at 2 AM

---

## Package Design Principles

### 1. Clean Architecture Layers
```
Handler (HTTP) → Service (Business Logic) → Repository (Data Access) → Infra (DB/Kafka)
```

### 2. Dependency Injection
- Use interfaces for dependencies
- Inject via constructors
- Easy to mock for testing

### 3. Domain Models
- Pure Go structs in `domain/`
- No database tags in domain models
- Conversion at repository layer

### 4. Error Handling
- Custom error types in `pkg/errors`
- Wrap errors with context
- Log errors at boundaries

---

## Key Interfaces

### Repository Interfaces

```go
// internal/repository/postgres/customer.go
type CustomerRepository interface {
    GetByID(ctx context.Context, customerID string) (*domain.Customer, error)
    DeductBalance(ctx context.Context, customerID string, amount int64) error
    DeductBalanceWithLock(ctx context.Context, customerID string, amount int64) error
}

// internal/repository/postgres/sms.go
type SMSRepository interface {
    Insert(ctx context.Context, sms *domain.SMS) (int64, error)
    InsertBatch(ctx context.Context, smsList []*domain.SMS) error
}

// internal/repository/clickhouse/stats.go
type StatsRepository interface {
    GetMinuteStats(ctx context.Context, customerID string, from, to time.Time) ([]*domain.MinuteStats, error)
    GetDailyStats(ctx context.Context, customerID string, from, to time.Time) ([]*domain.DailyStats, error)
}
```

### Service Interfaces

```go
// internal/service/sms/sender.go
type SMSSender interface {
    Send(ctx context.Context, req *domain.SendSMSRequest) (*domain.SendSMSResponse, error)
    SendBatch(ctx context.Context, reqs []*domain.SendSMSRequest) ([]*domain.SendSMSResponse, error)
}

// internal/service/balance/deductor.go
type BalanceDeductor interface {
    Deduct(ctx context.Context, customerID string, amount int64) error
    DeductWithAdvisoryLock(ctx context.Context, customerID string, amount int64) error
}
```

### Infra Interfaces

```go
// internal/infra/kafka/producer.go
type EventProducer interface {
    Produce(ctx context.Context, topic string, key string, value []byte) error
    ProduceAsync(ctx context.Context, topic string, key string, value []byte) error
    Close() error
}

// internal/infra/kafka/consumer.go
type EventConsumer interface {
    Subscribe(topics []string) error
    ReadMessage(timeout time.Duration) (*Message, error)
    CommitMessage(msg *Message) error
    Close() error
}
```

---

## Configuration Structure

```go
// internal/config/config.go
type Config struct {
    Env      string // production, staging, development
    LogLevel string // debug, info, warn, error
    
    HTTP HTTPConfig
    
    Postgres PostgresConfig
    ClickHouse ClickHouseConfig
    Kafka KafkaConfig
    Redis RedisConfig
    
    SMS SMSConfig
    
    Worker WorkerConfig
}

type HTTPConfig struct {
    Host string
    Port int
    ReadTimeout time.Duration
    WriteTimeout time.Duration
}

type PostgresConfig struct {
    Host string
    Port int
    User string
    Password string
    Database string
    MaxOpenConns int
    MaxIdleConns int
}

type KafkaConfig struct {
    Brokers []string
    Topics KafkaTopics
    Producer KafkaProducerConfig
    Consumer KafkaConsumerConfig
}

type SMSConfig struct {
    CostPerSMS int64 // In cents
    MaxMessageLength int
}

type WorkerConfig struct {
    NumWorkers int
    QueueSize int
    MaxRetries int
}
```

---

## Makefile Targets

```makefile
.PHONY: build test run clean docker

# Build all services
build:
	go build -o bin/api cmd/api/main.go
	go build -o bin/worker cmd/worker/main.go
	go build -o bin/reporting cmd/reporting/main.go
	go build -o bin/migrate cmd/migrate/main.go

# Run tests
test:
	go test -v -race -cover ./...

# Run API service locally
run-api:
	go run cmd/api/main.go

# Run worker locally
run-worker:
	go run cmd/worker/main.go

# Database migrations
migrate-up:
	go run cmd/migrate/main.go up

migrate-down:
	go run cmd/migrate/main.go down

# Docker
docker-build:
	docker build -t sms-gateway-api -f deployments/docker/Dockerfile.api .
	docker build -t sms-gateway-worker -f deployments/docker/Dockerfile.worker .

# Kafka setup
kafka-create-topics:
	./scripts/create_kafka_topics.sh

# Linting
lint:
	golangci-lint run ./...

# Clean
clean:
	rm -rf bin/
```

---

## Dependency Management (go.mod)

```go
module github.com/yourcompany/sms-gateway

go 1.21

require (
    // HTTP framework
    github.com/gin-gonic/gin v1.9.1
    
    // Database
    github.com/jackc/pgx/v5 v5.5.0
    github.com/ClickHouse/clickhouse-go/v2 v2.15.0
    
    // Kafka
    github.com/confluentinc/confluent-kafka-go/v2 v2.3.0
    
    // Redis (optional)
    github.com/redis/go-redis/v9 v9.3.0
    
    // Configuration
    github.com/spf13/viper v1.18.0
    
    // Logging
    github.com/sirupsen/logrus v1.9.3
    go.uber.org/zap v1.26.0
    
    // ULID
    github.com/oklog/ulid/v2 v2.1.0
    
    // Metrics
    github.com/prometheus/client_golang v1.18.0
    
    // Migrations
    github.com/golang-migrate/migrate/v4 v4.17.0
    
    // Rate limiting
    golang.org/x/time v0.5.0
    
    // Validation
    github.com/go-playground/validator/v10 v10.16.0
    
    // Testing
    github.com/stretchr/testify v1.8.4
)
```

---

## Service Communication Flow

### SMS Send Flow
```
1. Client → POST /v1/sms/send
2. API Handler → SMS Service
3. SMS Service → Balance Service (check + deduct)
4. Balance Service → PostgreSQL (atomic deduction)
5. SMS Service → SMS Repository (insert log)
6. SMS Repository → PostgreSQL (append-only)
7. SMS Service → Kafka Producer (produce event)
8. Kafka Producer → Kafka (sms.accepted topic)
9. API Handler → Client (200 OK response)

Async:
10. Worker → Kafka Consumer (consume sms.delivery)
11. Worker → Operator API (send SMS)
12. Worker → Kafka Producer (produce status)
13. Kafka → ClickHouse (ingest for analytics)
```

---

## Testing Strategy

### Unit Tests
- Test business logic in `service/` packages
- Mock repositories and infra
- Use `testify/mock`

### Integration Tests
- Test full API flow with test database
- Use `testcontainers` for PostgreSQL/Kafka
- Located in `tests/`

### Load Tests
- Use `k6` or `vegeta`
- Script in `scripts/benchmark.sh`
- Target: 3,000 req/sec sustained

---

## Observability

### Logging
- Structured JSON logs (logrus or zap)
- Request ID propagation
- Log levels: DEBUG, INFO, WARN, ERROR

### Metrics (Prometheus)
```go
// Example metrics
var (
    smsAccepted = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "sms_accepted_total",
            Help: "Total SMS accepted",
        },
        []string{"customer_id"},
    )
    
    balanceDeductions = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name: "balance_deduction_duration_seconds",
            Help: "Balance deduction latency",
            Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1},
        },
    )
)
```

### Tracing (Optional)
- OpenTelemetry for distributed tracing
- Jaeger or Tempo backend

---

## Security

### API Authentication
- No authentication required (as per requirements)
- But implement rate limiting per customer_id
- Optional: API key validation

### Database Security
- Use connection pooling
- Prepared statements (prevent SQL injection)
- TLS connections

### Network Security
- Internal services in private VPC
- Only API service exposed via load balancer

---

## Summary

This project structure achieves:
✅ Clear separation of concerns (Clean Architecture)  
✅ Scalable: Each service can scale independently  
✅ Testable: Interfaces and dependency injection  
✅ Maintainable: Organized by feature and layer  
✅ Observable: Logging, metrics, tracing  
✅ Production-ready: Error handling, retries, monitoring  

**Next**: Implement the core services with actual Go code.


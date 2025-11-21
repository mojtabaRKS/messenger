package constant

import (
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	RedisPlanKey       = "arvan:plans"
	TopicAccepted      = "sms.accepted"
	TopicStatus        = "sms.status"
	KafkaProducerAcks  = kafka.RequireAll
	KafkaWorkerCount   = 4
	KafkaWorkerBufSize = 100000 // capacity of in-memory channel; tune by memory and expected bursts
	KafkaRetryBackoff  = 500 * time.Millisecond
	DBTxTimeout        = 2 * time.Second // keep transactions short

	// Kafka
	KafkaGroupID        = "sms-processor-group"
	KafkaReaderMinBytes = 1
	KafkaReaderMaxBytes = 10e6

	// Channels & Workers
	IntakeChanSize       = 20000           // cap of messages buffered between kafka reader and scheduler (tune)
	ReadyQueueSize       = 100000          // cap of customerIDs ready to be served
	WorkerPoolSize       = 64              // number of concurrent processing goroutines
	CustomerPendingLimit = 1000            // per-customer pending cap before we start dropping (or take action)
	CustomerStateTTL     = 5 * time.Minute // when no activity, remove the customer state to free memory

	// Token bucket defaults (per-customer default; can be customized per customer)
	DefaultRefillRate = 10.0  // tokens per second
	DefaultCapacity   = 100.0 // maximum tokens

	// Timeouts and retries
	KafkaWriteTimeout   = 5 * time.Second
	KafkaWriteRetries   = 3
	StatusInsertTimeout = 2 * time.Second
	DLQInsertTimeout    = 2 * time.Second

	UserIdKey   = "user_id"
	PriorityKey = "priority"

	// Redis balance cache settings
	BalanceKeyPrefix     = "balance:"
	BalanceSyncBatchSize = 500             // batch size for DB sync
	BalanceSyncInterval  = 5 * time.Second // sync interval for balance updates

	// Kafka producer worker pool
	KafkaWriteWorkerPool = 50 // fixed worker pool size for Kafka writes
)

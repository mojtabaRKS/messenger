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
	KafkaWorkerBufSize = 100000
	KafkaRetryBackoff  = 500 * time.Millisecond
	DBTxTimeout        = 2 * time.Second

	DefaultPageSize    = 20
	DefaultCurrentPage = 1

	// Kafka
	KafkaGroupID = "sms-processor-group"

	// Timeouts and retries
	KafkaWriteTimeout = 5 * time.Second
	KafkaWriteRetries = 3

	UserIdKey   = "user_id"
	PriorityKey = "priority"

	// Redis balance cache settings
	BalanceKeyPrefix     = "balance:"
	BalanceSyncBatchSize = 500
	BalanceSyncInterval  = 800 * time.Millisecond
	BalanceQueueSize     = 100000
	BalanceWriterWorkers = 6

	// Kafka producer worker pool
	KafkaWriteWorkerPool = 50
)

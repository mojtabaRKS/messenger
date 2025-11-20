package constant

import (
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	RedisPlanKey       = "arvan:plans"
	KafkaTopic         = "sms.accepted"
	KafkaProducerAcks  = kafka.RequireAll
	KafkaWriteTimeout  = 5 * time.Second
	KafkaWorkerCount   = 4
	KafkaWorkerBufSize = 10000 // capacity of in-memory channel; tune by memory and expected bursts
	KafkaWriteRetries  = 3
	KafkaRetryBackoff  = 500 * time.Millisecond
	DBTxTimeout        = 2 * time.Second // keep transactions short
)

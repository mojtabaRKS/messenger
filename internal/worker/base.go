package worker

import (
	"arvan/message-gateway/internal/domain"
	"arvan/message-gateway/internal/provider"
	"github.com/segmentio/kafka-go"
	"sync"
)

// WorkerPool coordinates a set of workers that process jobs.
type WorkerPool struct {
	qm         domain.QueueManager
	provider   provider.SMSProvider
	numWorkers int
	wg         sync.WaitGroup

	kafkaSmsStatusWriter *kafka.Writer
}

// NewWorkerPool constructs a pool that will use provided QueueManager and SMSProvider.
func NewWorkerPool(
	qm domain.QueueManager,
	prov provider.SMSProvider,
	numWorkers int,
	kafkaSmsStatusWriter *kafka.Writer,
) *WorkerPool {
	return &WorkerPool{
		qm:                   qm,
		provider:             prov,
		numWorkers:           numWorkers,
		kafkaSmsStatusWriter: kafkaSmsStatusWriter,
	}
}

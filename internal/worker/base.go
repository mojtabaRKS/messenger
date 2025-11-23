package worker

import (
	"arvan/message-gateway/internal/domain"
	"arvan/message-gateway/internal/provider"
	"github.com/segmentio/kafka-go"
	"sync"
)

type WorkerPool struct {
	qm         domain.QueueManager
	provider   provider.SMSProvider
	numWorkers int
	wg         sync.WaitGroup

	kafkaSmsStatusWriter *kafka.Writer
}

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

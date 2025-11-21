package worker

import (
	"arvan/message-gateway/internal/domain"
	"arvan/message-gateway/internal/provider"
	"context"
	"sync"
)

// WorkerPool coordinates a set of workers that process jobs.
type WorkerPool struct {
	qm         domain.QueueManager
	provider   provider.SMSProvider
	numWorkers int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWorkerPool constructs a pool that will use provided QueueManager and SMSProvider.
func NewWorkerPool(qm domain.QueueManager, prov provider.SMSProvider, numWorkers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		qm:         qm,
		provider:   prov,
		numWorkers: numWorkers,
		ctx:        ctx,
		cancel:     cancel,
	}
}

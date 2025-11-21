package worker

import (
	"context"
	"log"
)

func (p *WorkerPool) Start(ctx context.Context) {
	for i := 0; i < p.numWorkers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
	log.Printf("worker pool: started %d workers", p.numWorkers)
}

// Stop gracefully stops all workers and waits.
func (p *WorkerPool) Stop(ctx context.Context) {
	_, cancel := context.WithCancel(ctx)
	defer cancel()
	p.wg.Wait()
	log.Println("worker pool: all workers stopped")
}

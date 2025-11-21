package worker

import (
	"log"
)

func (p *WorkerPool) Start() {
	for i := 0; i < p.numWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	log.Printf("worker pool: started %d workers", p.numWorkers)
}

// Stop gracefully stops all workers and waits.
func (p *WorkerPool) Stop() {
	p.cancel()
	p.wg.Wait()
	log.Println("worker pool: all workers stopped")
}

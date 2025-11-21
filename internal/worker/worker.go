package worker

import (
	"arvan/message-gateway/internal/domain"
	log "github.com/sirupsen/logrus"
	"time"
)

// worker executes in its own goroutine and processes jobs from queue manager one job at a time per customer.
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			log.Printf("worker %d: context cancelled, exiting", id)
			return
		default:
		}

		// Ask queue manager for a customer to process
		customerID, ok := p.qm.SelectNextCustomer()
		if !ok {
			// wait for new jobs or backoff
			select {
			case <-p.ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}

		// Attempt to dequeue a job for this customer
		job, err := p.qm.Dequeue(customerID)
		if err != nil {
			// Nothing to do — release lock and continue
			p.qm.UnlockCustomer(customerID)
			continue
		}

		// Process the job
		p.processJob(id, job)

		// Unlock the customer (allow other workers to process)
		p.qm.UnlockCustomer(customerID)
	}
}

func (p *WorkerPool) processJob(workerID int, job domain.Job) {
	log.Printf("worker %d: processing job %s for customer %s (phone=%s)", workerID, job.ID, job.CustomerID, job.Phone)
	start := time.Now()
	err := p.provider.Send(job)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("worker %d: job %s FAILED after %s: %v", workerID, job.ID, elapsed, err)
	} else {
		log.Printf("worker %d: job %s OK (elapsed=%s)", workerID, job.ID, elapsed)
	}
}

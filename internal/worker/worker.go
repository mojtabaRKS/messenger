package worker

import (
	"arvan/message-gateway/internal/domain"
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"time"
)

// worker executes in its own goroutine and processes jobs from queue manager one job at a time per customer.
func (p *WorkerPool) worker(ctx context.Context, id int) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			log.Printf("worker %d: context cancelled, exiting", id)
			return
		default:
		}

		// Ask queue manager for a customer to process
		customerID, ok := p.qm.SelectNextCustomer()
		if !ok {
			// wait for new jobs or backoff
			select {
			case <-ctx.Done():
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
		err = p.processJob(ctx, job)
		if err != nil {
			// requeue again for customer
			// probably couldn't queue for analysis
			err = p.qm.Enqueue(customerID, job)
			if err != nil {
				log.Printf("worker %d: enqueue job failed", id)
			}
			p.qm.UnlockCustomer(customerID)
			continue
		}

		// Unlock the customer (allow other workers to process)
		p.qm.UnlockCustomer(customerID)
	}
}

func (p *WorkerPool) processJob(ctx context.Context, job domain.Job) error {
	err := p.provider.Send(job)
	if err != nil {
		return err
	}

	msg := struct {
		domain.Job `json:",inline"`
		Status     string `json:"status"`
	}{
		job,
		"success",
	}

	marshalled, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if err = p.kafkaSmsStatusWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(job.ID),
		Value: marshalled,
		Time:  time.Now(),
	}); err != nil {
		return err
	}

	return nil
}

package queue

import (
	"arvan/message-gateway/internal/domain"
	"fmt"
)

func (q *customerQueue) Enqueue(job domain.Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, job)
	return nil
}

func (q *customerQueue) Dequeue() (domain.Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.jobs) == 0 {
		return domain.Job{}, fmt.Errorf("empty")
	}
	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job, nil
}

func (q *customerQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}

func (q *customerQueue) IsEmpty() bool {
	return q.Len() == 0
}

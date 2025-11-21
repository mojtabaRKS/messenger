package domain

// CustomerQueue defines the behavior of a per-customer FIFO queue.
type CustomerQueue interface {
	Enqueue(job Job) error
	Dequeue() (Job, error)
	Len() int
	IsEmpty() bool
}

// QueueManager is the dispatcher interface used by workers to select customers and access queues.
type QueueManager interface {
	Enqueue(customerID int, job Job) error
	Dequeue(customerID int) (Job, error)
	Len(customerID int) int

	// SelectNextCustomer chooses the next available unlocked customer using a fair algorithm.
	SelectNextCustomer() (int, bool)

	// UnlockCustomer releases the per-customer processing lock.
	UnlockCustomer(customerID int)
}

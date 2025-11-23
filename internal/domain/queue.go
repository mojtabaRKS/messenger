package domain

type CustomerQueue interface {
	Enqueue(job Job) error
	Dequeue() (Job, error)
	Len() int
	IsEmpty() bool
}

type QueueManager interface {
	Enqueue(customerID int, job Job) error
	Dequeue(customerID int) (Job, error)
	Len(customerID int) int

	SelectNextCustomer() (int, bool)

	UnlockCustomer(customerID int)
}

package queue

import (
	"arvan/message-gateway/internal/domain"
	"sync"
)

// customerQueue is a simple in-memory FIFO queue for a single customer.
type customerQueue struct {
	mu   sync.Mutex
	jobs []domain.Job
	id   int
}

// NewCustomerQueue constructs a new in-memory customer queue.
func NewCustomerQueue(customerID int) domain.CustomerQueue {
	return &customerQueue{
		jobs: make([]domain.Job, 0),
		id:   customerID,
	}
}

// QueueManager is the concrete in-memory implementation of domain.QueueManager.
type QueueManager struct {
	queues        sync.Map // map[int]domain.CustomerQueue - lock-free for read-heavy workload
	activeMu      sync.Mutex
	activeList    []int
	activeSet     map[int]bool
	lockedMu      sync.Mutex
	locked        map[int]bool
	roundRobinIdx int

	// Signal for workers that new jobs arrived; buffered to avoid blocking
	NewJobSignal chan struct{}
}

// NewQueueManager creates a new QueueManager instance.
func NewQueueManager() domain.QueueManager {
	return &QueueManager{
		queues:        sync.Map{},
		activeList:    make([]int, 0),
		activeSet:     make(map[int]bool),
		locked:        make(map[int]bool),
		NewJobSignal:  make(chan struct{}, 1),
		roundRobinIdx: 0,
	}
}

var (
	ErrQueueNotFound = &QueueError{"queue not found"}
)

// QueueError is a simple error type for queue manager.
type QueueError struct {
	Msg string
}

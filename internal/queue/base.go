package queue

import (
	"arvan/message-gateway/internal/domain"
	"sync"
)

type customerQueue struct {
	mu   sync.Mutex
	jobs []domain.Job
	id   int
}

func NewCustomerQueue(customerID int) domain.CustomerQueue {
	return &customerQueue{
		jobs: make([]domain.Job, 0),
		id:   customerID,
	}
}

type QueueManager struct {
	queues        sync.Map
	activeMu      sync.Mutex
	activeList    []int
	activeSet     map[int]bool
	lockedMu      sync.Mutex
	locked        map[int]bool
	roundRobinIdx int

	NewJobSignal chan struct{}
}

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

type QueueError struct {
	Msg string
}

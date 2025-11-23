package queue

import (
	"arvan/message-gateway/internal/domain"
)

func (qm *QueueManager) Enqueue(customerID int, job domain.Job) error {
	value, _ := qm.queues.LoadOrStore(customerID, NewCustomerQueue(customerID))
	q := value.(domain.CustomerQueue)

	if err := q.Enqueue(job); err != nil {
		return err
	}

	qm.activeMu.Lock()
	if !qm.activeSet[customerID] {
		qm.activeList = append(qm.activeList, customerID)
		qm.activeSet[customerID] = true
	}
	qm.activeMu.Unlock()

	select {
	case qm.NewJobSignal <- struct{}{}:
	default:
	}

	return nil
}

func (qm *QueueManager) Dequeue(customerID int) (domain.Job, error) {
	value, ok := qm.queues.Load(customerID)
	if !ok {
		return domain.Job{}, ErrQueueNotFound
	}
	q := value.(domain.CustomerQueue)

	job, err := q.Dequeue()
	if err != nil {
		return domain.Job{}, err
	}

	if q.IsEmpty() {
		qm.removeFromActive(customerID)
	}
	return job, nil
}

func (qm *QueueManager) Len(customerID int) int {
	value, ok := qm.queues.Load(customerID)
	if !ok {
		return 0
	}
	q := value.(domain.CustomerQueue)
	return q.Len()
}

func (qm *QueueManager) SelectNextCustomer() (int, bool) {
	qm.activeMu.Lock()
	defer qm.activeMu.Unlock()

	if len(qm.activeList) == 0 {
		return 0, false
	}

	maximum := len(qm.activeList)
	attempts := 0

	for attempts < maximum {
		if qm.roundRobinIdx >= len(qm.activeList) {
			qm.roundRobinIdx = 0
		}
		cust := qm.activeList[qm.roundRobinIdx]
		qm.roundRobinIdx++

		qm.lockedMu.Lock()
		locked := qm.locked[cust]
		qm.lockedMu.Unlock()

		if locked {
			attempts++
			continue
		}

		// check if queue still has jobs (avoid empty queues)
		if qm.Len(cust) == 0 {
			qm.removeFromActiveUnlocked(cust)
			continue
		}

		qm.lockedMu.Lock()
		qm.locked[cust] = true
		qm.lockedMu.Unlock()

		return cust, true
	}

	return 0, false
}

func (qm *QueueManager) UnlockCustomer(customerID int) {
	qm.lockedMu.Lock()
	delete(qm.locked, customerID)
	qm.lockedMu.Unlock()
}

func (qm *QueueManager) removeFromActive(customerID int) {
	qm.activeMu.Lock()
	defer qm.activeMu.Unlock()
	qm.removeFromActiveUnlocked(customerID)
}

func (qm *QueueManager) removeFromActiveUnlocked(customerID int) {
	if !qm.activeSet[customerID] {
		return
	}
	for i, id := range qm.activeList {
		if id == customerID {
			qm.activeList = append(qm.activeList[:i], qm.activeList[i+1:]...)
			break
		}
	}
	delete(qm.activeSet, customerID)
	if qm.roundRobinIdx >= len(qm.activeList) && len(qm.activeList) > 0 {
		qm.roundRobinIdx = 0
	}
}

func (e *QueueError) Error() string { return e.Msg }

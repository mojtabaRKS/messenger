package balance

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// BalanceService provides fast balance operations using Redis cache with DB persistence
type BalanceService struct {
	redisClient *redis.Client
	db          *gorm.DB
	logger      *logrus.Logger

	// Batch write queue
	pendingWrites chan *BalanceUpdate
	stopCh        chan struct{}
	wg            sync.WaitGroup

	// Pre-compiled Lua script for atomic balance deduction
	deductScript *redis.Script
}

type BalanceUpdate struct {
	MsgID      uuid.UUID
	CustomerID int
	ToNumber   string
	Body       string
	Timestamp  time.Time
}

// Lua script for atomic balance deduction (prevents race conditions)
var deductBalanceLua = redis.NewScript(`
	local key = KEYS[1]
	local deduction = tonumber(ARGV[1])
	local balance = tonumber(redis.call('GET', key) or 0)
	
	if balance >= deduction then
		redis.call('DECRBY', key, deduction)
		return balance - deduction
	else
		return -1
	end
`)

// NewBalanceService creates a new balance service with Redis caching
func NewBalanceService(
	redisClient *redis.Client,
	db *gorm.DB,
	logger *logrus.Logger,
) *BalanceService {
	bs := &BalanceService{
		redisClient:   redisClient,
		db:            db,
		logger:        logger,
		pendingWrites: make(chan *BalanceUpdate, 10000),
		stopCh:        make(chan struct{}),
		deductScript:  deductBalanceLua,
	}

	// Start batch writer goroutine
	bs.wg.Add(1)
	go bs.batchWriter()

	return bs
}

// Stop gracefully stops the balance service
func (bs *BalanceService) Stop() {
	close(bs.stopCh)
	bs.wg.Wait()
}

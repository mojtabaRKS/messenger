package middleware

import (
	"arvan/message-gateway/internal/constant"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type PriorityMiddleware struct {
	redisClient  *redis.Client
	data         map[string]int
	dataChecksum string
	planService  planService
	lastBucket   int64
	mu           sync.RWMutex
	stopCh       chan struct{}
}

type planService interface {
	GetAllPlansAndSetInRedis(ctx context.Context) (map[string]int, error)
}

func NewPriorityMiddleware(
	redisClient *redis.Client,
	planService planService,
	data map[string]int,
	dataChecksum string,
) *PriorityMiddleware {
	pm := &PriorityMiddleware{
		redisClient:  redisClient,
		data:         data,
		dataChecksum: dataChecksum,
		planService:  planService,
		stopCh:       make(chan struct{}),
	}

	// Start background refresh goroutine
	go pm.backgroundRefresh()

	return pm
}

// Stop gracefully stops the background refresh
func (m *PriorityMiddleware) Stop() {
	close(m.stopCh)
}

func (m *PriorityMiddleware) Handle(c *gin.Context) {
	apiKey := c.GetHeader("X-Api-Key")
	if apiKey == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "api key is empty"})
		return
	}

	// Fast read-only access to cached data (no blocking operations)
	m.mu.RLock()
	priority, exists := m.data[apiKey]
	m.mu.RUnlock()

	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "api key not found"})
		return
	}

	c.Set(constant.PriorityKey, priority)
	c.Next()
}

// backgroundRefresh runs in background and refreshes plan data every 5 minutes
func (m *PriorityMiddleware) backgroundRefresh() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			if err := m.refreshPlans(); err != nil {
				// Log but don't crash - continue with cached data
				// Logger would need to be added to struct, for now just continue
			}
		}
	}
}

func (m *PriorityMiddleware) refreshPlans() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	redisData, err := m.redisClient.Get(ctx, constant.RedisPlanKey).Result()
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write([]byte(redisData))
	hashed := h.Sum(nil)

	// Check if data changed
	m.mu.RLock()
	currentChecksum := m.dataChecksum
	m.mu.RUnlock()

	if currentChecksum == string(hashed) {
		return nil // No changes
	}

	// Fetch fresh data
	plans, err := m.planService.GetAllPlansAndSetInRedis(ctx)
	if err != nil {
		return err
	}

	marshalled, err := json.Marshal(plans)
	if err != nil {
		return err
	}

	h.Reset()
	h.Write(marshalled)
	newHash := h.Sum(nil)

	// Update with write lock
	m.mu.Lock()
	m.data = plans
	m.dataChecksum = string(newHash)
	m.mu.Unlock()

	return nil
}

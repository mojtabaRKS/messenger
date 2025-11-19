package middleware

import (
	"arvan/message-gateway/intrernal/constant"
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"net/http"
	"time"
)

type PriorityMiddleware struct {
	redisClient  *redis.Client
	data         map[string]int
	dataChecksum string
	planService  planService
}

var lastBucket int64

type planService interface {
	GetAllPlansAndSetInRedis(ctx context.Context) (map[string]int, error)
}

func NewPriorityMiddleware(
	redisClient *redis.Client,
	planService planService,
	data map[string]int,
	dataChecksum string,
) *PriorityMiddleware {
	return &PriorityMiddleware{
		redisClient:  redisClient,
		data:         data,
		dataChecksum: dataChecksum,
		planService:  planService,
	}
}

func (m *PriorityMiddleware) Handle(c *gin.Context) {
	err := m.CompareChecksum(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	apiKey := c.GetHeader("X-Api-Key")
	if apiKey == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "api key is empty"})
	}

	priority, exists := m.data[apiKey]
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "api key is empty"})
	}

	c.Set("priority", priority)
	c.Next()
}

func (m *PriorityMiddleware) CompareChecksum(ctx context.Context) error {
	now := time.Now().UnixMilli()
	shouldCompare := false

	// compare our plans every 5 minutes.
	// this leads to eventual consistency
	bucket := now / 300000
	if bucket != lastBucket {
		lastBucket = bucket
		shouldCompare = true
	}

	if !shouldCompare {
		return nil
	}

	redisData, err := m.redisClient.Get(ctx, constant.RedisPlanKey).Result()
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write([]byte(redisData))
	hashed := h.Sum(nil)
	// our checksum and plans are not changed! so ignore it!
	if m.dataChecksum == string(hashed) {
		return nil
	}

	// reset data in redis
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
	m.dataChecksum = string(newHash)

	return nil
}

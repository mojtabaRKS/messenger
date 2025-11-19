package middleware

import (
	"arvan/message-gateway/intrernal/api/request"
	"crypto/sha256"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"net/http"
)

type IdempotencyMiddleware struct {
	redisClient *redis.Client
}

func NewIdempotencyMiddleware(redisClient *redis.Client) *IdempotencyMiddleware {
	return &IdempotencyMiddleware{
		redisClient: redisClient,
	}
}

func (i *IdempotencyMiddleware) Handle(c *gin.Context) {
	var req request.SendSmsRequest
	err := c.BindJSON(req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "request is invalid"})
		return
	}

	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s", req.PhoneNumber, req.Message)))
	hashed := h.Sum(nil)

	exists, err := i.redisClient.Exists(c, fmt.Sprintf("%x", hashed)).Result()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	if exists > 0 {
		// its already accepted
		c.JSON(http.StatusAccepted, gin.H{
			"message": "message accepted",
		})
	} else {
		c.Next()
	}
}

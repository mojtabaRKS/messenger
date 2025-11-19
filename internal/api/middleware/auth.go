package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func HandleAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// we accept user_id in here
		// since we receive this from API-Gateway
		userId := c.GetHeader("X-Auth-User-Id")

		if userId == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": http.StatusUnauthorized,
				"msg":  "user is not authorized",
			})
			return
		}

		c.Set("user_id", userId)
		c.Next()
	}
}

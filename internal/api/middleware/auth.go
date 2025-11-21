package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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

		iUserId, err := strconv.Atoi(userId)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": http.StatusUnauthorized,
				"msg":  "user is not authorized",
			})
			return
		}

		c.Set("user_id", iUserId)
		c.Next()
	}
}

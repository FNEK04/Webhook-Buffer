package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func QueueSizeLimit(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("queue_max_size", maxSize)
		c.Next()
	}
}

func CheckQueueSize(getQueueSize func() (int64, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		maxSize, exists := c.Get("queue_max_size")
		if !exists {
			c.Next()
			return
		}

		size, err := getQueueSize()
		if err != nil {
			c.Next()
			return
		}

		if size >= maxSize.(int64) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":      "Queue is full",
				"queue_size": size,
				"max_size":   maxSize,
			})
			return
		}

		c.Header("X-Queue-Size", strconv.FormatInt(size, 10))
		c.Next()
	}
}

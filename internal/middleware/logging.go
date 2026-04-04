package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		requestID, _ := c.Get("request_id")

		log.Printf("[%s] %s %s %d %v %s %s",
			requestID, method, path, status, latency, clientIP, c.Errors.ByType(gin.ErrorTypePrivate).String())
	}
}

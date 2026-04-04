package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/pkg/config"
)

type rateBucket struct {
	limit    int
	window   time.Duration
	requests []time.Time
	mu       sync.Mutex
}

func (b *rateBucket) allow() (bool, int, time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-b.window)

	// Slide the window
	valid := b.requests[:0]
	for _, t := range b.requests {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	b.requests = valid

	remaining := b.limit - len(b.requests)
	if remaining <= 0 {
		resetAt := b.requests[0].Add(b.window)
		return false, 0, resetAt
	}

	b.requests = append(b.requests, now)
	remaining--

	resetAt := now.Add(b.window)
	return true, remaining, resetAt
}

type RateLimiter struct {
	cfg     config.RateLimitConfig
	buckets sync.Map // key -> *rateBucket
}

func NewRateLimiter(cfg config.RateLimitConfig) *RateLimiter {
	return &RateLimiter{cfg: cfg}
}

func (rl *RateLimiter) getBucket(key string, limit int, windowSecs int) *rateBucket {
	fullKey := key + ":" + strconv.Itoa(limit)
	actual, _ := rl.buckets.LoadOrStore(fullKey, &rateBucket{
		limit:  limit,
		window: time.Duration(windowSecs) * time.Second,
	})
	return actual.(*rateBucket)
}

func (rl *RateLimiter) RateLimit(bucketType string) gin.HandlerFunc {
	var limit, window int
	switch bucketType {
	case "read":
		limit = rl.cfg.ReadLimit
		window = rl.cfg.ReadWindow
	case "write":
		limit = rl.cfg.WriteLimit
		window = rl.cfg.WriteWindow
	case "download":
		limit = rl.cfg.DownloadLimit
		window = rl.cfg.DownloadWindow
	default:
		limit = rl.cfg.ReadLimit
		window = rl.cfg.ReadWindow
	}

	return func(c *gin.Context) {
		key := c.ClientIP()
		bucket := rl.getBucket(key+":"+bucketType, limit, window)

		allowed, remaining, resetAt := bucket.allow()

		c.Header("RateLimit-Limit", strconv.Itoa(limit))
		c.Header("RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

		if !allowed {
			retryAfter := time.Until(resetAt).Seconds()
			c.Header("Retry-After", fmt.Sprintf("%.0f", retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}

		c.Next()
	}
}

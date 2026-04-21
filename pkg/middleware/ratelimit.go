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

	// Slide the window — fresh slice to release old backing array
	valid := make([]time.Time, 0, len(b.requests))
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
	done    chan struct{}
}

func NewRateLimiter(cfg config.RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{cfg: cfg, done: make(chan struct{})}
	go rl.cleanup()
	return rl
}

// Close stops the cleanup goroutine.
func (rl *RateLimiter) Close() {
	close(rl.done)
}

// cleanup periodically evicts stale buckets to prevent unbounded memory growth.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			rl.buckets.Range(func(key, value any) bool {
				b := value.(*rateBucket)
				b.mu.Lock()
				if len(b.requests) == 0 || now.Sub(b.requests[len(b.requests)-1]) > b.window {
					b.mu.Unlock()
					rl.buckets.Delete(key)
				} else {
					b.mu.Unlock()
				}
				return true
			})
		case <-rl.done:
			return
		}
	}
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

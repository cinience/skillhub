package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/pkg/config"
	"github.com/cinience/skillhub/pkg/model"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequestID_Generated(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		reqID, _ := c.Get("request_id")
		c.String(200, reqID.(string))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	// Response should have X-Request-Id header
	rid := w.Header().Get(RequestIDHeader)
	if rid == "" {
		t.Error("expected X-Request-Id header to be set")
	}
	// Body should contain the request ID
	if w.Body.String() != rid {
		t.Errorf("body = %q, want %q", w.Body.String(), rid)
	}
}

func TestRequestID_Propagated(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		reqID, _ := c.Get("request_id")
		c.String(200, reqID.(string))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(RequestIDHeader, "my-custom-id")
	router.ServeHTTP(w, req)

	if w.Body.String() != "my-custom-id" {
		t.Errorf("body = %q, want %q", w.Body.String(), "my-custom-id")
	}
	if got := w.Header().Get(RequestIDHeader); got != "my-custom-id" {
		t.Errorf("header = %q, want %q", got, "my-custom-id")
	}
}

func TestGetUser_NoUser(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	u := GetUser(c)
	if u != nil {
		t.Errorf("GetUser() = %v, want nil", u)
	}
}

func TestGetUser_WithUser(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	user := &model.User{Handle: "testuser", Role: "user"}
	c.Set(UserContextKey, user)

	got := GetUser(c)
	if got == nil {
		t.Fatal("GetUser() = nil, want user")
	}
	if got.Handle != "testuser" {
		t.Errorf("GetUser().Handle = %q, want %q", got.Handle, "testuser")
	}
}

func TestRequireRole_Allowed(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(UserContextKey, &model.User{Handle: "admin", Role: "admin"})
		c.Next()
	})
	router.Use(RequireRole("admin"))
	router.GET("/admin", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRequireRole_Forbidden(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(UserContextKey, &model.User{Handle: "regular", Role: "user"})
		c.Next()
	})
	router.Use(RequireRole("admin"))
	router.GET("/admin", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireRole_NoUser(t *testing.T) {
	router := gin.New()
	router.Use(RequireRole("admin"))
	router.GET("/admin", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRequireRole_MultipleRoles(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(UserContextKey, &model.User{Handle: "mod", Role: "moderator"})
		c.Next()
	})
	router.Use(RequireRole("admin", "moderator"))
	router.GET("/mod", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/mod", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	rl := NewRateLimiter(config.RateLimitConfig{
		ReadLimit:  5,
		ReadWindow: 60,
	})

	router := gin.New()
	router.Use(rl.RateLimit("read"))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		router.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("request %d: status = %d, want 200", i, w.Code)
		}
		// Check rate limit headers
		if w.Header().Get("RateLimit-Limit") != "5" {
			t.Errorf("request %d: RateLimit-Limit = %q", i, w.Header().Get("RateLimit-Limit"))
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter(config.RateLimitConfig{
		ReadLimit:  2,
		ReadWindow: 60,
	})

	router := gin.New()
	router.Use(rl.RateLimit("read"))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	// First 2 should pass
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		router.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("request %d: status = %d, want 200", i, w.Code)
		}
	}

	// 3rd should be rate limited
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	router.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header")
	}
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	rl := NewRateLimiter(config.RateLimitConfig{
		ReadLimit:  1,
		ReadWindow: 60,
	})

	router := gin.New()
	router.Use(rl.RateLimit("read"))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	// IP 1 uses up its limit
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "1.1.1.1:1234"
	router.ServeHTTP(w1, req1)
	if w1.Code != 200 {
		t.Fatalf("IP1 first request: status = %d", w1.Code)
	}

	// IP 2 should still be allowed
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "2.2.2.2:1234"
	router.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Errorf("IP2 first request: status = %d, want 200", w2.Code)
	}
}

func TestLogging_DoesNotPanic(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.Use(Logging())
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

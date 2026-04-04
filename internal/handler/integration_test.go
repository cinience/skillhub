package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/internal/config"
	"github.com/cinience/skillhub/internal/model"
	"github.com/cinience/skillhub/internal/search"
)

// Integration tests: wire up a full gin router and test complete HTTP flows.

// setupIntegrationRouter creates a gin router with web + well-known routes using mocks.
func setupIntegrationRouter(svc WebSkillService, searchSvc WebSearchService) *gin.Engine {
	router := gin.New()

	// WellKnown
	cfg := &config.Config{Server: config.ServerConfig{BaseURL: "http://localhost:8080"}}
	wk := NewWellKnownHandler(cfg)
	router.GET("/.well-known/clawhub.json", wk.ClawHubJSON)

	// Web UI
	web := NewWebHandlerWithTemplate(svc, searchSvc, testWebTemplates())
	router.GET("/", web.Index)
	router.GET("/skills", web.Skills)
	router.GET("/skills/:slug", web.SkillDetail)
	router.GET("/search", web.Search)

	// Health
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return router
}

func TestIntegration_HealthEndpoint(t *testing.T) {
	router := setupIntegrationRouter(&mockSkillService{}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/healthz", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want 'ok'", resp["status"])
	}
}

func TestIntegration_WellKnown(t *testing.T) {
	router := setupIntegrationRouter(&mockSkillService{}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/.well-known/clawhub.json", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestIntegration_HomepageWithSkills(t *testing.T) {
	skill := testSkill()
	svc := &mockSkillService{
		listSkillsFn: func(_ context.Context, limit int, _, sort string) ([]model.SkillWithOwner, string, error) {
			return []model.SkillWithOwner{skill}, "", nil
		},
	}
	router := setupIntegrationRouter(svc, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(w.Body.String(), "test-skill") {
		t.Error("expected skill slug in homepage body")
	}
}

func TestIntegration_HomepageEmpty(t *testing.T) {
	router := setupIntegrationRouter(&mockSkillService{}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "INDEX") {
		t.Error("expected INDEX template content")
	}
}

func TestIntegration_SkillsListWithSort(t *testing.T) {
	sortOptions := []string{"created", "updated", "downloads", "stars", "name"}

	for _, sort := range sortOptions {
		t.Run("sort="+sort, func(t *testing.T) {
			var capturedSort string
			svc := &mockSkillService{
				listSkillsFn: func(_ context.Context, _ int, _, s string) ([]model.SkillWithOwner, string, error) {
					capturedSort = s
					return nil, "", nil
				},
			}
			router := setupIntegrationRouter(svc, nil)

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/skills?sort="+sort, nil)
			router.ServeHTTP(w, req)

			if w.Code != 200 {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			if capturedSort != sort {
				t.Errorf("captured sort = %q, want %q", capturedSort, sort)
			}
		})
	}
}

func TestIntegration_SkillsListPagination(t *testing.T) {
	callCount := 0
	svc := &mockSkillService{
		listSkillsFn: func(_ context.Context, _ int, cursor, _ string) ([]model.SkillWithOwner, string, error) {
			callCount++
			if callCount == 1 {
				if cursor != "" {
					t.Errorf("first call cursor = %q, want empty", cursor)
				}
				return []model.SkillWithOwner{testSkill()}, "cursor-abc", nil
			}
			if cursor != "cursor-abc" {
				t.Errorf("second call cursor = %q, want 'cursor-abc'", cursor)
			}
			return []model.SkillWithOwner{testSkill()}, "", nil
		},
	}
	router := setupIntegrationRouter(svc, nil)

	// First page
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/skills", nil)
	router.ServeHTTP(w1, req1)

	if w1.Code != 200 {
		t.Fatalf("page 1 status = %d", w1.Code)
	}
	if !strings.Contains(w1.Body.String(), "next=cursor-abc") {
		t.Error("expected next cursor in first page")
	}

	// Second page
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/skills?cursor=cursor-abc", nil)
	router.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Fatalf("page 2 status = %d", w2.Code)
	}
}

func TestIntegration_SkillDetailFound(t *testing.T) {
	skill := testSkill()
	version := testVersion()
	svc := &mockSkillService{
		getSkillFn: func(_ context.Context, slug string) (*model.SkillWithOwner, error) {
			if slug == "test-skill" {
				return &skill, nil
			}
			return nil, nil
		},
		getVersionsFn: func(_ context.Context, _ string) ([]model.SkillVersion, error) {
			return []model.SkillVersion{version}, nil
		},
		getFileFn: func(_ context.Context, _, _, _ string) ([]byte, error) {
			return []byte("# README"), nil
		},
	}
	router := setupIntegrationRouter(svc, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/skills/test-skill", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "test-skill") {
		t.Error("expected skill slug in detail page")
	}
}

func TestIntegration_SkillDetailNotFound(t *testing.T) {
	svc := &mockSkillService{
		getSkillFn: func(_ context.Context, _ string) (*model.SkillWithOwner, error) {
			return nil, nil
		},
	}
	router := setupIntegrationRouter(svc, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/skills/nonexistent", nil)
	router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestIntegration_SkillDetailError(t *testing.T) {
	svc := &mockSkillService{
		getSkillFn: func(_ context.Context, _ string) (*model.SkillWithOwner, error) {
			return nil, fmt.Errorf("database error")
		},
	}
	router := setupIntegrationRouter(svc, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/skills/broken", nil)
	router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404 (error treated as not found)", w.Code)
	}
}

func TestIntegration_SearchWithResults(t *testing.T) {
	searchSvc := &mockSearchService{
		searchFn: func(_ context.Context, query string, _, _ int, _ []string, _ string) (*search.SearchResult, error) {
			return &search.SearchResult{
				Hits: []map[string]interface{}{
					{"slug": "found-skill", "displayName": "Found Skill"},
				},
				Query:            query,
				ProcessingTimeMs: 3,
				EstimatedTotal:   1,
			}, nil
		},
	}
	router := setupIntegrationRouter(&mockSkillService{}, searchSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search?q=found", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "q=found") {
		t.Errorf("expected query in body, got %q", body)
	}
	if !strings.Contains(body, "hits=1") {
		t.Errorf("expected hits count in body, got %q", body)
	}
}

func TestIntegration_SearchNoQuery(t *testing.T) {
	router := setupIntegrationRouter(&mockSkillService{}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestIntegration_SearchError(t *testing.T) {
	searchSvc := &mockSearchService{
		searchFn: func(_ context.Context, _ string, _, _ int, _ []string, _ string) (*search.SearchResult, error) {
			return nil, fmt.Errorf("meilisearch down")
		},
	}
	router := setupIntegrationRouter(&mockSkillService{}, searchSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search?q=test", nil)
	router.ServeHTTP(w, req)

	// Should still return 200 with no results (graceful degradation)
	if w.Code != 200 {
		t.Errorf("status = %d, want 200 (graceful)", w.Code)
	}
}

func TestIntegration_404Route(t *testing.T) {
	router := setupIntegrationRouter(&mockSkillService{}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/nonexistent-path", nil)
	router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestIntegration_MethodNotAllowed(t *testing.T) {
	router := setupIntegrationRouter(&mockSkillService{}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 404 or 405", w.Code)
	}
}

func TestIntegration_ConcurrentRequests(t *testing.T) {
	skill := testSkill()
	svc := &mockSkillService{
		listSkillsFn: func(_ context.Context, _ int, _, _ string) ([]model.SkillWithOwner, string, error) {
			return []model.SkillWithOwner{skill}, "", nil
		},
		getSkillFn: func(_ context.Context, _ string) (*model.SkillWithOwner, error) {
			return &skill, nil
		},
		getVersionsFn: func(_ context.Context, _ string) ([]model.SkillVersion, error) {
			return nil, nil
		},
		getFileFn: func(_ context.Context, _, _, _ string) ([]byte, error) {
			return nil, nil
		},
	}
	router := setupIntegrationRouter(svc, nil)

	paths := []string{"/", "/skills", "/skills/test-skill", "/search", "/healthz"}
	done := make(chan bool, len(paths)*5)

	for i := 0; i < 5; i++ {
		for _, path := range paths {
			go func(p string) {
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", p, nil)
				router.ServeHTTP(w, req)
				if w.Code >= 500 {
					t.Errorf("GET %s returned %d", p, w.Code)
				}
				done <- true
			}(path)
		}
	}

	for i := 0; i < len(paths)*5; i++ {
		<-done
	}
}

func TestIntegration_ContentTypeHTML(t *testing.T) {
	router := setupIntegrationRouter(&mockSkillService{}, nil)

	htmlRoutes := []string{"/", "/skills", "/search"}
	for _, route := range htmlRoutes {
		t.Run(route, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", route, nil)
			router.ServeHTTP(w, req)

			ct := w.Header().Get("Content-Type")
			if !strings.Contains(ct, "text/html") {
				t.Errorf("GET %s Content-Type = %q, want text/html", route, ct)
			}
		})
	}
}

func TestIntegration_ContentTypeJSON(t *testing.T) {
	router := setupIntegrationRouter(&mockSkillService{}, nil)

	jsonRoutes := []string{"/healthz", "/.well-known/clawhub.json"}
	for _, route := range jsonRoutes {
		t.Run(route, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", route, nil)
			router.ServeHTTP(w, req)

			ct := w.Header().Get("Content-Type")
			if !strings.Contains(ct, "application/json") {
				t.Errorf("GET %s Content-Type = %q, want application/json", route, ct)
			}
		})
	}
}

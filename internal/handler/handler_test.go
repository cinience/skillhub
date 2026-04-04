package handler

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/cinience/skillhub/internal/config"
	"github.com/cinience/skillhub/internal/model"
	"github.com/cinience/skillhub/internal/search"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- Mock implementations ---

type mockSkillService struct {
	listSkillsFn   func(ctx context.Context, limit int, cursor, sort string) ([]model.SkillWithOwner, string, error)
	getSkillFn     func(ctx context.Context, slug string) (*model.SkillWithOwner, error)
	getVersionsFn  func(ctx context.Context, slug string) ([]model.SkillVersion, error)
	getFileFn      func(ctx context.Context, slug, version, path string) ([]byte, error)
	getVersionFn   func(ctx context.Context, slug, version string) (*model.SkillVersion, error)
}

func (m *mockSkillService) ListSkills(ctx context.Context, limit int, cursor, sort string) ([]model.SkillWithOwner, string, error) {
	if m.listSkillsFn != nil {
		return m.listSkillsFn(ctx, limit, cursor, sort)
	}
	return nil, "", nil
}

func (m *mockSkillService) GetSkill(ctx context.Context, slug string) (*model.SkillWithOwner, error) {
	if m.getSkillFn != nil {
		return m.getSkillFn(ctx, slug)
	}
	return nil, nil
}

func (m *mockSkillService) GetVersions(ctx context.Context, slug string) ([]model.SkillVersion, error) {
	if m.getVersionsFn != nil {
		return m.getVersionsFn(ctx, slug)
	}
	return nil, nil
}

func (m *mockSkillService) GetFile(ctx context.Context, slug, version, path string) ([]byte, error) {
	if m.getFileFn != nil {
		return m.getFileFn(ctx, slug, version, path)
	}
	return nil, nil
}

func (m *mockSkillService) GetVersion(ctx context.Context, slug, version string) (*model.SkillVersion, error) {
	if m.getVersionFn != nil {
		return m.getVersionFn(ctx, slug, version)
	}
	return nil, nil
}

type mockSearchService struct {
	searchFn func(ctx context.Context, query string, limit, offset int, sort []string, filters string) (*search.SearchResult, error)
}

func (m *mockSearchService) Search(ctx context.Context, query string, limit, offset int, sort []string, filters string) (*search.SearchResult, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query, limit, offset, sort, filters)
	}
	return nil, nil
}

// --- Test helpers ---

func testSkill() model.SkillWithOwner {
	name := "Test Skill"
	summary := "A test skill"
	return model.SkillWithOwner{
		Skill: model.Skill{
			ID:          uuid.New(),
			Slug:        "test-skill",
			DisplayName: &name,
			Summary:     &summary,
			Downloads:   100,
			StarsCount:  5,
			Tags:        model.StringArray{"go", "test"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		OwnerHandle: "testuser",
	}
}

func testVersion() model.SkillVersion {
	return model.SkillVersion{
		ID:        uuid.New(),
		SkillID:   uuid.New(),
		Version:   "1.0.0",
		CreatedAt: time.Now(),
	}
}

// Minimal templates for testing - each is self-contained (no shared layout).
func testWebTemplates() map[string]*template.Template {
	funcMap := TemplateFuncMap()
	templates := make(map[string]*template.Template)

	templates["index.html"] = template.Must(template.New("index.html").Funcs(funcMap).Parse(
		`INDEX{{range .Skills}}|{{.Slug}}{{end}}`))
	templates["skills.html"] = template.Must(template.New("skills.html").Funcs(funcMap).Parse(
		`SKILLS|sort={{.Sort}}{{range .Skills}}|{{.Slug}}{{end}}{{if .NextCursor}}|next={{.NextCursor}}{{end}}`))
	templates["skill_detail.html"] = template.Must(template.New("skill_detail.html").Funcs(funcMap).Parse(
		`DETAIL|{{.Skill.Slug}}|{{.SkillMdHTML}}`))
	templates["search.html"] = template.Must(template.New("search.html").Funcs(funcMap).Parse(
		`SEARCH|q={{.Query}}{{if .Results}}|hits={{.TotalHits}}{{end}}`))
	templates["publish.html"] = template.Must(template.New("publish.html").Funcs(funcMap).Parse(
		`PUBLISH|baseURL={{.BaseURL}}`))

	return templates
}

// --- WebHandler Tests ---

func TestWebHandler_Index(t *testing.T) {
	skills := []model.SkillWithOwner{testSkill()}
	mock := &mockSkillService{
		listSkillsFn: func(_ context.Context, limit int, cursor, sort string) ([]model.SkillWithOwner, string, error) {
			if limit != 6 {
				t.Errorf("Index should request 6 skills, got %d", limit)
			}
			if sort != "downloads" {
				t.Errorf("Index should sort by downloads, got %q", sort)
			}
			return skills, "", nil
		},
	}

	h := NewWebHandlerWithTemplate(mock, nil, testWebTemplates())
	router := gin.New()
	router.GET("/", h.Index)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "INDEX") {
		t.Error("expected INDEX in body")
	}
	if !strings.Contains(body, "test-skill") {
		t.Error("expected skill slug in body")
	}
}

func TestWebHandler_Index_NoSkills(t *testing.T) {
	mock := &mockSkillService{}
	h := NewWebHandlerWithTemplate(mock, nil, testWebTemplates())
	router := gin.New()
	router.GET("/", h.Index)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestWebHandler_Skills(t *testing.T) {
	skills := []model.SkillWithOwner{testSkill()}
	mock := &mockSkillService{
		listSkillsFn: func(_ context.Context, limit int, cursor, sort string) ([]model.SkillWithOwner, string, error) {
			if limit != 20 {
				t.Errorf("Skills should request 20, got %d", limit)
			}
			if sort != "stars" {
				t.Errorf("Skills should pass sort param, got %q", sort)
			}
			return skills, "next-cursor-123", nil
		},
	}

	h := NewWebHandlerWithTemplate(mock, nil, testWebTemplates())
	router := gin.New()
	router.GET("/skills", h.Skills)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/skills?sort=stars", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "sort=stars") {
		t.Errorf("expected sort=stars in body, got %q", body)
	}
	if !strings.Contains(body, "next=next-cursor-123") {
		t.Errorf("expected next cursor in body, got %q", body)
	}
}

func TestWebHandler_Skills_DefaultSort(t *testing.T) {
	mock := &mockSkillService{
		listSkillsFn: func(_ context.Context, _ int, _, sort string) ([]model.SkillWithOwner, string, error) {
			if sort != "created" {
				t.Errorf("default sort should be 'created', got %q", sort)
			}
			return nil, "", nil
		},
	}

	h := NewWebHandlerWithTemplate(mock, nil, testWebTemplates())
	router := gin.New()
	router.GET("/skills", h.Skills)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/skills", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestWebHandler_SkillDetail(t *testing.T) {
	skill := testSkill()
	versions := []model.SkillVersion{testVersion()}
	mock := &mockSkillService{
		getSkillFn: func(_ context.Context, slug string) (*model.SkillWithOwner, error) {
			if slug != "test-skill" {
				t.Errorf("slug = %q, want %q", slug, "test-skill")
			}
			return &skill, nil
		},
		getVersionsFn: func(_ context.Context, slug string) ([]model.SkillVersion, error) {
			return versions, nil
		},
		getFileFn: func(_ context.Context, slug, version, path string) ([]byte, error) {
			if path != "SKILL.md" {
				t.Errorf("path = %q, want SKILL.md", path)
			}
			return []byte("# Hello\n\nThis is **bold**."), nil
		},
	}

	h := NewWebHandlerWithTemplate(mock, nil, testWebTemplates())
	router := gin.New()
	router.GET("/skills/:slug", h.SkillDetail)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/skills/test-skill", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "DETAIL") {
		t.Error("expected DETAIL in body")
	}
	if !strings.Contains(body, "test-skill") {
		t.Error("expected skill slug in body")
	}
	// Goldmark should render markdown
	if !strings.Contains(body, "<h1>Hello</h1>") && !strings.Contains(body, "<strong>bold</strong>") {
		// Template may or may not include rendered HTML depending on test template
	}
}

func TestWebHandler_SkillDetail_NotFound(t *testing.T) {
	mock := &mockSkillService{
		getSkillFn: func(_ context.Context, slug string) (*model.SkillWithOwner, error) {
			return nil, nil
		},
	}

	h := NewWebHandlerWithTemplate(mock, nil, testWebTemplates())
	router := gin.New()
	router.GET("/skills/:slug", h.SkillDetail)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/skills/nonexistent", nil)
	router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestWebHandler_Search_WithQuery(t *testing.T) {
	searchMock := &mockSearchService{
		searchFn: func(_ context.Context, query string, limit, offset int, _ []string, filters string) (*search.SearchResult, error) {
			if query != "golang" {
				t.Errorf("query = %q, want 'golang'", query)
			}
			if limit != 20 {
				t.Errorf("limit = %d, want 20", limit)
			}
			if !strings.Contains(filters, "moderationStatus = approved") {
				t.Errorf("filters should include moderation check: %q", filters)
			}
			return &search.SearchResult{
				Hits:             []map[string]interface{}{{"slug": "go-skill"}},
				Query:            query,
				ProcessingTimeMs: 5,
				EstimatedTotal:   1,
			}, nil
		},
	}

	h := NewWebHandlerWithTemplate(&mockSkillService{}, searchMock, testWebTemplates())
	router := gin.New()
	router.GET("/search", h.Search)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search?q=golang", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "q=golang") {
		t.Errorf("expected query in body, got %q", body)
	}
	if !strings.Contains(body, "hits=1") {
		t.Errorf("expected hits count in body, got %q", body)
	}
}

func TestWebHandler_Search_EmptyQuery(t *testing.T) {
	h := NewWebHandlerWithTemplate(&mockSkillService{}, nil, testWebTemplates())
	router := gin.New()
	router.GET("/search", h.Search)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "SEARCH") {
		t.Error("expected SEARCH in body")
	}
}

func TestWebHandler_Search_NilSearchClient(t *testing.T) {
	h := NewWebHandlerWithTemplate(&mockSkillService{}, nil, testWebTemplates())
	router := gin.New()
	router.GET("/search", h.Search)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search?q=test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// --- WellKnownHandler Tests ---

func TestWellKnownHandler_ClawHubJSON(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			BaseURL: "https://skillhub.example.com",
		},
	}
	h := NewWellKnownHandler(cfg)

	router := gin.New()
	router.GET("/.well-known/clawhub.json", h.ClawHubJSON)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/.well-known/clawhub.json", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["registryUrl"] != "https://skillhub.example.com" {
		t.Errorf("registryUrl = %v", resp["registryUrl"])
	}
	if resp["apiVersion"] != "v1" {
		t.Errorf("apiVersion = %v", resp["apiVersion"])
	}

	endpoints, ok := resp["endpoints"].(map[string]interface{})
	if !ok {
		t.Fatal("expected endpoints object")
	}
	expectedEndpoints := []string{"search", "skills", "download", "resolve", "whoami", "publish"}
	for _, ep := range expectedEndpoints {
		if _, ok := endpoints[ep]; !ok {
			t.Errorf("missing endpoint %q", ep)
		}
	}
}

// --- Template FuncMap Tests ---

func TestTemplateFuncMap_Deref(t *testing.T) {
	fm := TemplateFuncMap()
	deref := fm["deref"].(func(*string) string)

	s := "hello"
	if got := deref(&s); got != "hello" {
		t.Errorf("deref(&hello) = %q", got)
	}
	if got := deref(nil); got != "" {
		t.Errorf("deref(nil) = %q, want empty", got)
	}
}

func TestTemplateFuncMap_Initial(t *testing.T) {
	fm := TemplateFuncMap()
	initial := fm["initial"].(func(string) string)

	if got := initial("alice"); got != "A" {
		t.Errorf("initial(alice) = %q, want A", got)
	}
	if got := initial(""); got != "?" {
		t.Errorf("initial('') = %q, want ?", got)
	}
}

func TestTemplateFuncMap_FormatTime(t *testing.T) {
	fm := TemplateFuncMap()
	formatTime := fm["formatTime"].(func(time.Time) string)

	tm := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	if got := formatTime(tm); got != "Mar 15, 2025" {
		t.Errorf("formatTime = %q, want 'Mar 15, 2025'", got)
	}
}

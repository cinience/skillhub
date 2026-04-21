package handler

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/pkg/middleware"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/search"
	"github.com/yuin/goldmark"
)

func normalizeTemplateWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func templateString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case *string:
		if v == nil {
			return ""
		}
		return *v
	default:
		return fmt.Sprint(v)
	}
}

func formatTemplateDisplayName(raw any, fallback any, maxLength int) (string, string) {
	source := normalizeTemplateWhitespace(templateString(raw))
	if source == "" {
		source = normalizeTemplateWhitespace(templateString(fallback))
	}
	runes := []rune(source)
	if len(runes) <= maxLength {
		return source, ""
	}
	clipped := strings.TrimRight(string(runes[:maxLength-1]), " ")
	return clipped + "…", source
}

// WebSkillService defines the skill operations needed by the web UI.
type WebSkillService interface {
	ListSkills(ctx context.Context, limit int, cursor, sort, category string, viewer *model.User) ([]model.SkillWithOwner, string, error)
	GetSkill(ctx context.Context, slug string, viewer *model.User) (*model.SkillWithOwner, error)
	GetVersions(ctx context.Context, slug string, viewer *model.User) ([]model.SkillVersion, error)
	GetFile(ctx context.Context, slug, version, path string, viewer *model.User) ([]byte, error)
}

// WebSearchService defines the search operations needed by the web UI.
type WebSearchService interface {
	Search(ctx context.Context, query string, limit, offset int, sort []string, filters string) (*search.SearchResult, error)
}

type WebHandler struct {
	svc       WebSkillService
	searchCli WebSearchService
	templates map[string]*template.Template
	baseURL   string
}

// TemplateFuncMap returns the template function map used by the web handler.
func TemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"deref": func(s *string) string {
			if s == nil {
				return ""
			}
			return *s
		},
		"initial": func(s string) string {
			if len(s) == 0 {
				return "?"
			}
			return strings.ToUpper(s[:1])
		},
		"formatTime": func(t time.Time) string {
			return t.Format("Jan 2, 2006")
		},
		"displayNameText": func(raw any, fallback any, maxLength int) string {
			text, _ := formatTemplateDisplayName(raw, fallback, maxLength)
			return text
		},
		"displayNameTooltip": func(raw any, fallback any, maxLength int) string {
			_, tooltip := formatTemplateDisplayName(raw, fallback, maxLength)
			return tooltip
		},
	}
}

func NewWebHandler(svc WebSkillService, searchCli WebSearchService, templateDir string, baseURL string) *WebHandler {
	funcMap := TemplateFuncMap()
	layoutFile := filepath.Join(templateDir, "layout.html")

	pages := []string{"index.html", "skills.html", "skill_detail.html", "search.html", "publish.html", "login.html"}
	templates := make(map[string]*template.Template, len(pages))

	for _, page := range pages {
		pageFile := filepath.Join(templateDir, page)
		tmpl := template.Must(
			template.New("").Funcs(funcMap).ParseFiles(layoutFile, pageFile),
		)
		templates[page] = tmpl
	}

	return &WebHandler{
		svc:       svc,
		searchCli: searchCli,
		templates: templates,
		baseURL:   baseURL,
	}
}

// NewWebHandlerWithTemplate creates a WebHandler with pre-parsed templates (for testing).
func NewWebHandlerWithTemplate(svc WebSkillService, searchCli WebSearchService, templates map[string]*template.Template) *WebHandler {
	return &WebHandler{
		svc:       svc,
		searchCli: searchCli,
		templates: templates,
	}
}

func (h *WebHandler) render(c *gin.Context, name string, data gin.H) {
	tmpl, ok := h.templates[name]
	if !ok {
		log.Printf("template not found: %s", name)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	// Inject current user into all templates
	if user := middleware.GetUser(c); user != nil {
		data["CurrentUser"] = user
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, name, data); err != nil {
		log.Printf("template error: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
	}
}

// Index renders the homepage with popular skills.
func (h *WebHandler) Index(c *gin.Context) {
	ctx := c.Request.Context()
	viewer := middleware.GetUser(c)
	skills, _, _ := h.svc.ListSkills(ctx, 6, "", "downloads", "", viewer)

	h.render(c, "index.html", gin.H{
		"Title":  "",
		"Skills": skills,
	})
}

// Skills renders the paginated skills list.
func (h *WebHandler) Skills(c *gin.Context) {
	ctx := c.Request.Context()
	sort := c.DefaultQuery("sort", "created")
	cursor := c.Query("cursor")
	viewer := middleware.GetUser(c)

	category := c.Query("category")
	skills, nextCursor, _ := h.svc.ListSkills(ctx, 20, cursor, sort, category, viewer)

	h.render(c, "skills.html", gin.H{
		"Title":      "Skills",
		"Skills":     skills,
		"Sort":       sort,
		"NextCursor": nextCursor,
	})
}

// SkillDetail renders the detail page for a single skill.
func (h *WebHandler) SkillDetail(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")
	viewer := middleware.GetUser(c)

	skill, err := h.svc.GetSkill(ctx, slug, viewer)
	if err != nil || skill == nil {
		c.String(http.StatusNotFound, "Skill not found")
		return
	}

	// Get versions
	versions, _ := h.svc.GetVersions(ctx, slug, viewer)

	// Get latest version
	var latestVersion interface{}
	if len(versions) > 0 {
		latestVersion = versions[0]
	}

	// Render SKILL.md as HTML
	var skillMdHTML template.HTML
	content, err := h.svc.GetFile(ctx, slug, "latest", "SKILL.md", viewer)
	if err == nil && len(content) > 0 {
		content = stripFrontmatter(content)
		var buf bytes.Buffer
		if err := goldmark.Convert(content, &buf); err == nil {
			skillMdHTML = template.HTML(buf.String())
		}
	}

	h.render(c, "skill_detail.html", gin.H{
		"Title":         skill.Slug,
		"Skill":         skill,
		"Versions":      versions,
		"LatestVersion": latestVersion,
		"SkillMdHTML":   skillMdHTML,
	})
}

// Search renders the search page with results.
func (h *WebHandler) Search(c *gin.Context) {
	query := c.Query("q")

	data := gin.H{
		"Title":   "Search",
		"Query":   query,
		"Results": nil,
	}

	if query != "" && h.searchCli != nil {
		ctx := c.Request.Context()
		filters := "visibility = public AND moderationStatus = approved AND isDeleted = false"
		result, err := h.searchCli.Search(ctx, query, 20, 0, nil, filters)
		if err == nil && result != nil {
			data["Results"] = result.Hits
			data["TotalHits"] = result.EstimatedTotal
			data["ProcessingTimeMs"] = result.ProcessingTimeMs
		}
	}

	h.render(c, "search.html", data)
}

// Publish renders the publish guide page.
func (h *WebHandler) Publish(c *gin.Context) {
	baseURL := h.baseURL
	if baseURL == "" {
		baseURL = "http://" + c.Request.Host
	}
	h.render(c, "publish.html", gin.H{
		"Title":   "Publish",
		"BaseURL": baseURL,
	})
}

// stripFrontmatter removes YAML frontmatter (--- delimited) from markdown content.
func stripFrontmatter(data []byte) []byte {
	s := string(data)
	if !strings.HasPrefix(s, "---") {
		return data
	}
	// Find the closing ---
	end := strings.Index(s[3:], "\n---")
	if end < 0 {
		return data
	}
	// Skip past closing --- and the newline after it
	rest := s[3+end+4:]
	return []byte(rest)
}

// LoginPage renders the login form (GET /login).
func (h *WebHandler) LoginPage(c *gin.Context) {
	// If already logged in, redirect to home
	if user := middleware.GetUser(c); user != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}
	h.render(c, "login.html", gin.H{
		"Title": "Login",
	})
}


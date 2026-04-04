package server

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/cinience/skillhub/internal/auth"
	"github.com/cinience/skillhub/internal/config"
	"github.com/cinience/skillhub/internal/gitstore"
	"github.com/cinience/skillhub/internal/handler"
	"github.com/cinience/skillhub/internal/middleware"
	"github.com/cinience/skillhub/internal/model"
	"github.com/cinience/skillhub/internal/repository"
	"github.com/cinience/skillhub/internal/search"
	"github.com/cinience/skillhub/internal/service"
	"github.com/cinience/skillhub/web"
	"gorm.io/gorm"
)

type Server struct {
	router     *gin.Engine
	cfg        *config.Config
	httpServer *http.Server
}

func New(cfg *config.Config) (*Server, error) {
	// Database
	db, err := repository.NewDB(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	// Auto-setup: create admin user from env vars if not exists
	autoSetup(db)

	// Repositories
	userRepo := repository.NewUserRepo(db)
	skillRepo := repository.NewSkillRepo(db)
	versionRepo := repository.NewVersionRepo(db)
	tokenRepo := repository.NewTokenRepo(db)
	starRepo := repository.NewStarRepo(db)
	downloadRepo := repository.NewDownloadRepo(db)

	// Git Store
	gs, err := gitstore.New(cfg.GitStore.BasePath)
	if err != nil {
		return nil, fmt.Errorf("gitstore: %w", err)
	}

	// Mirror Service
	mirrorSvc := gitstore.NewMirrorService(gs, cfg.GitStore.Mirror)

	// Import Service
	importSvc := gitstore.NewImportService(gs, cfg.GitStore.Import)

	// Search (Bleve - embedded, always available)
	searchClient, err := search.New(cfg.Search)
	if err != nil {
		log.Printf("warning: search index unavailable: %v (search will be disabled)", err)
		searchClient = nil
	}

	// Auth
	authSvc := auth.NewService(tokenRepo, userRepo)

	// Service
	skillSvc := service.NewSkillService(skillRepo, versionRepo, userRepo, downloadRepo, starRepo, gs, searchClient, mirrorSvc)

	// Rate Limiter
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimit)

	// Handlers
	skillHandler := handler.NewSkillHandler(skillSvc)
	searchHandler := handler.NewSearchHandler(searchClient)
	downloadHandler := handler.NewDownloadHandler(skillSvc)
	authHandler := handler.NewAuthHandler(authSvc, userRepo)
	starHandler := handler.NewStarHandler(skillSvc)
	adminHandler := handler.NewAdminHandler(userRepo)
	webhookHandler := handler.NewWebhookHandler(importSvc)
	wellKnownHandler := handler.NewWellKnownHandler(cfg)

	// Web auth handler (login/logout still server-side for cookie management)
	webAuthHandler := handler.NewWebAuthHandler(authSvc)

	// Router
	router := gin.Default()
	router.Use(middleware.RequestID())
	router.Use(middleware.Logging())
	router.Use(middleware.SecurityHeaders())

	// Well-known
	router.GET("/.well-known/clawhub.json", wellKnownHandler.ClawHubJSON)

	// Login/logout routes (server-side cookie management)
	webRoutes := router.Group("")
	webRoutes.Use(middleware.WebOptionalAuth(authSvc))
	{
		webRoutes.POST("/login", rateLimiter.RateLimit("write"), webAuthHandler.LoginSubmit)
		webRoutes.POST("/logout", webAuthHandler.Logout)
	}

	// Serve embedded frontend static assets
	staticFS, _ := fs.Sub(web.StaticFS, "static/assets")
	router.StaticFS("/assets", http.FS(staticFS))

	// Health
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(503, gin.H{"status": "not ready"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	// API v1
	api := router.Group("/api/v1")

	// Public endpoints (with read rate limit)
	public := api.Group("")
	public.Use(rateLimiter.RateLimit("read"))
	public.Use(middleware.OptionalAuth(authSvc))
	{
		public.GET("/search", searchHandler.Search)
		public.GET("/skills", skillHandler.List)
		public.GET("/skills/:slug", skillHandler.Get)
		public.GET("/skills/:slug/versions", skillHandler.Versions)
		public.GET("/skills/:slug/versions/:version", skillHandler.Version)
		public.GET("/skills/:slug/file", skillHandler.GetFile)
		public.GET("/resolve", downloadHandler.Resolve)
	}

	// Download endpoint (with download rate limit)
	download := api.Group("")
	download.Use(rateLimiter.RateLimit("download"))
	{
		download.GET("/download", downloadHandler.Download)
	}

	// Authenticated endpoints (with write rate limit)
	authed := api.Group("")
	authed.Use(middleware.RequireAuth(authSvc))
	authed.Use(rateLimiter.RateLimit("write"))
	{
		authed.GET("/whoami", authHandler.WhoAmI)
		authed.POST("/skills", skillHandler.Publish)
		authed.DELETE("/skills/:slug", skillHandler.Delete)
		authed.POST("/skills/:slug/undelete", skillHandler.Undelete)
		authed.POST("/stars/:slug", starHandler.Star)
		authed.DELETE("/stars/:slug", starHandler.Unstar)
	}

	// Admin endpoints
	admin := api.Group("")
	admin.Use(middleware.RequireAuth(authSvc))
	admin.Use(middleware.RequireRole("admin"))
	{
		admin.POST("/users/ban", adminHandler.BanUser)
		admin.POST("/users/role", adminHandler.SetRole)
		admin.GET("/users", adminHandler.ListUsers)
		admin.POST("/users", adminHandler.CreateUser)
		admin.POST("/tokens", authHandler.CreateToken)
	}

	// Webhook endpoints
	webhooks := api.Group("/webhooks")
	{
		webhooks.POST("/github", webhookHandler.GitHubWebhook)
		webhooks.POST("/gitlab", webhookHandler.GitLabWebhook)
		webhooks.POST("/gitea", webhookHandler.GiteaWebhook)
	}

	// SPA fallback: serve index.html for all non-API routes
	indexHTML, _ := web.StaticFS.ReadFile("static/index.html")
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.Data(200, "text/html; charset=utf-8", indexHTML)
	})

	// Mirror push on startup if configured
	if mirrorSvc.Enabled() && cfg.GitStore.Mirror.PushOnStartup {
		go mirrorSvc.PushAll(nil)
	}

	return &Server{router: router, cfg: cfg}, nil
}

func (s *Server) Run() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// autoSetup creates an admin user on first startup if SKILLHUB_ADMIN_USER
// and SKILLHUB_ADMIN_PASSWORD environment variables are set.
func autoSetup(db *gorm.DB) {
	handle := os.Getenv("SKILLHUB_ADMIN_USER")
	password := os.Getenv("SKILLHUB_ADMIN_PASSWORD")
	if handle == "" || password == "" {
		return
	}

	ctx := context.Background()
	userRepo := repository.NewUserRepo(db)

	// Check if user already exists
	existing, _ := userRepo.GetByHandle(ctx, handle)
	if existing != nil {
		return
	}

	// Create admin user
	user := &model.User{
		ID:     uuid.New(),
		Handle: handle,
		Role:   "admin",
	}
	if err := userRepo.Create(ctx, user); err != nil {
		log.Printf("auto-setup: failed to create admin user: %v", err)
		return
	}

	// Set password
	tokenRepo := repository.NewTokenRepo(db)
	authSvc := auth.NewService(tokenRepo, userRepo)
	if err := authSvc.SetPassword(ctx, user.ID, password); err != nil {
		log.Printf("auto-setup: failed to set admin password: %v", err)
		return
	}

	log.Printf("auto-setup: created admin user %q", handle)
}

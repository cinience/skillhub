package server

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/auth"
	"github.com/cinience/skillhub/pkg/config"
	"github.com/cinience/skillhub/pkg/gitstore"
	"github.com/cinience/skillhub/pkg/handler"
	"github.com/cinience/skillhub/pkg/middleware"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/repository"
	"github.com/cinience/skillhub/pkg/search"
	"github.com/cinience/skillhub/pkg/service"
	"github.com/cinience/skillhub/pkg/store"
	"github.com/cinience/skillhub/web"
	"gorm.io/gorm"
)

type Server struct {
	router       *gin.Engine
	cfg          *config.Config
	httpServer   *http.Server
	db           *gorm.DB
	searchClient *search.Client
	oauthSvc     *auth.OAuthService
	deviceSvc    *auth.DeviceAuthService
	rateLimiter  *middleware.RateLimiter
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

	// Git Store (always created — Mirror and Import depend on it)
	gs, err := gitstore.New(cfg.GitStore.BasePath)
	if err != nil {
		return nil, fmt.Errorf("gitstore: %w", err)
	}

	// File Store backend (git / s3 / oss)
	var fileStore store.Store
	switch cfg.Store.Backend {
	case "s3":
		fileStore, err = store.NewS3Backend(store.S3Config{
			Bucket:    cfg.Store.S3.Bucket,
			Region:    cfg.Store.S3.Region,
			Prefix:    cfg.Store.S3.Prefix,
			Endpoint:  cfg.Store.S3.Endpoint,
			AccessKey: cfg.Store.S3.AccessKey,
			SecretKey: cfg.Store.S3.SecretKey,
		})
		if err != nil {
			return nil, fmt.Errorf("s3 store: %w", err)
		}
	case "oss":
		fileStore, err = store.NewOSSBackend(store.OSSConfig{
			Bucket:    cfg.Store.OSS.Bucket,
			Region:    cfg.Store.OSS.Region,
			Prefix:    cfg.Store.OSS.Prefix,
			Endpoint:  cfg.Store.OSS.Endpoint,
			AccessKey: cfg.Store.OSS.AccessKey,
			SecretKey: cfg.Store.OSS.SecretKey,
		})
		if err != nil {
			return nil, fmt.Errorf("oss store: %w", err)
		}
	default: // "git" or ""
		fileStore = store.NewGitBackend(gs)
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

	// OAuth
	oauthRepo := repository.NewOAuthRepo(db)
	oauthSvc := auth.NewOAuthService(cfg.Auth.OAuth, oauthRepo, userRepo, authSvc, cfg.Server.BaseURL)

	// Device Auth
	deviceSvc := auth.NewDeviceAuthService(authSvc, cfg.Server.BaseURL)

	// Audit
	auditRepo := repository.NewAuditRepo(db)
	auditSvc := service.NewAuditService(auditRepo)

	// Service
	skillSvc := service.NewSkillService(db, skillRepo, versionRepo, userRepo, downloadRepo, starRepo, fileStore, searchClient, mirrorSvc, auditSvc)

	// Namespace
	nsRepo := repository.NewNamespaceRepo(db)
	nsSvc := service.NewNamespaceService(nsRepo, userRepo)

	// Notifications
	notifRepo := repository.NewNotificationRepo(db)
	notifSvc := service.NewNotificationService(notifRepo)

	// Ratings
	ratingRepo := repository.NewRatingRepo(db)
	ratingSvc := service.NewRatingService(ratingRepo, skillRepo)

	// Rate Limiter
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimit)

	// Handlers
	skillHandler := handler.NewSkillHandler(skillSvc)
	searchHandler := handler.NewSearchHandler(searchClient)
	downloadHandler := handler.NewDownloadHandler(skillSvc)
	authHandler := handler.NewAuthHandler(authSvc, userRepo)
	starHandler := handler.NewStarHandler(skillSvc)
	adminHandler := handler.NewAdminHandler(userRepo, skillSvc, auditSvc)
	auditHandler := handler.NewAuditHandler(auditSvc)
	tokenHandler := handler.NewTokenHandler(authSvc, tokenRepo)
	nsHandler := handler.NewNamespaceHandler(nsSvc)
	notifHandler := handler.NewNotificationHandler(notifSvc)
	ratingHandler := handler.NewRatingHandler(ratingSvc)
	oauthHandler := handler.NewOAuthHandler(oauthSvc)
	deviceHandler := handler.NewDeviceAuthHandler(deviceSvc)
	webhookHandler := handler.NewWebhookHandler(importSvc)
	wellKnownHandler := handler.NewWellKnownHandler(cfg)
	agentHandler := handler.NewAgentHandler(authSvc, userRepo)

	// Web auth handler (login/logout still server-side for cookie management)
	webAuthHandler := handler.NewWebAuthHandler(authSvc)

	// Router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.Logging())
	router.Use(middleware.SecurityHeaders())

	// Well-known
	router.GET("/.well-known/clawhub.json", wellKnownHandler.ClawHubJSON)

	// OAuth routes
	router.GET("/auth/:provider", oauthHandler.Redirect)
	router.GET("/auth/:provider/callback", oauthHandler.Callback)

	// Device auth routes (rate-limited to prevent DoS)
	deviceRoutes := router.Group("/auth/device")
	deviceRoutes.Use(rateLimiter.RateLimit("write"))
	{
		deviceRoutes.POST("/code", deviceHandler.RequestCode)
		deviceRoutes.GET("/verify", middleware.OptionalAuth(authSvc), deviceHandler.VerifyPage)
		deviceRoutes.POST("/verify", middleware.RequireAuth(authSvc), deviceHandler.VerifySubmit)
		deviceRoutes.POST("/token", deviceHandler.PollToken)
	}

	// API v1 device auth aliases — animus CLI & other API clients expect /api/v1 prefix.
	// Same handlers, different mount point.
	apiDeviceRoutes := router.Group("/api/v1/auth/device")
	apiDeviceRoutes.Use(rateLimiter.RateLimit("write"))
	{
		apiDeviceRoutes.POST("/code", deviceHandler.RequestCode)
		apiDeviceRoutes.POST("/verify", middleware.RequireAuth(authSvc), deviceHandler.VerifySubmit)
		apiDeviceRoutes.POST("/token", deviceHandler.PollToken)
	}

	// Login/logout routes (server-side cookie management)
	webRoutes := router.Group("")
	webRoutes.Use(middleware.OptionalAuth(authSvc))
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
		public.GET("/skills/:slug/ratings", ratingHandler.List)
	}

	// Download endpoint (with download rate limit)
	download := api.Group("")
	download.Use(rateLimiter.RateLimit("download"))
	download.Use(middleware.OptionalAuth(authSvc))
	{
		download.GET("/download", downloadHandler.Download)
	}

	// Authenticated endpoints (with write rate limit and scope enforcement)
	authed := api.Group("")
	authed.Use(middleware.RequireAuth(authSvc))
	authed.Use(middleware.RequireScope())
	authed.Use(rateLimiter.RateLimit("write"))
	{
		authed.GET("/whoami", authHandler.WhoAmI)
		authed.POST("/skills", skillHandler.Publish)
		authed.DELETE("/skills/:slug", skillHandler.Delete)
		authed.POST("/skills/:slug/undelete", skillHandler.Undelete)
		authed.POST("/skills/:slug/request-public", skillHandler.RequestPublic)
		authed.POST("/stars/:slug", starHandler.Star)
		authed.DELETE("/stars/:slug", starHandler.Unstar)
		authed.GET("/tokens", tokenHandler.List)
		authed.POST("/tokens", tokenHandler.Create)
		authed.DELETE("/tokens/:id", tokenHandler.Revoke)
		authed.POST("/namespaces", nsHandler.Create)
		authed.GET("/namespaces", nsHandler.List)
		authed.GET("/namespaces/:slug", nsHandler.Get)
		authed.PUT("/namespaces/:slug", nsHandler.Update)
		authed.DELETE("/namespaces/:slug", nsHandler.Delete)
		authed.GET("/namespaces/:slug/members", nsHandler.ListMembers)
		authed.POST("/namespaces/:slug/members", nsHandler.AddMember)
		authed.DELETE("/namespaces/:slug/members/:handle", nsHandler.RemoveMember)
		authed.GET("/notifications", notifHandler.List)
		authed.GET("/notifications/unread", notifHandler.Unread)
		authed.POST("/notifications/:id/read", notifHandler.MarkRead)
		authed.POST("/notifications/read-all", notifHandler.MarkAllRead)
		authed.POST("/skills/:slug/ratings", ratingHandler.Rate)
		authed.DELETE("/skills/:slug/ratings", ratingHandler.Delete)
	}

	// Admin endpoints
	admin := api.Group("/admin")
	admin.Use(middleware.RequireAuth(authSvc))
	admin.Use(middleware.RequireRole("admin"))
	{
		admin.POST("/users/ban", adminHandler.BanUser)
		admin.POST("/users/role", adminHandler.SetRole)
		admin.GET("/users", adminHandler.ListUsers)
		admin.POST("/users", adminHandler.CreateUser)
		admin.POST("/tokens", authHandler.CreateToken)
		admin.GET("/skills", adminHandler.ListAllSkills)
		admin.POST("/skills/:slug/review", adminHandler.ReviewSkill)
		admin.POST("/skills/:slug/visibility", adminHandler.SetVisibility)
		admin.GET("/audit-logs", auditHandler.List)
	}

	// Agent auto-provisioning (rate-limited, requires SKILLHUB_AGENT_SECRET)
	api.POST("/agent/provision", rateLimiter.RateLimit("write"), agentHandler.Provision)

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
		go func() {
			if err := mirrorSvc.PushAll(context.Background()); err != nil {
				log.Printf("mirror push on startup failed: %v", err)
			}
		}()
	}

	return &Server{
		router:       router,
		cfg:          cfg,
		db:           db,
		searchClient: searchClient,
		oauthSvc:     oauthSvc,
		deviceSvc:    deviceSvc,
		rateLimiter:  rateLimiter,
	}, nil
}

func (s *Server) Run() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	var firstErr error
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			firstErr = err
		}
	}
	if s.deviceSvc != nil {
		s.deviceSvc.Close()
	}
	if s.oauthSvc != nil {
		s.oauthSvc.Close()
	}
	if s.rateLimiter != nil {
		s.rateLimiter.Close()
	}
	if s.searchClient != nil {
		if err := s.searchClient.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.db != nil {
		if sqlDB, err := s.db.DB(); err == nil {
			if err := sqlDB.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
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

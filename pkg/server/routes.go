package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/saker-ai/skillhub/pkg/middleware"
	"github.com/saker-ai/skillhub/web"
)

// RegisterRoutes 把 SkillHub 的全部 HTTP API 挂到给定 router 上。
//
// 适用场景：
//
//	standalone — Run() 内部自己创建 gin.Engine 后调用本方法（兼容历史行为）。
//	embedded   — 嵌入方传入自己已有的 gin.Engine 或 RouterGroup，
//	             把 SkillHub 路由挂到任意子路径下。
//
// 路由列表（路径与历史行为完全一致）：
//
//	GET  /.well-known/clawhub.json
//	GET  /auth/:provider                          OAuth login redirect
//	GET  /auth/:provider/callback                 OAuth callback
//	*    /auth/device/{code,verify,token}         device-flow login (rate limited)
//	*    /api/v1/auth/device/{code,verify,token}  CLI device-flow alias
//	POST /login, /logout                          web cookie session
//	GET  /metrics                                 prometheus scrape
//	GET  /healthz, /readyz                        liveness / readiness
//	GET  /api/v1/...                              public + download + authed + admin
//	GET  /api/runtime/...                         Runtime skill loading API
//	POST /api/agent/skills                        Agent skill publish API
//	POST /api/v1/agent/provision                  agent auto-provision
//	POST /api/v1/webhooks/{github,gitlab,gitea}   import webhooks
//
// 不包含静态资源与 SPA 兜底——那部分由 RegisterStatic 单独负责，
// 因为它需要 *gin.Engine（router.NoRoute 不在 IRouter 接口上）。
//
// 嵌入方如果不希望某些路由出现，可以选择不调用本方法、改为只手动挂自己关心的子集
// （目前 SkillHub 暂不提供细粒度路由开关——按 KISS 原则等真正出现需求再加）。
func (s *Server) RegisterRoutes(r gin.IRouter) {
	s.registerHumaDocs(r)

	// Well-known
	r.GET("/.well-known/clawhub.json", s.h.wellKnown.ClawHubJSON)

	// Agent-readable install/operations guide.
	// Discoverable via "Read https://<host>/skills.md and follow the instructions".
	r.GET("/skills.md", s.h.wellKnown.InstallMarkdown)

	// OAuth routes
	r.GET("/auth/:provider", s.h.oauth.Redirect)
	r.GET("/auth/:provider/callback", s.h.oauth.Callback)

	// Device auth routes (rate-limited to prevent DoS)
	deviceRoutes := r.Group("/auth/device")
	deviceRoutes.Use(s.rateLimiter.RateLimit("write"))
	{
		deviceRoutes.POST("/code", s.h.device.RequestCode)
		deviceRoutes.GET("/verify", middleware.OptionalAuth(s.idp), s.h.device.VerifyPage)
		deviceRoutes.POST("/verify", middleware.RequireAuth(s.idp), s.h.device.VerifySubmit)
		deviceRoutes.POST("/token", s.h.device.PollToken)
	}

	// API v1 device auth aliases — animus CLI & other API clients expect /api/v1 prefix.
	// Same handlers, different mount point.
	apiDeviceRoutes := r.Group("/api/v1/auth/device")
	apiDeviceRoutes.Use(s.rateLimiter.RateLimit("write"))
	{
		apiDeviceRoutes.POST("/code", s.h.device.RequestCode)
		apiDeviceRoutes.POST("/verify", middleware.RequireAuth(s.idp), s.h.device.VerifySubmit)
		apiDeviceRoutes.POST("/token", s.h.device.PollToken)
	}

	// Login/logout routes (server-side cookie management)
	webRoutes := r.Group("")
	webRoutes.Use(middleware.OptionalAuth(s.idp))
	{
		webRoutes.POST("/login", s.rateLimiter.RateLimit("write"), s.h.webAuth.LoginSubmit)
		webRoutes.POST("/logout", s.h.webAuth.Logout)
	}

	// Prometheus scrape endpoint. Unauthenticated: in production this
	// should sit behind network ACLs (private subnet / sidecar proxy)
	// rather than HTTP auth — same convention as kube-state-metrics.
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := s.db.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(503, gin.H{"status": "not ready"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	// API v1
	api := r.Group("/api/v1")

	runtimeSkills := r.Group("/api/runtime")
	runtimeSkills.Use(s.rateLimiter.RateLimit("read"))
	runtimeSkills.Use(middleware.OptionalAuth(s.idp))
	{
		runtimeSkills.GET("/capabilities", s.h.runtime.Capabilities)
		runtimeSkills.POST("/skills/resolve", s.h.runtime.Resolve)
		runtimeSkills.GET("/skills", s.h.runtime.List)
		runtimeSkills.GET("/skills/by-id/:id", s.h.runtime.ByID)
		runtimeSkills.GET("/skills/by-slug", s.h.runtime.BySlug)
		runtimeSkills.GET("/skills/files", s.h.runtime.File)
		runtimeSkills.GET("/skills/bundle", s.h.runtime.Bundle)
	}

	// Public endpoints (with read rate limit)
	public := api.Group("")
	public.Use(s.rateLimiter.RateLimit("read"))
	public.Use(middleware.OptionalAuth(s.idp))
	{
		public.GET("/search", s.h.search.Search)
		public.GET("/categories", s.h.skill.Categories)
		public.GET("/skills", s.h.skill.List)
		deprecate := middleware.DeprecateRoute("2026-12-31")
		public.GET("/skills/:slug", deprecate, s.h.skill.Get)
		public.GET("/skills/:slug/versions", deprecate, s.h.skill.Versions)
		public.GET("/skills/:slug/versions/:version", deprecate, s.h.skill.Version)
		public.GET("/skills/:slug/file", deprecate, s.h.skill.GetFile)
		public.GET("/resolve", s.h.download.Resolve)
		public.GET("/skills/:slug/ratings", s.h.rating.List)
		public.GET("/skills/:slug/comments", s.h.comment.List)

		// Namespace-qualified skill routes (public read)
		public.GET("/skills/@:namespace/:slug", s.h.skill.Get)
		public.GET("/skills/@:namespace/:slug/versions", s.h.skill.Versions)
		public.GET("/skills/@:namespace/:slug/versions/:version", s.h.skill.Version)
		public.GET("/skills/@:namespace/:slug/file", s.h.skill.GetFile)
		public.GET("/skills/@:namespace/:slug/ratings", s.h.rating.List)
		public.GET("/skills/@:namespace/:slug/comments", s.h.comment.List)

		// List skills belonging to a namespace (public read)
		public.GET("/namespaces/:slug/skills", s.h.skill.ListByNamespace)

		// Plugin endpoints (public read)
		public.GET("/plugins", s.h.plugin.List)
		public.GET("/plugins/by-id/:id", s.h.plugin.GetByID)
		public.GET("/plugins/:slug", s.h.plugin.Get)
		public.GET("/plugins/:slug/versions", s.h.plugin.Versions)
		public.GET("/plugins/file", s.h.plugin.GetFile)
		public.GET("/plugins/download", s.h.plugin.Download)

		// Namespace-qualified plugin routes (public read)
		public.GET("/plugins/@:namespace/:slug", s.h.plugin.Get)
		public.GET("/plugins/@:namespace/:slug/versions", s.h.plugin.Versions)
	}

	// Download endpoint (with download rate limit)
	download := api.Group("")
	download.Use(s.rateLimiter.RateLimit("download"))
	download.Use(middleware.OptionalAuth(s.idp))
	{
		download.GET("/download", s.h.download.Download)
	}

	// Authenticated endpoints (with write rate limit and scope enforcement)
	authed := api.Group("")
	authed.Use(middleware.RequireAuth(s.idp))
	authed.Use(middleware.RequireScope())
	authed.Use(s.rateLimiter.RateLimit("write"))
	{
		authed.GET("/whoami", s.h.auth.WhoAmI)
		authed.POST("/skills", s.h.skill.Publish)
		authed.POST("/skills/direct-upload", s.h.skill.DirectUploadPlan)
		authed.POST("/skills/direct-upload/complete", s.h.skill.DirectUploadComplete)
		authed.DELETE("/skills/:slug", s.h.skill.Delete)
		authed.DELETE("/skills/:slug/purge", s.h.skill.Purge)
		authed.POST("/skills/:slug/undelete", s.h.skill.Undelete)
		authed.POST("/skills/:slug/request-public", s.h.skill.RequestPublic)
		authed.PUT("/skills/:slug/file", s.h.skill.UpdateFile)
		authed.POST("/skills/:slug/transfer", s.h.skill.TransferSkill)
		authed.POST("/skills/:slug/versions/:version/yank", s.h.skill.YankVersion)
		authed.DELETE("/skills/:slug/versions/:version/yank", s.h.skill.UnyankVersion)
		authed.POST("/skills/:slug/versions/:version/deprecate", s.h.skill.DeprecateVersion)
		authed.DELETE("/skills/:slug/versions/:version/deprecate", s.h.skill.UndeprecateVersion)
		authed.POST("/stars/:slug", s.h.star.Star)
		authed.DELETE("/stars/:slug", s.h.star.Unstar)
		authed.GET("/tokens", s.h.token.List)
		authed.POST("/tokens", s.h.token.Create)
		authed.DELETE("/tokens/:id", s.h.token.Revoke)
		authed.POST("/namespaces", s.h.namespace.Create)
		authed.GET("/namespaces", s.h.namespace.List)
		authed.GET("/namespaces/:slug", s.h.namespace.Get)
		authed.PUT("/namespaces/:slug", s.h.namespace.Update)
		authed.DELETE("/namespaces/:slug", s.h.namespace.Delete)
		authed.POST("/namespaces/:slug/tokens", s.h.token.CreateForNamespace)
		authed.GET("/namespaces/:slug/tokens", s.h.token.ListForNamespace)
		authed.DELETE("/namespaces/:slug/tokens/:id", s.h.token.RevokeFromNamespace)
		authed.GET("/namespaces/:slug/members", s.h.namespace.ListMembers)
		authed.POST("/namespaces/:slug/members", s.h.namespace.AddMember)
		authed.DELETE("/namespaces/:slug/members/:handle", s.h.namespace.RemoveMember)
		authed.POST("/namespaces/:slug/leave", s.h.namespace.Leave)
		authed.POST("/namespaces/:slug/transfer", s.h.namespace.TransferOwnership)
		authed.POST("/namespaces/:slug/invitations", s.h.namespace.Invite)
		authed.GET("/namespaces/:slug/invitations", s.h.namespace.ListInvitations)
		authed.DELETE("/namespaces/:slug/invitations/:id", s.h.namespace.RevokeInvitation)
		authed.GET("/invitations", s.h.namespace.MyInvitations)
		authed.POST("/invitations/:id/accept", s.h.namespace.AcceptInvitation)
		authed.POST("/invitations/:id/decline", s.h.namespace.DeclineInvitation)
		authed.GET("/notifications", s.h.notif.List)
		authed.GET("/notifications/unread", s.h.notif.Unread)
		authed.POST("/notifications/:id/read", s.h.notif.MarkRead)
		authed.POST("/notifications/read-all", s.h.notif.MarkAllRead)
		authed.POST("/skills/:slug/ratings", s.h.rating.Rate)
		authed.DELETE("/skills/:slug/ratings", s.h.rating.Delete)
		authed.POST("/skills/:slug/comments", s.h.comment.Create)
		authed.DELETE("/comments/:id", s.h.comment.Delete)

		// Namespace-qualified skill routes (authenticated write)
		authed.DELETE("/skills/@:namespace/:slug", s.h.skill.Delete)
		authed.DELETE("/skills/@:namespace/:slug/purge", s.h.skill.Purge)
		authed.POST("/skills/@:namespace/:slug/undelete", s.h.skill.Undelete)
		authed.POST("/skills/@:namespace/:slug/request-public", s.h.skill.RequestPublic)
		authed.PUT("/skills/@:namespace/:slug/file", s.h.skill.UpdateFile)
		authed.POST("/skills/@:namespace/:slug/transfer", s.h.skill.TransferSkill)
		authed.POST("/skills/@:namespace/:slug/versions/:version/yank", s.h.skill.YankVersion)
		authed.DELETE("/skills/@:namespace/:slug/versions/:version/yank", s.h.skill.UnyankVersion)
		authed.POST("/skills/@:namespace/:slug/versions/:version/deprecate", s.h.skill.DeprecateVersion)
		authed.DELETE("/skills/@:namespace/:slug/versions/:version/deprecate", s.h.skill.UndeprecateVersion)
		authed.POST("/stars/@:namespace/:slug", s.h.star.Star)
		authed.DELETE("/stars/@:namespace/:slug", s.h.star.Unstar)
		authed.POST("/skills/@:namespace/:slug/ratings", s.h.rating.Rate)
		authed.DELETE("/skills/@:namespace/:slug/ratings", s.h.rating.Delete)
		authed.POST("/skills/@:namespace/:slug/comments", s.h.comment.Create)

		// Plugin publish (authenticated)
		authed.POST("/plugins", s.h.plugin.Publish)
		authed.DELETE("/plugins/:slug", s.h.plugin.Delete)
		authed.POST("/plugins/:slug/undelete", s.h.plugin.Undelete)
		authed.POST("/plugins/:slug/versions/:version/yank", s.h.plugin.YankVersion)
		authed.DELETE("/plugins/:slug/versions/:version/yank", s.h.plugin.UnyankVersion)

		// Namespace-qualified plugin routes (authenticated write)
		authed.DELETE("/plugins/@:namespace/:slug", s.h.plugin.Delete)
		authed.POST("/plugins/@:namespace/:slug/undelete", s.h.plugin.Undelete)
		authed.POST("/plugins/@:namespace/:slug/versions/:version/yank", s.h.plugin.YankVersion)
		authed.DELETE("/plugins/@:namespace/:slug/versions/:version/yank", s.h.plugin.UnyankVersion)
	}

	// Agent Skill API. Saker publishes learned/user-authored skills here and
	// loads skills dynamically via the GET endpoints.
	agentSkills := r.Group("/api/agent")
	agentSkills.Use(middleware.RequireAuth(s.idp))
	agentSkills.Use(middleware.RequireScope())
	{
		agentSkills.POST("/skills", s.rateLimiter.RateLimit("write"), s.h.agentSkill.Publish)
		agentSkills.GET("/skills", s.rateLimiter.RateLimit("read"), s.h.agentSkill.List)
		agentSkills.GET("/skills/:id", s.rateLimiter.RateLimit("read"), s.h.agentSkill.Detail)
		agentSkills.GET("/skills/:id/download", s.rateLimiter.RateLimit("download"), s.h.agentSkill.Download)
		agentSkills.GET("/skills/:id/archive", s.rateLimiter.RateLimit("download"), s.h.agentSkill.Archive)
	}

	// Admin endpoints
	admin := api.Group("/admin")
	admin.Use(middleware.RequireAuth(s.idp))
	admin.Use(middleware.RequireRole("admin"))
	{
		admin.POST("/users/ban", s.h.admin.BanUser)
		admin.POST("/users/role", s.h.admin.SetRole)
		admin.GET("/users", s.h.admin.ListUsers)
		admin.POST("/users", s.h.admin.CreateUser)
		admin.POST("/tokens", s.h.auth.CreateToken)
		admin.GET("/skills", s.h.admin.ListAllSkills)
		admin.POST("/skills/:slug/review", s.h.admin.ReviewSkill)
		admin.POST("/skills/:slug/visibility", s.h.admin.SetVisibility)
		admin.POST("/skills/@:namespace/:slug/review", s.h.admin.ReviewSkill)
		admin.POST("/skills/@:namespace/:slug/visibility", s.h.admin.SetVisibility)
		admin.GET("/audit-logs", s.h.audit.List)
		admin.GET("/plugins", s.h.admin.ListAllPlugins)
		admin.POST("/plugins/:slug/review", s.h.admin.ReviewPlugin)
		admin.POST("/plugins/:slug/visibility", s.h.admin.SetPluginVisibility)
	}

	// Agent auto-provisioning (rate-limited, requires SKILLHUB_AGENT_SECRET)
	api.POST("/agent/provision", s.rateLimiter.RateLimit("write"), s.h.agent.Provision)

	// Webhook endpoints
	webhooks := api.Group("/webhooks")
	{
		webhooks.POST("/github", s.h.webhook.GitHubWebhook)
		webhooks.POST("/gitlab", s.h.webhook.GitLabWebhook)
		webhooks.POST("/gitea", s.h.webhook.GiteaWebhook)
	}
}

// RegisterStatic 把 SkillHub 自带的前端 SPA 静态资源 + NoRoute 兜底挂到 engine。
//
// 这一段被单独抽出来的原因：
//   - StaticFS 与 NoRoute 都是 *gin.Engine 的方法，gin.IRouter（即 RouterGroup）没有 NoRoute；
//   - 嵌入方往往已经有自己的前端，不需要 SkillHub 的 SPA。把它独立成一个方法，
//     调用方按需选择是否复用。
//
// 注意：NoRoute 只能在 *gin.Engine 上注册一次；多次调用会以最后一次为准（gin 行为）。
func (s *Server) RegisterStatic(engine *gin.Engine) {
	bp := s.cfg.Server.BasePath // "" or e.g. "/skillhub"

	staticFS, _ := fs.Sub(web.StaticFS, "static/assets")
	engine.StaticFS(bp+"/assets", http.FS(staticFS))

	if swaggerFS, err := fs.Sub(web.StaticFS, "static/swagger"); err == nil {
		engine.StaticFS(bp+"/swagger", http.FS(swaggerFS))
	}

	engine.StaticFileFS(bp+"/swagger-init.js", "static/swagger-init.js", http.FS(web.StaticFS))

	indexHTML, _ := web.StaticFS.ReadFile("static/index.html")
	apiPrefix := bp + "/api/"
	engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, apiPrefix) {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		if bp != "" && !strings.HasPrefix(path, bp+"/") && path != bp {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.Data(200, "text/html; charset=utf-8", indexHTML)
	})
}

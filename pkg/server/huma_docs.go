package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/saker-ai/skillhub/pkg/model"
)

type docSlugInput struct {
	Slug string `path:"slug" doc:"Skill, plugin, or namespace slug" required:"true"`
}

type docNamespaceSkillInput struct {
	Namespace string `path:"namespace" doc:"Namespace slug" required:"true"`
	Slug      string `path:"slug" doc:"Skill or plugin slug" required:"true"`
}

type docVersionInput struct {
	Slug    string `path:"slug" doc:"Skill or plugin slug" required:"true"`
	Version string `path:"version" doc:"Version" required:"true"`
}

type docNamespaceVersionInput struct {
	Namespace string `path:"namespace" doc:"Namespace slug" required:"true"`
	Slug      string `path:"slug" doc:"Skill or plugin slug" required:"true"`
	Version   string `path:"version" doc:"Version" required:"true"`
}

type docIDInput struct {
	ID string `path:"id" doc:"Resource ID" required:"true"`
}

type docTokenIDInput struct {
	Slug string `path:"slug" doc:"Namespace slug" required:"true"`
	ID   string `path:"id" doc:"Token ID" required:"true"`
}

type docListInput struct {
	Limit    int    `query:"limit" doc:"Maximum number of results" required:"false"`
	Cursor   string `query:"cursor" doc:"Pagination cursor" required:"false"`
	Sort     string `query:"sort" doc:"Sort order" required:"false"`
	Category string `query:"category" doc:"Category filter" required:"false"`
}

type docSearchInput struct {
	Query string `query:"q" doc:"Search query" required:"false"`
	Limit int    `query:"limit" doc:"Maximum number of results" required:"false"`
}

type docSkillFileInput struct {
	Slug    string `path:"slug" doc:"Skill slug" required:"true"`
	Version string `query:"version" doc:"Version, defaults to latest" required:"false"`
	Path    string `query:"path" doc:"File path" required:"true"`
}

type docDownloadInput struct {
	Slug      string `query:"slug" doc:"Skill slug" required:"true"`
	Namespace string `query:"namespace" doc:"Namespace slug" required:"false"`
	Version   string `query:"version" doc:"Version, defaults to latest" required:"false"`
}

type docPluginFileInput struct {
	Slug      string `query:"slug" doc:"Plugin slug" required:"true"`
	Namespace string `query:"namespace" doc:"Optional namespace slug" required:"false"`
	Version   string `query:"version" doc:"Version, defaults to latest" required:"false"`
	Path      string `query:"path" doc:"File path" required:"true"`
}

type docPluginDownloadInput struct {
	Slug      string `query:"slug" doc:"Plugin slug" required:"true"`
	Namespace string `query:"namespace" doc:"Optional namespace slug" required:"false"`
	Version   string `query:"version" doc:"Version, defaults to latest" required:"false"`
}

type docSkillPublishInput struct {
	Slug        string `form:"slug" doc:"Skill slug" required:"true"`
	Version     string `form:"version" doc:"Version" required:"true"`
	DisplayName string `form:"displayName" doc:"Display name" required:"false"`
	Visibility  string `form:"visibility" doc:"private, workspace, or public" required:"false"`
	Overwrite   string `form:"overwrite" doc:"true/1 permits existing skills; false/0 rejects them" required:"false"`
	Files       []byte `form:"files" contentType:"application/octet-stream" required:"true"`
}

type docPluginPublishInput struct {
	Slug      string `form:"slug" doc:"Plugin slug" required:"true"`
	Version   string `form:"version" doc:"Version" required:"true"`
	Summary   string `form:"summary" doc:"Short summary" required:"false"`
	Category  string `form:"category" doc:"Category" required:"false"`
	Tags      string `form:"tags" doc:"Comma-separated tags" required:"false"`
	Namespace string `form:"namespace" doc:"Namespace slug" required:"false"`
}

type docTokenCreateInput struct {
	Body struct {
		Label     string `json:"label" required:"true"`
		Scope     string `json:"scope,omitempty"`
		ExpiresIn int    `json:"expiresIn,omitempty" doc:"Lifetime in seconds"`
	}
}

type docNamespaceCreateInput struct {
	Body struct {
		Slug              string `json:"slug" required:"true"`
		DisplayName       string `json:"displayName,omitempty"`
		Description       string `json:"description,omitempty"`
		DefaultVisibility string `json:"defaultVisibility,omitempty"`
		MaxSkills         int    `json:"maxSkills,omitempty"`
	}
}

type docNamespaceUpdateInput struct {
	Slug string `path:"slug" doc:"Namespace slug" required:"true"`
	Body struct {
		DisplayName       string `json:"displayName,omitempty"`
		Description       string `json:"description,omitempty"`
		DefaultVisibility string `json:"defaultVisibility,omitempty"`
		MaxSkills         int    `json:"maxSkills,omitempty"`
	}
}

type docMemberInput struct {
	Slug string `path:"slug" doc:"Namespace slug" required:"true"`
	Body struct {
		Handle string `json:"handle" required:"true"`
		Role   string `json:"role,omitempty"`
	}
}

type docMemberPathInput struct {
	Slug   string `path:"slug" doc:"Namespace slug" required:"true"`
	Handle string `path:"handle" doc:"User handle" required:"true"`
}

type docTransferInput struct {
	Slug string `path:"slug" doc:"Skill or namespace slug" required:"true"`
	Body struct {
		Namespace string `json:"namespace,omitempty"`
		Handle    string `json:"handle,omitempty"`
	}
}

type docYankInput struct {
	Slug    string `path:"slug" doc:"Skill or plugin slug" required:"true"`
	Version string `path:"version" doc:"Version" required:"true"`
	Body    struct {
		Reason string `json:"reason,omitempty"`
	}
}

type docRatingInput struct {
	Slug string `path:"slug" doc:"Skill slug" required:"true"`
	Body struct {
		Rating int `json:"rating" required:"true" minimum:"1" maximum:"5"`
	}
}

type docCommentInput struct {
	Slug string `path:"slug" doc:"Skill slug" required:"true"`
	Body struct {
		Body string `json:"body" required:"true"`
	}
}

type docLoginInput struct {
	Body struct {
		Handle   string `json:"handle" required:"true"`
		Password string `json:"password" required:"true"`
	}
}

type docDeviceCodeOutput struct {
	Body map[string]any
}

type docOKOutput struct {
	Body struct {
		OK bool `json:"ok"`
	}
}

type docStatusOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}

type docSkillListOutput struct {
	Body struct {
		Skills     []model.SkillWithOwner `json:"skills"`
		NextCursor string                 `json:"nextCursor,omitempty"`
	}
}

type docSkillOutput struct {
	Body struct {
		Skill model.SkillWithOwner `json:"skill"`
	}
}

type docAgentSkillPublishOutput struct {
	Body struct {
		Data struct {
			Skill struct {
				Slug string `json:"slug"`
			} `json:"skill"`
			Version struct {
				Version     string `json:"version"`
				Fingerprint string `json:"fingerprint"`
			} `json:"version"`
		} `json:"data"`
		RequestID string `json:"requestId"`
	}
}

type docSkillVersionsOutput struct {
	Body struct {
		Versions []model.SkillVersion `json:"versions"`
	}
}

type docPluginListOutput struct {
	Body struct {
		Plugins    []model.PluginWithOwner `json:"plugins"`
		NextCursor string                  `json:"nextCursor,omitempty"`
	}
}

type docPluginOutput struct {
	Body struct {
		Plugin model.PluginWithOwner `json:"plugin"`
	}
}

type docPluginVersionsOutput struct {
	Body struct {
		Versions []model.PluginVersion `json:"versions"`
	}
}

type docUserOutput struct {
	Body struct {
		User model.User `json:"user"`
	}
}

type docMapOutput struct {
	Body map[string]any
}

func registerOpenAPIDocs(api huma.API) {
	doc := NewDocOnlyAPI(api)
	public := []map[string][]string{}
	security := []map[string][]string{{"BearerAuth": {}}, {"CookieAuth": {}}}
	admin := []map[string][]string{{"BearerAuth": {}}, {"CookieAuth": {}}}

	registerDoc(doc, http.MethodGet, "/healthz", "healthz", "health", "Liveness check", public, (*struct{})(nil), (*docStatusOutput)(nil))
	registerDoc(doc, http.MethodGet, "/readyz", "readyz", "health", "Readiness check", public, (*struct{})(nil), (*docStatusOutput)(nil))
	registerDoc(doc, http.MethodGet, "/.well-known/clawhub.json", "well-known-clawhub", "well-known", "Service discovery document", public, (*struct{})(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodGet, "/skills.md", "skills-md", "well-known", "Agent operations guide", public, (*struct{})(nil), (*struct{})(nil))
	registerDoc(doc, http.MethodPost, "/login", "login", "auth", "Create web session", public, (*docLoginInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodPost, "/logout", "logout", "auth", "Clear web session", security, (*struct{})(nil), (*struct{})(nil))
	registerDoc(doc, http.MethodPost, "/auth/device/code", "device-code", "auth", "Request device auth code", public, (*struct{})(nil), (*docDeviceCodeOutput)(nil))
	registerDoc(doc, http.MethodPost, "/auth/device/token", "device-token", "auth", "Poll device auth token", public, (*docMapOutput)(nil), (*docMapOutput)(nil))

	registerDoc(doc, http.MethodGet, "/api/v1/search", "search", "skills", "Search skills", public, (*docSearchInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/categories", "categories", "skills", "List skill categories", public, (*struct{})(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/skills", "skills-list", "skills", "List skills", public, (*docListInput)(nil), (*docSkillListOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/agent/skills", "agent-skills-publish", "skills", "Publish skill", security, (*docSkillPublishInput)(nil), (*docAgentSkillPublishOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/skills/{slug}", "skills-get", "skills", "Get skill", public, (*docSlugInput)(nil), (*docSkillOutput)(nil))
	registerDoc(doc, http.MethodDelete, "/api/v1/skills/{slug}", "skills-delete", "skills", "Delete skill", security, (*docSlugInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/skills/{slug}/undelete", "skills-undelete", "skills", "Restore deleted skill", security, (*docSlugInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/skills/{slug}/request-public", "skills-request-public", "skills", "Request public review", security, (*docSlugInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/skills/{slug}/versions", "skills-versions", "skills", "List skill versions", public, (*docSlugInput)(nil), (*docSkillVersionsOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/skills/{slug}/versions/{version}", "skills-version", "skills", "Get skill version", public, (*docVersionInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/skills/{slug}/versions/{version}/yank", "skills-yank", "skills", "Yank skill version", security, (*docYankInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodDelete, "/api/v1/skills/{slug}/versions/{version}/yank", "skills-unyank", "skills", "Unyank skill version", security, (*docVersionInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/skills/{slug}/file", "skills-file", "skills", "Get skill file", public, (*docSkillFileInput)(nil), (*struct{})(nil))
	registerDoc(doc, http.MethodPut, "/api/v1/skills/{slug}/file", "skills-update-file", "skills", "Update skill file", security, (*docSlugInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/skills/{slug}/transfer", "skills-transfer", "skills", "Transfer skill", security, (*docTransferInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/download", "download", "skills", "Download skill archive", public, (*docDownloadInput)(nil), (*struct{})(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/resolve", "resolve", "skills", "Resolve skill version", public, (*docDownloadInput)(nil), (*docMapOutput)(nil))

	registerDoc(doc, http.MethodGet, "/api/v1/skills/@{namespace}/{slug}", "namespace-skills-get", "skills", "Get namespaced skill", public, (*docNamespaceSkillInput)(nil), (*docSkillOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/skills/@{namespace}/{slug}/versions", "namespace-skills-versions", "skills", "List namespaced skill versions", public, (*docNamespaceSkillInput)(nil), (*docSkillVersionsOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/skills/@{namespace}/{slug}/versions/{version}", "namespace-skills-version", "skills", "Get namespaced skill version", public, (*docNamespaceVersionInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/skills/@{namespace}/{slug}/file", "namespace-skills-file", "skills", "Get namespaced skill file", public, (*docNamespaceSkillInput)(nil), (*struct{})(nil))

	registerDoc(doc, http.MethodGet, "/api/v1/whoami", "whoami", "account", "Get current user", security, (*struct{})(nil), (*docUserOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/tokens", "tokens-list", "account", "List user tokens", security, (*struct{})(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/tokens", "tokens-create", "account", "Create user token", security, (*docTokenCreateInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodDelete, "/api/v1/tokens/{id}", "tokens-revoke", "account", "Revoke user token", security, (*docIDInput)(nil), (*docOKOutput)(nil))

	registerDoc(doc, http.MethodGet, "/api/v1/namespaces", "namespaces-list", "namespaces", "List namespaces", security, (*struct{})(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/namespaces", "namespaces-create", "namespaces", "Create namespace", security, (*docNamespaceCreateInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/namespaces/{slug}", "namespaces-get", "namespaces", "Get namespace", security, (*docSlugInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodPut, "/api/v1/namespaces/{slug}", "namespaces-update", "namespaces", "Update namespace", security, (*docNamespaceUpdateInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodDelete, "/api/v1/namespaces/{slug}", "namespaces-delete", "namespaces", "Delete namespace", security, (*docSlugInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/namespaces/{slug}/skills", "namespaces-skills", "namespaces", "List namespace skills", public, (*docListInput)(nil), (*docSkillListOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/namespaces/{slug}/members", "namespaces-members", "namespaces", "List members", security, (*docSlugInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/namespaces/{slug}/members", "namespaces-add-member", "namespaces", "Add member", security, (*docMemberInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodDelete, "/api/v1/namespaces/{slug}/members/{handle}", "namespaces-remove-member", "namespaces", "Remove member", security, (*docMemberPathInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/namespaces/{slug}/tokens", "namespace-tokens-list", "namespaces", "List namespace tokens", security, (*docSlugInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/namespaces/{slug}/tokens", "namespace-tokens-create", "namespaces", "Create namespace token", security, (*docTokenCreateInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodDelete, "/api/v1/namespaces/{slug}/tokens/{id}", "namespace-tokens-revoke", "namespaces", "Revoke namespace token", security, (*docTokenIDInput)(nil), (*docOKOutput)(nil))

	registerDoc(doc, http.MethodGet, "/api/v1/plugins", "plugins-list", "plugins", "List plugins", public, (*docListInput)(nil), (*docPluginListOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/plugins/by-id/{id}", "plugins-get-by-id", "plugins", "Get plugin by stable ID", public, (*docIDInput)(nil), (*docPluginOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/plugins", "plugins-publish", "plugins", "Publish plugin", security, (*docPluginPublishInput)(nil), (*docPluginOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/plugins/{slug}", "plugins-get", "plugins", "Get plugin", public, (*docSlugInput)(nil), (*docPluginOutput)(nil))
	registerDoc(doc, http.MethodDelete, "/api/v1/plugins/{slug}", "plugins-delete", "plugins", "Delete plugin", security, (*docSlugInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/plugins/{slug}/undelete", "plugins-undelete", "plugins", "Restore deleted plugin", security, (*docSlugInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/plugins/{slug}/versions", "plugins-versions", "plugins", "List plugin versions", public, (*docSlugInput)(nil), (*docPluginVersionsOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/plugins/{slug}/versions/{version}/yank", "plugins-yank", "plugins", "Yank plugin version", security, (*docYankInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodDelete, "/api/v1/plugins/{slug}/versions/{version}/yank", "plugins-unyank", "plugins", "Unyank plugin version", security, (*docVersionInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/plugins/file", "plugins-file", "plugins", "Get plugin file", public, (*docPluginFileInput)(nil), (*struct{})(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/plugins/download", "plugins-download", "plugins", "Download plugin archive", public, (*docPluginDownloadInput)(nil), (*struct{})(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/plugins/@{namespace}/{slug}", "namespace-plugins-get", "plugins", "Get namespaced plugin", public, (*docNamespaceSkillInput)(nil), (*docPluginOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/plugins/@{namespace}/{slug}/versions", "namespace-plugins-versions", "plugins", "List namespaced plugin versions", public, (*docNamespaceSkillInput)(nil), (*docPluginVersionsOutput)(nil))

	registerDoc(doc, http.MethodPost, "/api/v1/skills/{slug}/ratings", "ratings-create", "social", "Rate skill", security, (*docRatingInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/skills/{slug}/ratings", "ratings-list", "social", "List ratings", public, (*docSlugInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodDelete, "/api/v1/skills/{slug}/ratings", "ratings-delete", "social", "Delete rating", security, (*docSlugInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/skills/{slug}/comments", "comments-create", "social", "Create comment", security, (*docCommentInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/skills/{slug}/comments", "comments-list", "social", "List comments", public, (*docSlugInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodDelete, "/api/v1/comments/{id}", "comments-delete", "social", "Delete comment", security, (*docIDInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/notifications", "notifications-list", "account", "List notifications", security, (*struct{})(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/notifications/unread", "notifications-unread", "account", "Get unread notification count", security, (*struct{})(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/notifications/{id}/read", "notifications-read", "account", "Mark notification read", security, (*docIDInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/notifications/read-all", "notifications-read-all", "account", "Mark all notifications read", security, (*struct{})(nil), (*docOKOutput)(nil))

	registerDoc(doc, http.MethodGet, "/api/v1/admin/users", "admin-users-list", "admin", "List users", admin, (*struct{})(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/admin/users", "admin-users-create", "admin", "Create user", admin, (*docMapOutput)(nil), (*docUserOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/admin/users/ban", "admin-users-ban", "admin", "Ban user", admin, (*docMapOutput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/admin/users/role", "admin-users-role", "admin", "Set user role", admin, (*docMapOutput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/admin/skills", "admin-skills-list", "admin", "List all skills", admin, (*docListInput)(nil), (*docSkillListOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/admin/skills/{slug}/review", "admin-skills-review", "admin", "Review skill", admin, (*docSlugInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/admin/skills/@{namespace}/{slug}/review", "admin-skills-review-qualified", "admin", "Review namespace-qualified skill", admin, (*docSlugInput)(nil), (*docOKOutput)(nil))
	registerDoc(doc, http.MethodGet, "/api/v1/admin/audit-logs", "admin-audit-logs", "admin", "List audit logs", admin, (*docListInput)(nil), (*docMapOutput)(nil))
	registerDoc(doc, http.MethodPost, "/api/v1/agent/provision", "agent-provision", "agent", "Provision agent token", public, (*docMapOutput)(nil), (*docMapOutput)(nil))
}

func registerDoc[I, O any](api huma.API, method, path, operationID, tag, summary string, security []map[string][]string, _ *I, _ *O) {
	huma.Register(api, huma.Operation{
		OperationID: operationID,
		Method:      method,
		Path:        path,
		Tags:        []string{tag},
		Summary:     summary,
		Security:    security,
		Errors:      []int{400, 401, 403, 404, 429},
	}, func(ctx context.Context, input *I) (*O, error) { return nil, nil })
}

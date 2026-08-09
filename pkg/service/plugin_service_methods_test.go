package service

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saker-ai/skillhub/pkg/config"
	"github.com/saker-ai/skillhub/pkg/gitstore"
	"github.com/saker-ai/skillhub/pkg/model"
	"github.com/saker-ai/skillhub/pkg/repository"
	storegit "github.com/saker-ai/skillhub/pkg/store/git"
)

type pluginFixtures struct {
	db         *gorm.DB
	svc        *PluginService
	pluginRepo *repository.PluginRepo
	nsSvc      *NamespaceService
	owner      *model.User
	other      *model.User
	admin      *model.User
}

func setupPluginFixtures(t *testing.T) *pluginFixtures {
	t.Helper()
	tmp := t.TempDir()

	db, err := repository.NewDB(config.DatabaseConfig{
		Driver:      "sqlite",
		URL:         filepath.Join(tmp, "plugin.db"),
		AutoMigrate: true,
	})
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, _ := db.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	ctx := context.Background()
	userRepo := repository.NewUserRepo(db)
	pluginRepo := repository.NewPluginRepo(db)
	auditRepo := repository.NewAuditRepo(db)
	nsRepo := repository.NewNamespaceRepo(db)

	if err := pluginRepo.Migrate(ctx); err != nil {
		t.Fatalf("plugin migrate: %v", err)
	}

	owner := &model.User{ID: uuid.New(), Handle: "alice", Role: "user"}
	other := &model.User{ID: uuid.New(), Handle: "bob", Role: "user"}
	admin := &model.User{ID: uuid.New(), Handle: "root", Role: "admin"}
	for _, u := range []*model.User{owner, other, admin} {
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatalf("create user %s: %v", u.Handle, err)
		}
	}

	gs, err := gitstore.New(filepath.Join(tmp, "repos"))
	if err != nil {
		t.Fatalf("gitstore.New: %v", err)
	}
	fileStore := storegit.New(gs)
	auditSvc := NewAuditService(auditRepo)
	nsSvc := NewNamespaceService(nsRepo, userRepo)

	svc := NewPluginService(db, pluginRepo, fileStore, nil, auditSvc, slog.Default())
	svc.SetNamespaceService(nsSvc)

	return &pluginFixtures{
		db:         db,
		svc:        svc,
		pluginRepo: pluginRepo,
		nsSvc:      nsSvc,
		owner:      owner,
		other:      other,
		admin:      admin,
	}
}

func validPluginJSON(name string) []byte {
	return []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"` + name + `"}`)
}

func validPluginFiles(name string) map[string][]byte {
	return map[string][]byte{
		"plugin.json": validPluginJSON(name),
		"README.md":   []byte("# " + name),
	}
}

func (fx *pluginFixtures) publishBasePlugin(t *testing.T, slug string, as *model.User) *PluginPublishResult {
	t.Helper()
	result, err := fx.svc.Publish(context.Background(), PluginPublishInput{
		Slug:    slug,
		Version: "1.0.0",
		Files:   validPluginFiles(slug),
		OwnerID: as.ID,
	})
	if err != nil {
		t.Fatalf("publishBasePlugin(%s): %v", slug, err)
	}
	return result
}

func TestPluginService_Publish(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		setup   func(t *testing.T, fx *pluginFixtures)
		input   func(fx *pluginFixtures) PluginPublishInput
		wantErr string
	}{
		{
			name: "happy: publish new plugin 1.0.0",
			input: func(fx *pluginFixtures) PluginPublishInput {
				return PluginPublishInput{
					Slug:    "demo-plugin",
					Version: "1.0.0",
					Files:   validPluginFiles("demo-plugin"),
					OwnerID: fx.owner.ID,
				}
			},
		},
		{
			name: "reject: invalid slug",
			input: func(fx *pluginFixtures) PluginPublishInput {
				return PluginPublishInput{
					Slug:    "INVALID_SLUG!!",
					Version: "1.0.0",
					Files:   validPluginFiles("test"),
					OwnerID: fx.owner.ID,
				}
			},
			wantErr: "invalid plugin slug",
		},
		{
			name: "reject: invalid semver",
			input: func(fx *pluginFixtures) PluginPublishInput {
				return PluginPublishInput{
					Slug:    "bad-semver",
					Version: "not.valid",
					Files:   validPluginFiles("test"),
					OwnerID: fx.owner.ID,
				}
			},
			wantErr: "invalid semver",
		},
		{
			name: "reject: missing plugin.json",
			input: func(fx *pluginFixtures) PluginPublishInput {
				return PluginPublishInput{
					Slug:    "no-manifest",
					Version: "1.0.0",
					Files:   map[string][]byte{"README.md": []byte("# hello")},
					OwnerID: fx.owner.ID,
				}
			},
			wantErr: "plugin.json is required",
		},
		{
			name: "reject: plugin.json missing name",
			input: func(fx *pluginFixtures) PluginPublishInput {
				return PluginPublishInput{
					Slug:    "no-name",
					Version: "1.0.0",
					Files: map[string][]byte{
						"plugin.json": []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","version":"1.0.0"}`),
					},
					OwnerID: fx.owner.ID,
				}
			},
			wantErr: "name is required",
		},
		{
			name: "happy: plugin.json version is optional",
			input: func(fx *pluginFixtures) PluginPublishInput {
				return PluginPublishInput{
					Slug:    "no-ver",
					Version: "1.0.0",
					Files: map[string][]byte{
						"plugin.json": validPluginJSON("test"),
					},
					OwnerID: fx.owner.ID,
				}
			},
		},
		{
			name: "reject: duplicate version",
			setup: func(t *testing.T, fx *pluginFixtures) {
				fx.publishBasePlugin(t, "dup", fx.owner)
			},
			input: func(fx *pluginFixtures) PluginPublishInput {
				return PluginPublishInput{
					Slug:    "dup",
					Version: "1.0.0",
					Files: map[string][]byte{
						"plugin.json": validPluginJSON("dup"),
						"README.md":   []byte("# dup v2 content"),
					},
					OwnerID: fx.owner.ID,
				}
			},
			wantErr: "already exists",
		},
		{
			name: "reject: version not greater than latest",
			setup: func(t *testing.T, fx *pluginFixtures) {
				fx.publishBasePlugin(t, "older", fx.owner)
			},
			input: func(fx *pluginFixtures) PluginPublishInput {
				return PluginPublishInput{
					Slug:    "older",
					Version: "0.5.0",
					Files: map[string][]byte{
						"plugin.json": []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"older","version":"0.5.0"}`),
						"README.md":   []byte("# older downgrade content"),
					},
					OwnerID: fx.owner.ID,
				}
			},
			wantErr: "must be greater",
		},
		{
			name: "reject: non-owner publish to existing plugin",
			setup: func(t *testing.T, fx *pluginFixtures) {
				fx.publishBasePlugin(t, "owned", fx.owner)
			},
			input: func(fx *pluginFixtures) PluginPublishInput {
				return PluginPublishInput{
					Slug:    "owned",
					Version: "2.0.0",
					Files:   validPluginFiles("owned"),
					OwnerID: fx.other.ID,
				}
			},
			wantErr: "not the plugin owner",
		},
		{
			name: "happy: plugin with skill entries",
			input: func(fx *pluginFixtures) PluginPublishInput {
				return PluginPublishInput{
					Slug:    "with-skills",
					Version: "1.0.0",
					Files: map[string][]byte{
						"plugin.json":           validPluginJSON("with-skills"),
						"skills/greet/SKILL.md": []byte("# greet"),
					},
					OwnerID: fx.owner.ID,
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := setupPluginFixtures(t)
			if tc.setup != nil {
				tc.setup(t, fx)
			}

			result, err := fx.svc.Publish(context.Background(), tc.input(fx))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Publish: unexpected error %v", err)
				}
				if result == nil {
					t.Fatal("Publish: expected non-nil result")
				}
				return
			}
			if err == nil {
				t.Fatalf("Publish: want error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Publish: error %v does not contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestPluginService_SoftDelete(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		as      func(fx *pluginFixtures) *model.User
		publish bool
		wantErr error
		wantStr string
	}{
		{name: "owner can delete", as: func(fx *pluginFixtures) *model.User { return fx.owner }, publish: true},
		{name: "admin can delete", as: func(fx *pluginFixtures) *model.User { return fx.admin }, publish: true},
		{name: "non-owner forbidden", as: func(fx *pluginFixtures) *model.User { return fx.other }, publish: true, wantErr: ErrForbidden},
		{name: "not found", as: func(fx *pluginFixtures) *model.User { return fx.owner }, publish: false, wantErr: ErrNotFound},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := setupPluginFixtures(t)
			slug := "demo"
			if tc.publish {
				fx.publishBasePlugin(t, slug, fx.owner)
			}

			err := fx.svc.SoftDelete(context.Background(), tc.as(fx), slug)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("SoftDelete: want errors.Is %v, got %v", tc.wantErr, err)
				}
				return
			}
			if tc.wantStr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantStr) {
					t.Fatalf("SoftDelete: want error containing %q, got %v", tc.wantStr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("SoftDelete: unexpected error %v", err)
			}
			var got model.Plugin
			if err := fx.db.Where("slug = ?", slug).First(&got).Error; err != nil {
				t.Fatalf("raw plugin lookup after SoftDelete: %v", err)
			}
			if got.SoftDeletedAt == nil {
				t.Errorf("expected SoftDeletedAt to be set")
			}
		})
	}
}

func TestPluginService_Undelete(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		as        func(fx *pluginFixtures) *model.User
		preDelete bool
		wantErr   error
	}{
		{
			name:      "restore deleted plugin",
			as:        func(fx *pluginFixtures) *model.User { return fx.owner },
			preDelete: true,
		},
		{
			name:    "not deleted returns error",
			as:      func(fx *pluginFixtures) *model.User { return fx.owner },
			wantErr: ErrValidation,
		},
		{
			name:      "non-owner forbidden",
			as:        func(fx *pluginFixtures) *model.User { return fx.other },
			preDelete: true,
			wantErr:   ErrForbidden,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := setupPluginFixtures(t)
			slug := "demo"
			fx.publishBasePlugin(t, slug, fx.owner)

			if tc.preDelete {
				if err := fx.svc.SoftDelete(context.Background(), fx.owner, slug); err != nil {
					t.Fatalf("pre-delete: %v", err)
				}
			}

			err := fx.svc.Undelete(context.Background(), tc.as(fx), slug)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Undelete: want errors.Is %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Undelete: unexpected error %v", err)
			}
			p, err := fx.pluginRepo.GetBySlug(context.Background(), slug)
			if err != nil || p == nil {
				t.Fatalf("plugin missing after Undelete")
			}
			if p.SoftDeletedAt != nil {
				t.Errorf("SoftDeletedAt should be nil after Undelete")
			}
		})
	}
}

func TestPluginService_YankVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		setup   func(t *testing.T, fx *pluginFixtures, slug string)
		as      func(fx *pluginFixtures) *model.User
		version string
		wantErr error
	}{
		{
			name:    "owner yanks 1.0.0",
			as:      func(fx *pluginFixtures) *model.User { return fx.owner },
			version: "1.0.0",
		},
		{
			name: "double yank rejected",
			setup: func(t *testing.T, fx *pluginFixtures, slug string) {
				if err := fx.svc.YankVersion(context.Background(), fx.owner, slug, "1.0.0", "first"); err != nil {
					t.Fatalf("priming yank: %v", err)
				}
			},
			as:      func(fx *pluginFixtures) *model.User { return fx.owner },
			version: "1.0.0",
			wantErr: ErrConflict,
		},
		{
			name:    "non-existent version",
			as:      func(fx *pluginFixtures) *model.User { return fx.owner },
			version: "9.9.9",
			wantErr: ErrNotFound,
		},
		{
			name:    "non-owner forbidden",
			as:      func(fx *pluginFixtures) *model.User { return fx.other },
			version: "1.0.0",
			wantErr: ErrForbidden,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := setupPluginFixtures(t)
			slug := "demo"
			fx.publishBasePlugin(t, slug, fx.owner)
			if tc.setup != nil {
				tc.setup(t, fx, slug)
			}

			err := fx.svc.YankVersion(context.Background(), tc.as(fx), slug, tc.version, "test reason")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("YankVersion: want errors.Is %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("YankVersion: unexpected error %v", err)
			}
			p, _ := fx.pluginRepo.GetBySlug(context.Background(), slug)
			ver, _ := fx.pluginRepo.GetVersion(context.Background(), p.ID, tc.version)
			if ver == nil || ver.YankedAt == nil {
				t.Errorf("expected YankedAt to be set after yank")
			}
		})
	}
}

func TestPluginService_UnyankVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		preYank bool
		wantErr error
	}{
		{name: "unyank yanked version", preYank: true},
		{name: "unyank non-yanked version fails", wantErr: ErrValidation},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := setupPluginFixtures(t)
			slug := "demo"
			fx.publishBasePlugin(t, slug, fx.owner)

			ctx := context.Background()
			if tc.preYank {
				if err := fx.svc.YankVersion(ctx, fx.owner, slug, "1.0.0", "reason"); err != nil {
					t.Fatalf("priming yank: %v", err)
				}
			}

			err := fx.svc.UnyankVersion(ctx, fx.owner, slug, "1.0.0")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("UnyankVersion: want errors.Is %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("UnyankVersion: unexpected error %v", err)
			}
			p, _ := fx.pluginRepo.GetBySlug(ctx, slug)
			ver, _ := fx.pluginRepo.GetVersion(ctx, p.ID, "1.0.0")
			if ver == nil || ver.YankedAt != nil {
				t.Errorf("expected YankedAt to be nil after unyank")
			}
		})
	}
}

func TestPluginService_Download(t *testing.T) {
	t.Parallel()

	t.Run("download latest", func(t *testing.T) {
		t.Parallel()
		fx := setupPluginFixtures(t)
		fx.publishBasePlugin(t, "dl", fx.owner)

		reader, etag, err := fx.svc.Download(context.Background(), "dl", "")
		if err != nil {
			t.Fatalf("Download: %v", err)
		}
		defer reader.Close()
		if etag == "" {
			t.Error("expected non-empty etag")
		}
	})

	t.Run("download specific version", func(t *testing.T) {
		t.Parallel()
		fx := setupPluginFixtures(t)
		fx.publishBasePlugin(t, "dlv", fx.owner)

		reader, _, err := fx.svc.Download(context.Background(), "dlv", "1.0.0")
		if err != nil {
			t.Fatalf("Download: %v", err)
		}
		defer reader.Close()
	})

	t.Run("download yanked version rejected", func(t *testing.T) {
		t.Parallel()
		fx := setupPluginFixtures(t)
		fx.publishBasePlugin(t, "yanked", fx.owner)

		ctx := context.Background()
		if err := fx.svc.YankVersion(ctx, fx.owner, "yanked", "1.0.0", "broken"); err != nil {
			t.Fatalf("yank: %v", err)
		}

		_, _, err := fx.svc.Download(ctx, "yanked", "1.0.0")
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Download yanked: want ErrValidation, got %v", err)
		}
	})

	t.Run("download not found", func(t *testing.T) {
		t.Parallel()
		fx := setupPluginFixtures(t)

		_, _, err := fx.svc.Download(context.Background(), "nonexistent", "")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Download not found: want ErrNotFound, got %v", err)
		}
	})
}

func TestPluginService_GetFile(t *testing.T) {
	t.Parallel()

	t.Run("get existing file", func(t *testing.T) {
		t.Parallel()
		fx := setupPluginFixtures(t)
		fx.publishBasePlugin(t, "fileplug", fx.owner)

		data, err := fx.svc.GetFile(context.Background(), "fileplug", "1.0.0", "plugin.json")
		if err != nil {
			t.Fatalf("GetFile: %v", err)
		}
		if len(data) == 0 {
			t.Error("expected non-empty file content")
		}
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		t.Parallel()
		fx := setupPluginFixtures(t)
		fx.publishBasePlugin(t, "traverse", fx.owner)

		_, err := fx.svc.GetFile(context.Background(), "traverse", "1.0.0", "../../etc/passwd")
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("GetFile traversal: want ErrValidation, got %v", err)
		}
	})

	t.Run("not found plugin", func(t *testing.T) {
		t.Parallel()
		fx := setupPluginFixtures(t)

		_, err := fx.svc.GetFile(context.Background(), "nonexistent", "", "file.txt")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetFile not found: want ErrNotFound, got %v", err)
		}
	})
}

func TestPluginService_PublicReadsRejectPrivateAndYanked(t *testing.T) {
	fx := setupPluginFixtures(t)
	result := fx.publishBasePlugin(t, "guarded", fx.owner)
	result.Plugin.Visibility = "private"
	if err := fx.pluginRepo.Update(context.Background(), &result.Plugin); err != nil {
		t.Fatalf("make private: %v", err)
	}
	if _, err := fx.svc.Get(context.Background(), "guarded"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("private detail: want ErrNotFound, got %v", err)
	}
	if _, err := fx.svc.GetFile(context.Background(), "guarded", "1.0.0", "plugin.json"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("private file: want ErrNotFound, got %v", err)
	}

	result.Plugin.Visibility = "public"
	if err := fx.pluginRepo.Update(context.Background(), &result.Plugin); err != nil {
		t.Fatalf("make public: %v", err)
	}
	if err := fx.svc.YankVersion(context.Background(), fx.owner, "guarded", "1.0.0", "unsafe"); err != nil {
		t.Fatalf("yank: %v", err)
	}
	if _, err := fx.svc.GetFile(context.Background(), "guarded", "1.0.0", "plugin.json"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("yanked file: want ErrNotFound, got %v", err)
	}
}

func TestPluginService_NamespaceQualifiedRefsAreIsolated(t *testing.T) {
	t.Parallel()
	fx := setupPluginFixtures(t)
	ctx := context.Background()

	if _, err := fx.nsSvc.Create(ctx, fx.owner, "team-a", "", "", "team"); err != nil {
		t.Fatalf("create team-a: %v", err)
	}
	if _, err := fx.nsSvc.Create(ctx, fx.owner, "team-b", "", "", "team"); err != nil {
		t.Fatalf("create team-b: %v", err)
	}

	publish := func(ns, body string) {
		t.Helper()
		_, err := fx.svc.Publish(ctx, PluginPublishInput{
			Slug:          "shared",
			Version:       "1.0.0",
			NamespaceSlug: ns,
			User:          fx.owner,
			OwnerID:       fx.owner.ID,
			Files: map[string][]byte{
				"plugin.json": validPluginJSON("shared"),
				"README.md":   []byte(body),
			},
		})
		if err != nil {
			t.Fatalf("publish %s/shared: %v", ns, err)
		}
	}
	publish("team-a", "from team a")
	publish("team-b", "from team b")

	if _, err := fx.svc.Get(ctx, "shared"); err == nil {
		t.Fatalf("bare shared lookup should be ambiguous")
	}

	a, err := fx.svc.GetFile(ctx, "@team-a/shared", "1.0.0", "README.md")
	if err != nil {
		t.Fatalf("get team-a file: %v", err)
	}
	if string(a) != "from team a" {
		t.Fatalf("team-a file = %q", a)
	}
	b, err := fx.svc.GetFile(ctx, "@team-b/shared", "1.0.0", "README.md")
	if err != nil {
		t.Fatalf("get team-b file: %v", err)
	}
	if string(b) != "from team b" {
		t.Fatalf("team-b file = %q", b)
	}

	if err := fx.svc.SoftDelete(ctx, fx.owner, "@team-a/shared"); err != nil {
		t.Fatalf("delete team-a: %v", err)
	}
	if err := fx.svc.Undelete(ctx, fx.owner, "@team-a/shared"); err != nil {
		t.Fatalf("undelete team-a: %v", err)
	}
}

func TestPluginService_PublishNamespaceRequiresMembership(t *testing.T) {
	t.Parallel()
	fx := setupPluginFixtures(t)
	ctx := context.Background()
	if _, err := fx.nsSvc.Create(ctx, fx.owner, "locked", "", "", "team"); err != nil {
		t.Fatalf("create locked: %v", err)
	}
	_, err := fx.svc.Publish(ctx, PluginPublishInput{
		Slug:          "blocked",
		Version:       "1.0.0",
		NamespaceSlug: "locked",
		User:          fx.other,
		OwnerID:       fx.other.ID,
		Files:         validPluginFiles("blocked"),
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("publish as non-member: want ErrForbidden, got %v", err)
	}
}

func TestPluginService_Get(t *testing.T) {
	t.Parallel()

	t.Run("existing plugin", func(t *testing.T) {
		t.Parallel()
		fx := setupPluginFixtures(t)
		fx.publishBasePlugin(t, "getme", fx.owner)

		p, err := fx.svc.Get(context.Background(), "getme")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if p.Slug != "getme" {
			t.Errorf("slug = %q, want %q", p.Slug, "getme")
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		fx := setupPluginFixtures(t)

		_, err := fx.svc.Get(context.Background(), "nope")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get not found: want ErrNotFound, got %v", err)
		}
	})
}

func TestPluginService_Versions(t *testing.T) {
	t.Parallel()
	fx := setupPluginFixtures(t)
	fx.publishBasePlugin(t, "versioned", fx.owner)

	// Publish a second version with different content so git accepts the commit
	_, err := fx.svc.Publish(context.Background(), PluginPublishInput{
		Slug:    "versioned",
		Version: "2.0.0",
		Files: map[string][]byte{
			"plugin.json": []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"versioned","version":"2.0.0"}`),
			"README.md":   []byte("# versioned v2"),
		},
		OwnerID: fx.owner.ID,
	})
	if err != nil {
		t.Fatalf("publish 2.0.0: %v", err)
	}

	versions, err := fx.svc.Versions(context.Background(), "versioned")
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
}

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/gitstore"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/repository"
	"github.com/cinience/skillhub/pkg/search"
	"github.com/cinience/skillhub/pkg/semver"
	"github.com/cinience/skillhub/pkg/store"
	"gorm.io/gorm"
)

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+(-[\w.]+)?(\+[\w.]+)?$`)

type SkillService struct {
	db           *gorm.DB
	skillRepo    *repository.SkillRepo
	versionRepo  *repository.VersionRepo
	userRepo     *repository.UserRepo
	downloadRepo *repository.DownloadRepo
	starRepo     *repository.StarRepo
	fileStore    store.Store
	searchClient *search.Client
	mirrorSvc    *gitstore.MirrorService
	auditSvc     *AuditService
}

func NewSkillService(
	db *gorm.DB,
	skillRepo *repository.SkillRepo,
	versionRepo *repository.VersionRepo,
	userRepo *repository.UserRepo,
	downloadRepo *repository.DownloadRepo,
	starRepo *repository.StarRepo,
	fs store.Store,
	sc *search.Client,
	ms *gitstore.MirrorService,
	auditSvc *AuditService,
) *SkillService {
	return &SkillService{
		db:           db,
		skillRepo:    skillRepo,
		versionRepo:  versionRepo,
		userRepo:     userRepo,
		downloadRepo: downloadRepo,
		starRepo:     starRepo,
		fileStore:    fs,
		searchClient: sc,
		mirrorSvc:    ms,
		auditSvc:     auditSvc,
	}
}

type PublishRequest struct {
	Slug        string
	Version     string
	Changelog   string
	DisplayName string
	Summary     string
	Category    string
	Kind        string
	Tags        []string
	Files       map[string][]byte // path → content
}

func (s *SkillService) PublishVersion(ctx context.Context, user *model.User, req PublishRequest) (*model.SkillWithOwner, *model.SkillVersion, error) {
	// Validate slug
	if req.Slug == "" {
		return nil, nil, fmt.Errorf("slug is required")
	}
	reserved, err := s.skillRepo.IsSlugReserved(ctx, req.Slug)
	if err != nil {
		return nil, nil, fmt.Errorf("check reserved slug: %w", err)
	}
	if reserved {
		return nil, nil, fmt.Errorf("slug '%s' is reserved", req.Slug)
	}
	// Enforce reserved namespaces — animus-builtin/*, animus-domain/* are admin-only.
	if model.IsReservedNamespace(req.Slug) && !user.IsAdmin() {
		return nil, nil, fmt.Errorf("slug '%s' falls in a reserved namespace (admin only)", req.Slug)
	}
	// Validate kind if provided (default: custom)
	if req.Kind == "" {
		req.Kind = "custom"
	}
	if !model.IsValidKind(req.Kind) {
		return nil, nil, fmt.Errorf("invalid kind '%s'", req.Kind)
	}
	// Only admins may publish builtin/domain kinds.
	if (req.Kind == "builtin" || req.Kind == "domain") && !user.IsAdmin() {
		return nil, nil, fmt.Errorf("kind '%s' is admin-only", req.Kind)
	}

	// Validate semver
	if !semverRe.MatchString(req.Version) {
		return nil, nil, fmt.Errorf("invalid version '%s': must be valid semver (e.g. 1.0.0)", req.Version)
	}
	version := req.Version

	// Find or create skill
	skill, err := s.skillRepo.GetBySlug(ctx, req.Slug)
	if err != nil {
		return nil, nil, fmt.Errorf("get skill: %w", err)
	}

	if skill == nil {
		// Create new skill (default private)
		category := req.Category
		if category == "" {
			category = "general"
		}
		if !model.IsValidCategory(category) {
			return nil, nil, fmt.Errorf("invalid category '%s'", category)
		}
		// Auto-derive kind from reserved namespace prefix if caller didn't override.
		kind := req.Kind
		if kind == "" || kind == "custom" {
			switch {
			case strings.HasPrefix(req.Slug, "animus-builtin/"):
				kind = "builtin"
			case strings.HasPrefix(req.Slug, "animus-domain/"):
				kind = "domain"
			default:
				kind = req.Kind
			}
		}
		newSkill := &model.Skill{
			ID:               uuid.New(),
			Slug:             req.Slug,
			OwnerID:          user.ID,
			Category:         category,
			Tags:             model.StringArray(req.Tags),
			Visibility:       "private",
			ModerationStatus: "approved",
			Kind:             kind,
		}
		if req.DisplayName != "" {
			newSkill.DisplayName = &req.DisplayName
		}
		if req.Summary != "" {
			newSkill.Summary = &req.Summary
		}
		if err := s.skillRepo.Create(ctx, newSkill); err != nil {
			return nil, nil, fmt.Errorf("create skill: %w", err)
		}
		skill = &model.SkillWithOwner{
			Skill:       *newSkill,
			OwnerHandle: user.Handle,
		}
	} else {
		// Verify ownership
		if skill.OwnerID != user.ID && !user.IsAdmin() {
			return nil, nil, fmt.Errorf("you don't own this skill")
		}
	}

	// Check version doesn't exist
	existing, err := s.versionRepo.GetBySkillAndVersion(ctx, skill.ID, version)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		return nil, nil, fmt.Errorf("version %s already exists", version)
	}

	// Check version is greater than latest
	latest, _ := s.versionRepo.GetLatest(ctx, skill.ID)
	if latest != nil {
		if semver.Compare(version, latest.Version) <= 0 {
			return nil, nil, fmt.Errorf("version %s must be greater than current latest %s", version, latest.Version)
		}
	}

	// Validate and compute file metadata and fingerprint
	var filesMeta []model.VersionFile
	var hashParts []string
	for path, content := range req.Files {
		cleanPath := sanitizeFilePath(path)
		if cleanPath == "" {
			return nil, nil, fmt.Errorf("invalid file path: %s", path)
		}
		if cleanPath != path {
			req.Files[cleanPath] = content
			delete(req.Files, path)
			path = cleanPath
		}
		h := sha256.Sum256(content)
		fileHash := hex.EncodeToString(h[:])
		filesMeta = append(filesMeta, model.VersionFile{
			Path:   path,
			Size:   int64(len(content)),
			SHA256: fileHash,
		})
		hashParts = append(hashParts, fileHash)
	}
	sort.Strings(hashParts)
	aggregateHash := sha256.Sum256([]byte(strings.Join(hashParts, ":")))
	fingerprint := hex.EncodeToString(aggregateHash[:])

	filesJSON, _ := json.Marshal(filesMeta)

	// Extract parsed info from SKILL.md if present
	parsedJSON := json.RawMessage("{}")
	if content, ok := req.Files["SKILL.md"]; ok {
		parsedJSON = extractFrontmatter(content)
	}

	// Publish to git
	email := user.Handle + "@skillhub.local"
	commitHash, err := s.fileStore.Publish(ctx, store.PublishOpts{
		Owner:   user.Handle,
		Slug:    req.Slug,
		Version: version,
		Files:   req.Files,
		Author:  user.Handle,
		Email:   email,
		Message: req.Changelog,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("git publish: %w", err)
	}

	// Create version record
	ver := &model.SkillVersion{
		ID:            uuid.New(),
		SkillID:       skill.ID,
		Version:       version,
		Fingerprint:   fingerprint,
		GitCommitHash: &commitHash,
		Files:         filesJSON,
		Parsed:        parsedJSON,
		CreatedBy:     user.ID,
		SHA256Hash:    fingerprint,
	}
	if req.Changelog != "" {
		ver.Changelog = &req.Changelog
	}

	// Wrap DB writes in a transaction for atomicity
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ver).Error; err != nil {
			return fmt.Errorf("create version: %w", err)
		}
		if err := tx.Model(&model.Skill{}).Where("id = ?", skill.ID).
			Updates(map[string]interface{}{
				"latest_version_id": ver.ID,
				"versions_count":    gorm.Expr("versions_count + 1"),
				"updated_at":        ver.CreatedAt,
			}).Error; err != nil {
			return fmt.Errorf("update latest version: %w", err)
		}
		// Update metadata if provided
		if req.DisplayName != "" || req.Summary != "" || req.Category != "" || len(req.Tags) > 0 {
			updates := map[string]interface{}{}
			if req.DisplayName != "" {
				skill.DisplayName = &req.DisplayName
				updates["display_name"] = req.DisplayName
			}
			if req.Summary != "" {
				skill.Summary = &req.Summary
				updates["summary"] = req.Summary
			}
			if req.Category != "" && model.IsValidCategory(req.Category) {
				skill.Category = req.Category
				updates["category"] = req.Category
			}
			if len(req.Tags) > 0 {
				skill.Tags = model.StringArray(req.Tags)
				updates["tags"] = skill.Tags
			}
			if len(updates) > 0 {
				if err := tx.Model(&model.Skill{}).Where("id = ?", skill.ID).Updates(updates).Error; err != nil {
					return fmt.Errorf("update skill metadata: %w", err)
				}
			}
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}

	// Index to search
	if s.searchClient != nil {
		skillMdContent := ""
		if content, ok := req.Files["SKILL.md"]; ok {
			skillMdContent = string(content)
		}
		doc := &search.SkillDocument{
			ID:               skill.ID.String(),
			Slug:             skill.Slug,
			DisplayName:      derefStr(skill.DisplayName),
			Summary:          derefStr(skill.Summary),
			SkillMdContent:   skillMdContent,
			Category:         skill.Category,
			Tags:             []string(skill.Tags),
			OwnerHandle:      skill.OwnerHandle,
			Visibility:       skill.Visibility,
			ModerationStatus: skill.ModerationStatus,
			IsSuspicious:     skill.IsSuspicious,
			IsDeleted:        skill.SoftDeletedAt != nil,
			Downloads:        skill.Downloads,
			Stars:            skill.StarsCount,
			UpdatedAt:        skill.UpdatedAt.Unix(),
			CreatedAt:        skill.CreatedAt.Unix(),
		}
		if err := s.searchClient.IndexSkill(ctx, doc); err != nil {
			log.Printf("warning: failed to index skill to search: %v", err)
		}
	}

	// Mirror push (async)
	if s.mirrorSvc != nil && s.mirrorSvc.Enabled() {
		go func() {
			if err := s.mirrorSvc.PushMirror(context.Background(), user.Handle, req.Slug); err != nil {
				log.Printf("warning: mirror push failed for %s/%s: %v", user.Handle, req.Slug, err)
			}
		}()
	}

	// Write audit log
	if s.auditSvc != nil {
		s.auditSvc.Log(ctx, &user.ID, "publish", "skill_version", &ver.ID, "", "")
	}

	return skill, ver, nil
}

// DownloadResult carries a zip archive plus metadata for ETag / filename.
type DownloadResult struct {
	Archive     io.ReadCloser
	Filename    string
	Fingerprint string
	Version     string
}

// ResolveVersion returns the version record for a slug+version (accepts "latest").
// It enforces visibility. Returned SkillVersion is non-nil on success.
func (s *SkillService) ResolveVersion(ctx context.Context, slug, version string, viewer *model.User) (*model.SkillWithOwner, *model.SkillVersion, error) {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil {
		return nil, nil, err
	}
	if skill == nil || !canViewSkill(skill, viewer) {
		return nil, nil, fmt.Errorf("skill not found: %s", slug)
	}

	if version == "" || version == "latest" {
		if skill.LatestVersionID == nil {
			return skill, nil, fmt.Errorf("no versions published")
		}
		v, err := s.versionRepo.GetByID(ctx, *skill.LatestVersionID)
		if err != nil {
			return skill, nil, err
		}
		if v == nil {
			return skill, nil, fmt.Errorf("latest version not found")
		}
		return skill, v, nil
	}

	ver, err := s.versionRepo.GetBySkillAndVersion(ctx, skill.ID, version)
	if err != nil {
		return skill, nil, err
	}
	if ver == nil {
		return skill, nil, fmt.Errorf("version %s not found", version)
	}
	return skill, ver, nil
}

// Download returns a zip archive for a skill version along with its fingerprint.
func (s *SkillService) Download(ctx context.Context, slug, version string, identityHash string, viewer *model.User) (*DownloadResult, error) {
	skill, ver, err := s.ResolveVersion(ctx, slug, version, viewer)
	if err != nil {
		return nil, err
	}

	archive, err := s.fileStore.Archive(skill.OwnerHandle, slug, ver.Version)
	if err != nil {
		return nil, fmt.Errorf("create archive: %w", err)
	}

	// Record download (deduplicate) — best-effort, not fatal.
	if identityHash != "" {
		isNew, _ := s.downloadRepo.RecordDownload(ctx, skill.ID, ver.ID, identityHash)
		if isNew {
			s.skillRepo.IncrementDownloads(ctx, skill.ID)
		}
	}

	return &DownloadResult{
		Archive:     archive,
		Filename:    fmt.Sprintf("%s-%s.zip", slug, ver.Version),
		Fingerprint: ver.Fingerprint,
		Version:     ver.Version,
	}, nil
}

// GetFile reads a single file from a skill version.
func (s *SkillService) GetFile(ctx context.Context, slug, version, filePath string, viewer *model.User) ([]byte, error) {
	cleanPath := sanitizeFilePath(filePath)
	if cleanPath == "" {
		return nil, fmt.Errorf("invalid file path")
	}
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil {
		return nil, err
	}
	if skill == nil {
		return nil, fmt.Errorf("skill not found: %s", slug)
	}
	if !canViewSkill(skill, viewer) {
		return nil, fmt.Errorf("skill not found: %s", slug)
	}

	if version == "" || version == "latest" {
		if skill.LatestVersionID == nil {
			return nil, fmt.Errorf("no versions published")
		}
		v, err := s.versionRepo.GetByID(ctx, *skill.LatestVersionID)
		if err != nil {
			return nil, err
		}
		version = v.Version
	}

	content, err := s.fileStore.GetFile(skill.OwnerHandle, slug, version, cleanPath)
	if err != nil {
		return nil, err
	}

	// 200KB limit
	if len(content) > 200*1024 {
		return nil, fmt.Errorf("file too large (max 200KB)")
	}
	return content, nil
}

// ResolveFingerprint finds a version by its fingerprint and returns the associated skill.
func (s *SkillService) ResolveFingerprint(ctx context.Context, fingerprint string) (*model.SkillVersion, *model.SkillWithOwner, error) {
	ver, err := s.versionRepo.GetByFingerprint(ctx, fingerprint)
	if err != nil || ver == nil {
		return nil, nil, err
	}
	skill, err := s.skillRepo.GetByID(ctx, ver.SkillID)
	if err != nil || skill == nil {
		return ver, nil, err
	}
	owner, _ := s.userRepo.GetByID(ctx, skill.OwnerID)
	swo := &model.SkillWithOwner{Skill: *skill}
	if owner != nil {
		swo.OwnerHandle = owner.Handle
	}
	return ver, swo, nil
}

// GetSkill returns a skill by slug with visibility check.
// viewer may be nil for anonymous access.
func (s *SkillService) GetSkill(ctx context.Context, slug string, viewer *model.User) (*model.SkillWithOwner, error) {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return nil, err
	}
	if !canViewSkill(skill, viewer) {
		return nil, nil // invisible = not found
	}
	return skill, nil
}

// ListSkills returns a paginated list of skills with visibility filtering.
func (s *SkillService) ListSkills(ctx context.Context, limit int, cursor, sort, category string, viewer *model.User) ([]model.SkillWithOwner, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	filter := repository.ListFilter{Category: category}
	if viewer != nil {
		filter.ViewerID = &viewer.ID
		filter.IsAdmin = viewer.IsModerator()
	}
	return s.skillRepo.List(ctx, limit, cursor, sort, filter)
}

// ListAllSkillsForAdmin returns all skills for admin management.
func (s *SkillService) ListAllSkillsForAdmin(ctx context.Context, limit int, cursor, visibility string) ([]model.SkillWithOwner, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.skillRepo.ListAllForAdmin(ctx, limit, cursor, visibility)
}

// RequestPublic lets the owner request to make a skill public.
func (s *SkillService) RequestPublic(ctx context.Context, user *model.User, slug string) error {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return fmt.Errorf("skill not found")
	}
	if skill.OwnerID != user.ID && !user.IsAdmin() {
		return fmt.Errorf("forbidden")
	}
	if skill.Visibility == "public" && skill.ModerationStatus == "approved" {
		return fmt.Errorf("skill is already public")
	}
	if err := s.skillRepo.SetVisibility(ctx, skill.ID, "private", "pending_review"); err != nil {
		return err
	}
	if s.auditSvc != nil {
		s.auditSvc.Log(ctx, &user.ID, "request_public", "skill", &skill.ID, "", "")
	}
	return nil
}

// ReviewSkill lets an admin/moderator approve or reject a skill for public visibility.
func (s *SkillService) ReviewSkill(ctx context.Context, reviewerID *uuid.UUID, slug string, approve bool) error {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return fmt.Errorf("skill not found")
	}
	action := "reject"
	if approve {
		action = "approve"
		if err := s.skillRepo.SetVisibility(ctx, skill.ID, "public", "approved"); err != nil {
			return err
		}
	} else {
		if err := s.skillRepo.SetVisibility(ctx, skill.ID, "private", "rejected"); err != nil {
			return err
		}
	}
	if s.auditSvc != nil {
		s.auditSvc.Log(ctx, reviewerID, action, "skill", &skill.ID, "", "")
	}
	return nil
}

// SetSkillVisibility lets an admin directly set a skill's visibility.
func (s *SkillService) SetSkillVisibility(ctx context.Context, adminID *uuid.UUID, slug, visibility string) error {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return fmt.Errorf("skill not found")
	}
	if visibility != "public" && visibility != "private" {
		return fmt.Errorf("visibility must be 'public' or 'private'")
	}
	moderationStatus := "approved"
	if err := s.skillRepo.SetVisibility(ctx, skill.ID, visibility, moderationStatus); err != nil {
		return err
	}
	if s.auditSvc != nil {
		s.auditSvc.Log(ctx, adminID, "set_visibility", "skill", &skill.ID, visibility, "")
	}
	return nil
}

// canViewSkill checks if a viewer has access to a skill.
func canViewSkill(skill *model.SkillWithOwner, viewer *model.User) bool {
	// Public and approved: visible to all
	if skill.Visibility == "public" && skill.ModerationStatus == "approved" {
		return true
	}
	if viewer == nil {
		return false
	}
	// Admin/moderator can see all
	if viewer.IsModerator() {
		return true
	}
	// Owner can see their own skill
	return skill.OwnerID == viewer.ID
}

// GetVersions returns all versions for a skill.
func (s *SkillService) GetVersions(ctx context.Context, slug string, viewer *model.User) ([]model.SkillVersion, error) {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return nil, err
	}
	if !canViewSkill(skill, viewer) {
		return nil, nil
	}
	return s.versionRepo.ListBySkill(ctx, skill.ID)
}

// GetVersion returns a specific version.
func (s *SkillService) GetVersion(ctx context.Context, slug, version string, viewer *model.User) (*model.SkillVersion, error) {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return nil, err
	}
	if !canViewSkill(skill, viewer) {
		return nil, nil
	}
	return s.versionRepo.GetBySkillAndVersion(ctx, skill.ID, version)
}

// SoftDelete soft-deletes a skill.
func (s *SkillService) SoftDelete(ctx context.Context, user *model.User, slug string) error {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return fmt.Errorf("skill not found")
	}
	if skill.OwnerID != user.ID && !user.IsAdmin() {
		return fmt.Errorf("forbidden")
	}
	if err := s.skillRepo.SoftDelete(ctx, skill.ID); err != nil {
		return err
	}
	if s.auditSvc != nil {
		s.auditSvc.Log(ctx, &user.ID, "delete", "skill", &skill.ID, "", "")
	}
	return nil
}

// Undelete restores a soft-deleted skill.
func (s *SkillService) Undelete(ctx context.Context, user *model.User, slug string) error {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return fmt.Errorf("skill not found")
	}
	if skill.OwnerID != user.ID && !user.IsAdmin() {
		return fmt.Errorf("forbidden")
	}
	if err := s.skillRepo.Undelete(ctx, skill.ID); err != nil {
		return err
	}
	if s.auditSvc != nil {
		s.auditSvc.Log(ctx, &user.ID, "undelete", "skill", &skill.ID, "", "")
	}
	return nil
}

// Star adds a star to a skill (atomic transaction).
func (s *SkillService) Star(ctx context.Context, userID uuid.UUID, slug string) error {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return fmt.Errorf("skill not found")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.starRepo.Star(ctx, userID, skill.ID); err != nil {
			return err
		}
		return s.skillRepo.UpdateStarsCount(ctx, skill.ID, 1)
	})
}

// Unstar removes a star from a skill (atomic transaction).
func (s *SkillService) Unstar(ctx context.Context, userID uuid.UUID, slug string) error {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return fmt.Errorf("skill not found")
	}
	starred, err := s.starRepo.IsStarred(ctx, userID, skill.ID)
	if err != nil {
		return err
	}
	if !starred {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.starRepo.Unstar(ctx, userID, skill.ID); err != nil {
			return err
		}
		return s.skillRepo.UpdateStarsCount(ctx, skill.ID, -1)
	})
}

const (
	maxUploadFiles   = 500
	maxFileSize      = 5 * 1024 * 1024 // 5MB per file
)

// ReadMultipartFiles reads all files from a multipart form.
func ReadMultipartFiles(form *multipart.Form) (map[string][]byte, error) {
	files := make(map[string][]byte)
	for _, headers := range form.File {
		for _, header := range headers {
			if len(files) >= maxUploadFiles {
				return nil, fmt.Errorf("too many files (max %d)", maxUploadFiles)
			}
			if header.Size > maxFileSize {
				return nil, fmt.Errorf("file %s exceeds max size (%d bytes)", header.Filename, maxFileSize)
			}
			name := sanitizeFilePath(header.Filename)
			if name == "" {
				continue
			}
			f, err := header.Open()
			if err != nil {
				return nil, fmt.Errorf("open file %s: %w", name, err)
			}
			data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
			f.Close()
			if err != nil {
				return nil, fmt.Errorf("read file %s: %w", name, err)
			}
			if int64(len(data)) > maxFileSize {
				return nil, fmt.Errorf("file %s exceeds max size (%d bytes)", name, maxFileSize)
			}
			files[name] = data
		}
	}
	return files, nil
}

// sanitizeFilePath cleans a user-supplied file path, rejecting traversal attempts.
func sanitizeFilePath(name string) string {
	name = path.Clean(name)
	name = strings.TrimPrefix(name, "/")
	if name == "." || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") {
		return ""
	}
	return name
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func extractFrontmatter(content []byte) json.RawMessage {
	s := string(content)
	if !strings.HasPrefix(s, "---") {
		return json.RawMessage("{}")
	}
	end := strings.Index(s[3:], "---")
	if end == -1 {
		return json.RawMessage("{}")
	}
	fm := strings.TrimSpace(s[3 : end+3])
	result, _ := json.Marshal(map[string]string{"raw": fm})
	return result
}


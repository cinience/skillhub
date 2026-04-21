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
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/gitstore"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/repository"
	"github.com/cinience/skillhub/pkg/search"
	"github.com/cinience/skillhub/pkg/store"
)

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+(-[\w.]+)?(\+[\w.]+)?$`)

type SkillService struct {
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
		newSkill := &model.Skill{
			ID:               uuid.New(),
			Slug:             req.Slug,
			OwnerID:          user.ID,
			Tags:             model.StringArray(req.Tags),
			Visibility:       "private",
			ModerationStatus: "approved",
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
		if compareSemver(version, latest.Version) <= 0 {
			return nil, nil, fmt.Errorf("version %s must be greater than current latest %s", version, latest.Version)
		}
	}

	// Compute file metadata and fingerprint
	var filesMeta []model.VersionFile
	var hashParts []string
	for path, content := range req.Files {
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

	if err := s.versionRepo.Create(ctx, ver); err != nil {
		return nil, nil, fmt.Errorf("create version: %w", err)
	}

	// Update skill
	if err := s.skillRepo.UpdateLatestVersion(ctx, skill.ID, ver.ID); err != nil {
		return nil, nil, fmt.Errorf("update latest version: %w", err)
	}

	// Update display name and summary if provided
	if req.DisplayName != "" || req.Summary != "" || len(req.Tags) > 0 {
		if req.DisplayName != "" {
			skill.DisplayName = &req.DisplayName
		}
		if req.Summary != "" {
			skill.Summary = &req.Summary
		}
		if len(req.Tags) > 0 {
			skill.Tags = model.StringArray(req.Tags)
		}
		s.skillRepo.Update(ctx, &skill.Skill)
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

// Download returns a zip archive for a skill version.
func (s *SkillService) Download(ctx context.Context, slug, version string, identityHash string, viewer *model.User) (io.ReadCloser, string, error) {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil {
		return nil, "", err
	}
	if skill == nil {
		return nil, "", fmt.Errorf("skill not found: %s", slug)
	}
	if !canViewSkill(skill, viewer) {
		return nil, "", fmt.Errorf("skill not found: %s", slug)
	}

	// Resolve version
	if version == "" || version == "latest" {
		if skill.LatestVersionID == nil {
			return nil, "", fmt.Errorf("no versions published")
		}
		v, err := s.versionRepo.GetByID(ctx, *skill.LatestVersionID)
		if err != nil {
			return nil, "", err
		}
		if v == nil {
			return nil, "", fmt.Errorf("latest version not found")
		}
		version = v.Version
	}

	ver, err := s.versionRepo.GetBySkillAndVersion(ctx, skill.ID, version)
	if err != nil {
		return nil, "", err
	}
	if ver == nil {
		return nil, "", fmt.Errorf("version %s not found", version)
	}

	// Get archive from git
	archive, err := s.fileStore.Archive(skill.OwnerHandle, slug, version)
	if err != nil {
		return nil, "", fmt.Errorf("create archive: %w", err)
	}

	// Record download (deduplicate)
	if identityHash != "" {
		isNew, _ := s.downloadRepo.RecordDownload(ctx, skill.ID, ver.ID, identityHash)
		if isNew {
			s.skillRepo.IncrementDownloads(ctx, skill.ID)
		}
	}

	filename := fmt.Sprintf("%s-%s.zip", slug, version)
	return archive, filename, nil
}

// GetFile reads a single file from a skill version.
func (s *SkillService) GetFile(ctx context.Context, slug, version, path string, viewer *model.User) ([]byte, error) {
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

	content, err := s.fileStore.GetFile(skill.OwnerHandle, slug, version, path)
	if err != nil {
		return nil, err
	}

	// 200KB limit
	if len(content) > 200*1024 {
		return nil, fmt.Errorf("file too large (max 200KB)")
	}
	return content, nil
}

// ResolveFingerprint finds a version by its fingerprint.
func (s *SkillService) ResolveFingerprint(ctx context.Context, fingerprint string) (*model.SkillVersion, *model.SkillWithOwner, error) {
	ver, err := s.versionRepo.GetByFingerprint(ctx, fingerprint)
	if err != nil || ver == nil {
		return nil, nil, err
	}
	return ver, nil, nil
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
func (s *SkillService) ListSkills(ctx context.Context, limit int, cursor, sort string, viewer *model.User) ([]model.SkillWithOwner, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	filter := repository.ListFilter{}
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

// Star adds a star to a skill.
func (s *SkillService) Star(ctx context.Context, userID uuid.UUID, slug string) error {
	skill, err := s.skillRepo.GetBySlugOrAlias(ctx, slug)
	if err != nil || skill == nil {
		return fmt.Errorf("skill not found")
	}
	if err := s.starRepo.Star(ctx, userID, skill.ID); err != nil {
		return err
	}
	return s.skillRepo.UpdateStarsCount(ctx, skill.ID, 1)
}

// Unstar removes a star from a skill.
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
		return nil // not starred, nothing to do
	}
	if err := s.starRepo.Unstar(ctx, userID, skill.ID); err != nil {
		return err
	}
	return s.skillRepo.UpdateStarsCount(ctx, skill.ID, -1)
}

const maxUploadFiles = 500

// ReadMultipartFiles reads all files from a multipart form.
func ReadMultipartFiles(form *multipart.Form) (map[string][]byte, error) {
	files := make(map[string][]byte)
	for _, headers := range form.File {
		for _, header := range headers {
			if len(files) >= maxUploadFiles {
				return nil, fmt.Errorf("too many files (max %d)", maxUploadFiles)
			}
			name := sanitizeFilePath(header.Filename)
			if name == "" {
				continue
			}
			f, err := header.Open()
			if err != nil {
				return nil, fmt.Errorf("open file %s: %w", name, err)
			}
			data, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				return nil, fmt.Errorf("read file %s: %w", name, err)
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

// compareSemver compares two semver strings. Returns -1, 0, or 1.
func compareSemver(a, b string) int {
	aParts := parseSemverParts(a)
	bParts := parseSemverParts(b)
	for i := 0; i < 3; i++ {
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}
	return 0
}

func parseSemverParts(v string) [3]int {
	// Strip pre-release and build metadata
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		result[i], _ = strconv.Atoi(parts[i])
	}
	return result
}

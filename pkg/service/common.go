package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"path"
	"sort"
	"strings"

	"github.com/saker-ai/skillhub/pkg/model"
	"github.com/saker-ai/skillhub/pkg/repository"
	"gorm.io/gorm"
)

const (
	maxUploadFiles                 = 500
	maxFileSize                    = 5 * 1024 * 1024 // 5MB per file
	directUploadMultipartThreshold = 64 << 20
	directUploadMultipartPartSize  = 16 << 20
)

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func formValue(form *multipart.Form, key string) string {
	if vals, ok := form.Value[key]; ok && len(vals) > 0 {
		return strings.TrimSpace(vals[0])
	}
	return ""
}

// computeFingerprint hashes file names and contents together.
// Used by plugins where file renames must produce a different fingerprint.
func computeFingerprint(files map[string][]byte) string {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write(files[k])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// computeContentFingerprint hashes only file contents (not names).
// Used by skills to preserve backward-compatible fingerprints for existing versions.
func computeContentFingerprint(files map[string][]byte) string {
	var hashParts []string
	for _, content := range files {
		h := sha256.Sum256(content)
		hashParts = append(hashParts, hex.EncodeToString(h[:]))
	}
	sort.Strings(hashParts)
	aggregate := sha256.Sum256([]byte(strings.Join(hashParts, ":")))
	return hex.EncodeToString(aggregate[:])
}

// ReadMultipartFiles reads all files from a multipart form.
func ReadMultipartFiles(form *multipart.Form) (map[string][]byte, error) {
	return readMultipartFiles(form, maxFileSize)
}

func readMultipartFiles(form *multipart.Form, fileSizeLimit int64) (map[string][]byte, error) {
	files := make(map[string][]byte)
	for _, headers := range form.File {
		for _, header := range headers {
			if len(files) >= maxUploadFiles {
				return nil, fmt.Errorf("too many files (max %d)", maxUploadFiles)
			}
			if header.Size > fileSizeLimit {
				return nil, fmt.Errorf("file %s exceeds max size (%d bytes)", header.Filename, fileSizeLimit)
			}
			name := multipartFileName(header)
			name = sanitizeFilePath(name)
			if name == "" {
				return nil, fmt.Errorf("invalid file path %q", header.Filename)
			}
			if _, exists := files[name]; exists {
				return nil, fmt.Errorf("duplicate file path %q", name)
			}
			f, err := header.Open()
			if err != nil {
				return nil, fmt.Errorf("open file %s: %w", name, err)
			}
			data, err := io.ReadAll(io.LimitReader(f, fileSizeLimit+1))
			_ = f.Close()
			if err != nil {
				return nil, fmt.Errorf("read file %s: %w", name, err)
			}
			if int64(len(data)) > fileSizeLimit {
				return nil, fmt.Errorf("file %s exceeds max size (%d bytes)", name, fileSizeLimit)
			}
			files[name] = data
		}
	}
	return files, nil
}

func multipartFileName(fh *multipart.FileHeader) string {
	if cd := fh.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fn := params["filename"]; fn != "" {
				return fn
			}
		}
	}
	return fh.Filename
}

func sanitizeFilePath(name string) string {
	name = path.Clean(name)
	name = strings.TrimPrefix(name, "/")
	if name == "." || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") {
		return ""
	}
	return name
}

// resolveSkillRefWith is the shared SkillRef resolution logic usable by any
// service that has a SkillRepo + NamespaceService. nsSvc may be nil (qualified
// refs will fail).
func resolveSkillRefWith(ctx context.Context, ref model.SkillRef, skillRepo *repository.SkillRepo, nsSvc *NamespaceService) (*model.SkillWithOwner, error) {
	if ref.IsQualified() {
		if nsSvc == nil {
			return nil, nil
		}
		ns, err := nsSvc.GetBySlug(ctx, ref.Namespace)
		if err != nil || ns == nil {
			return nil, nil
		}
		return skillRepo.GetByNSAndSlug(ctx, ns.ID, ref.Slug)
	}

	all, err := skillRepo.GetBySlugGlobal(ctx, ref.Slug)
	if err != nil {
		return nil, err
	}
	switch len(all) {
	case 0:
		return skillRepo.GetBySlugOrAlias(ctx, ref.Slug)
	case 1:
		return &all[0], nil
	default:
		candidates := make([]AmbiguousCandidate, 0, len(all))
		for i := range all {
			candidates = append(candidates, AmbiguousCandidate{
				Namespace:   all[i].NamespaceSlug,
				Slug:        all[i].Slug,
				OwnerHandle: all[i].OwnerHandle,
				SkillID:     all[i].ID.String(),
			})
		}
		return nil, &AmbiguousSlugError{Slug: ref.Slug, Candidates: candidates}
	}
}

// canViewSkillWith is the shared visibility check used by CommentService,
// RatingService, and any service that doesn't own SkillService but needs
// to verify skill access. nsSvc may be nil (namespace member check skipped).
func canViewSkillWith(ctx context.Context, skill *model.SkillWithOwner, viewer *model.User, nsSvc *NamespaceService) bool {
	if skill.Visibility == "public" && skill.ModerationStatus == "approved" {
		return true
	}
	if viewer == nil {
		return false
	}
	if viewer.IsModerator() {
		return true
	}
	if skill.OwnerID == viewer.ID {
		return true
	}
	if skill.NamespaceID != nil && nsSvc != nil {
		if nsSvc.IsMemberOrAdmin(ctx, *skill.NamespaceID, viewer) {
			return true
		}
	}
	return false
}

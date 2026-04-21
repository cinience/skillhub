package store

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	osscredentials "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

// OSSConfig holds configuration for the Alibaba Cloud OSS storage backend.
type OSSConfig struct {
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`   // e.g. "cn-hangzhou"
	Prefix    string `yaml:"prefix"`   // object key prefix, default "skills"
	Endpoint  string `yaml:"endpoint"` // e.g. "oss-cn-hangzhou.aliyuncs.com"
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

// OSSBackend implements Store using Alibaba Cloud OSS.
type OSSBackend struct {
	client *oss.Client
	bucket string
	prefix string
}

func NewOSSBackend(cfg OSSConfig) (*OSSBackend, error) {
	ossCfg := oss.LoadDefaultConfig().
		WithRegion(cfg.Region)

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		provider := osscredentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey)
		ossCfg = ossCfg.WithCredentialsProvider(provider)
	}

	if cfg.Endpoint != "" {
		ossCfg = ossCfg.WithEndpoint(cfg.Endpoint)
	}

	client := oss.NewClient(ossCfg)

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "skills"
	}

	return &OSSBackend{
		client: client,
		bucket: cfg.Bucket,
		prefix: prefix,
	}, nil
}

func (b *OSSBackend) key(owner, slug, version, filePath string) string {
	return fmt.Sprintf("%s/%s/%s/versions/%s/files/%s", b.prefix, owner, slug, version, filePath)
}

func (b *OSSBackend) metaKey(owner, slug, version string) string {
	return fmt.Sprintf("%s/%s/%s/versions/%s/meta.json", b.prefix, owner, slug, version)
}

func (b *OSSBackend) versionPrefix(owner, slug string) string {
	return fmt.Sprintf("%s/%s/%s/versions/", b.prefix, owner, slug)
}

func (b *OSSBackend) Publish(ctx context.Context, opts PublishOpts) (string, error) {
	// Upload each file
	for filePath, content := range opts.Files {
		key := b.key(opts.Owner, opts.Slug, opts.Version, filePath)
		_, err := b.client.PutObject(ctx, &oss.PutObjectRequest{
			Bucket: &b.bucket,
			Key:    &key,
			Body:   bytes.NewReader(content),
		})
		if err != nil {
			return "", fmt.Errorf("upload %s: %w", filePath, err)
		}
	}

	// Write version metadata
	meta := publishMeta{
		Version:   opts.Version,
		Author:    opts.Author,
		Email:     opts.Email,
		Message:   opts.Message,
		Tags:      opts.Tags,
		CreatedAt: time.Now(),
	}
	metaBytes, _ := json.Marshal(meta)
	metaKey := b.metaKey(opts.Owner, opts.Slug, opts.Version)
	_, err := b.client.PutObject(ctx, &oss.PutObjectRequest{
		Bucket: &b.bucket,
		Key:    &metaKey,
		Body:   bytes.NewReader(metaBytes),
	})
	if err != nil {
		return "", fmt.Errorf("upload meta: %w", err)
	}

	return fmt.Sprintf("oss://%s/%s", b.bucket, metaKey), nil
}

func (b *OSSBackend) Archive(owner, slug, version string) (io.ReadCloser, error) {
	ctx := context.Background()
	prefix := b.key(owner, slug, version, "")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	var continuationToken *string
	for {
		out, err := b.client.ListObjectsV2(ctx, &oss.ListObjectsV2Request{
			Bucket:            &b.bucket,
			Prefix:            &prefix,
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}

		for _, obj := range out.Contents {
			relPath := strings.TrimPrefix(*obj.Key, prefix)
			if relPath == "" {
				continue
			}

			getOut, err := b.client.GetObject(ctx, &oss.GetObjectRequest{
				Bucket: &b.bucket,
				Key:    obj.Key,
			})
			if err != nil {
				return nil, fmt.Errorf("get object %s: %w", relPath, err)
			}

			data, err := io.ReadAll(io.LimitReader(getOut.Body, 10<<20)) // 10MB per file
			getOut.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("read object %s: %w", relPath, err)
			}

			archivePath := fmt.Sprintf("%s-%s/%s", slug, version, relPath)
			w, err := zw.Create(archivePath)
			if err != nil {
				return nil, fmt.Errorf("create zip entry: %w", err)
			}
			if _, err := w.Write(data); err != nil {
				return nil, fmt.Errorf("write zip entry: %w", err)
			}
		}

		if !out.IsTruncated {
			break
		}
		continuationToken = out.NextContinuationToken
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

func (b *OSSBackend) GetFile(owner, slug, version, path string) ([]byte, error) {
	ctx := context.Background()
	key := b.key(owner, slug, version, path)

	out, err := b.client.GetObject(ctx, &oss.GetObjectRequest{
		Bucket: &b.bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("get file %s: %w", path, err)
	}
	defer out.Body.Close()

	return io.ReadAll(io.LimitReader(out.Body, 10<<20)) // 10MB limit
}

func (b *OSSBackend) ListVersions(owner, slug string) ([]string, error) {
	ctx := context.Background()
	prefix := b.versionPrefix(owner, slug)
	delimiter := "/"

	var versions []string
	var continuationToken *string
	for {
		out, err := b.client.ListObjectsV2(ctx, &oss.ListObjectsV2Request{
			Bucket:            &b.bucket,
			Prefix:            &prefix,
			Delimiter:         &delimiter,
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list versions: %w", err)
		}

		for _, cp := range out.CommonPrefixes {
			ver := strings.TrimPrefix(*cp.Prefix, prefix)
			ver = strings.TrimSuffix(ver, "/")
			if ver != "" {
				versions = append(versions, ver)
			}
		}

		if !out.IsTruncated {
			break
		}
		continuationToken = out.NextContinuationToken
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareSemver(versions[i], versions[j]) > 0
	})

	return versions, nil
}

func (b *OSSBackend) Exists(owner, slug string) bool {
	ctx := context.Background()
	prefix := b.versionPrefix(owner, slug)
	maxKeys := int32(1)

	out, err := b.client.ListObjectsV2(ctx, &oss.ListObjectsV2Request{
		Bucket:  &b.bucket,
		Prefix:  &prefix,
		MaxKeys: maxKeys,
	})
	if err != nil {
		return false
	}
	return len(out.Contents) > 0
}

func (b *OSSBackend) Rename(owner, oldSlug, newSlug string) error {
	ctx := context.Background()
	oldPrefix := fmt.Sprintf("%s/%s/%s/", b.prefix, owner, oldSlug)
	newPrefix := fmt.Sprintf("%s/%s/%s/", b.prefix, owner, newSlug)

	var continuationToken *string
	for {
		out, err := b.client.ListObjectsV2(ctx, &oss.ListObjectsV2Request{
			Bucket:            &b.bucket,
			Prefix:            &oldPrefix,
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return fmt.Errorf("list objects for rename: %w", err)
		}

		for _, obj := range out.Contents {
			newKey := newPrefix + strings.TrimPrefix(*obj.Key, oldPrefix)
			copySource := fmt.Sprintf("/%s/%s", b.bucket, *obj.Key)

			// Copy to new key
			_, err := b.client.CopyObject(ctx, &oss.CopyObjectRequest{
				Bucket:       &b.bucket,
				Key:          &newKey,
				SourceKey:    &copySource,
			})
			if err != nil {
				return fmt.Errorf("copy %s: %w", *obj.Key, err)
			}

			// Delete old key
			_, err = b.client.DeleteObject(ctx, &oss.DeleteObjectRequest{
				Bucket: &b.bucket,
				Key:    obj.Key,
			})
			if err != nil {
				return fmt.Errorf("delete %s: %w", *obj.Key, err)
			}
		}

		if !out.IsTruncated {
			break
		}
		continuationToken = out.NextContinuationToken
	}

	return nil
}

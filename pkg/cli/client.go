package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	Token      string
	httpClient *http.Client
}

func NewClient(cfg *CLIConfig) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(cfg.Registry, "/"),
		Token:      cfg.Token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func NewClientFromConfig() (*Client, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return NewClient(cfg), nil
}

func (c *Client) do(method, path string, body io.Reader, contentType string) (*http.Response, error) {
	u := c.BaseURL + path
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("User-Agent", "skillhub-cli/1.0")
	return c.httpClient.Do(req)
}

func (c *Client) getJSON(path string, out interface{}) error {
	resp, err := c.do("GET", path, nil, "")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return parseAPIError(resp)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func parseAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
	}
	return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
}

// WhoAmI calls GET /api/v1/whoami
func (c *Client) WhoAmI() (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.getJSON("/api/v1/whoami", &result)
	return result, err
}

// Search calls GET /api/v1/search?q=...
func (c *Client) Search(query string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.getJSON("/api/v1/search?q="+url.QueryEscape(query), &result)
	return result, err
}

// ListSkills calls GET /api/v1/skills
func (c *Client) ListSkills(sort string, limit int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/api/v1/skills?sort=%s&limit=%d", url.QueryEscape(sort), limit)
	var result map[string]interface{}
	err := c.getJSON(path, &result)
	return result, err
}

// GetSkill calls GET /api/v1/skills/:slug
func (c *Client) GetSkill(slug string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.getJSON("/api/v1/skills/"+url.PathEscape(slug), &result)
	return result, err
}

// GetVersions calls GET /api/v1/skills/:slug/versions
func (c *Client) GetVersions(slug string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.getJSON("/api/v1/skills/"+url.PathEscape(slug)+"/versions", &result)
	return result, err
}

// Download calls GET /api/v1/download and returns the response body (ZIP)
func (c *Client) Download(slug, version string) (io.ReadCloser, error) {
	path := fmt.Sprintf("/api/v1/download?slug=%s&version=%s", url.QueryEscape(slug), url.QueryEscape(version))
	resp, err := c.do("GET", path, nil, "")
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, parseAPIError(resp)
	}
	return resp.Body, nil
}

// Publish calls POST /api/v1/skills with multipart form
func (c *Client) Publish(slug, version, summary, tags, changelog, category string, files map[string][]byte) (map[string]interface{}, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if slug != "" {
		writer.WriteField("slug", slug)
	}
	if version != "" {
		writer.WriteField("version", version)
	}
	if summary != "" {
		writer.WriteField("summary", summary)
	}
	if tags != "" {
		writer.WriteField("tags", tags)
	}
	if changelog != "" {
		writer.WriteField("changelog", changelog)
	}
	if category != "" {
		writer.WriteField("category", category)
	}

	for name, content := range files {
		part, err := writer.CreateFormFile("files", name)
		if err != nil {
			return nil, fmt.Errorf("creating form file: %w", err)
		}
		if _, err := part.Write(content); err != nil {
			return nil, fmt.Errorf("writing file content: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	resp, err := c.do("POST", "/api/v1/skills", &buf, writer.FormDataContentType())
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return result, nil
}

// ReadDirFiles reads all files from a directory path, returning a map of relative path -> content.
func ReadDirFiles(dirPath string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(info.Name(), ".") && path != dirPath {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip hidden files
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		rel, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", rel, err)
		}
		files[rel] = data
		return nil
	})
	return files, err
}

// Package agentplugin validates portable Agent Plugins packages at the
// registry trust boundary.
package agentplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/http/httpguts"
)

const (
	ManifestSchemaID = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
	MCPSchemaID      = "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"
)

var pluginNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)

// Manifest is the subset of validated metadata needed by SkillHub.
type Manifest struct {
	Name    string
	Version string
}

// ValidatePackage validates the portable package surface and returns its
// manifest metadata. Registry release versioning remains separate from the
// optional manifest version.
func ValidatePackage(files map[string][]byte) (*Manifest, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("plugin package is empty")
	}
	for name := range files {
		if err := ValidateFilePath(name); err != nil {
			return nil, err
		}
	}
	data, ok := files["plugin.json"]
	if !ok {
		return nil, fmt.Errorf("plugin.json is required at the package root")
	}
	manifest, err := ParseManifest(data)
	if err != nil {
		return nil, err
	}
	if data, ok := files["mcp.json"]; ok {
		if err := validateMCP(data); err != nil {
			return nil, err
		}
	}
	return manifest, nil
}

// ValidateFilePath rejects non-canonical and escaping package paths.
func ValidateFilePath(name string) error {
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") {
		return fmt.Errorf("invalid package path %q", name)
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != name {
		return fmt.Errorf("package path %q is not canonical and local", name)
	}
	return nil
}

// ParseManifest validates Agent Plugins Specification 1.0.0 plugin.json.
func ParseManifest(data []byte) (*Manifest, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return nil, fmt.Errorf("plugin.json must be a JSON object")
	}
	allowed := map[string]bool{
		"$schema": true, "name": true, "version": true, "description": true,
		"author": true, "homepage": true, "repository": true, "license": true,
		"keywords": true, "extensions": true,
	}
	for key := range raw {
		// Unknown top-level fields are report-and-ignore at client load time.
		if !allowed[key] {
			continue
		}
	}
	schema, err := requiredString(raw, "$schema")
	if err != nil || schema != ManifestSchemaID {
		return nil, fmt.Errorf("plugin.json: $schema must be %q", ManifestSchemaID)
	}
	name, err := requiredString(raw, "name")
	if err != nil {
		return nil, fmt.Errorf("plugin.json: %w", err)
	}
	if err := ValidateName(name); err != nil {
		return nil, fmt.Errorf("plugin.json: %w", err)
	}

	manifest := &Manifest{Name: name}
	for _, key := range []string{"version", "description", "homepage", "repository", "license"} {
		if value, ok := raw[key]; ok {
			s, err := stringValue(value, key)
			if err != nil {
				return nil, fmt.Errorf("plugin.json: %w", err)
			}
			if key == "version" {
				manifest.Version = s
			}
		}
	}
	if value, ok := raw["author"]; ok {
		if err := validateStringObject(value, "author", map[string]bool{"name": true, "email": true, "url": true}); err != nil {
			return nil, fmt.Errorf("plugin.json: %w", err)
		}
	}
	if value, ok := raw["keywords"]; ok {
		var keywords []string
		if json.Unmarshal(value, &keywords) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("plugin.json: keywords must be an array of strings")
		}
	}
	if value, ok := raw["extensions"]; ok && isObject(value) {
		var extensions map[string]json.RawMessage
		_ = json.Unmarshal(value, &extensions)
		for namespace, extension := range extensions {
			if !isObject(extension) {
				return nil, fmt.Errorf("plugin.json: extensions.%s must be an object", namespace)
			}
		}
	}
	// A non-object extensions value is intentionally report-and-ignore.
	return manifest, nil
}

// ValidateName implements §5.5.
func ValidateName(name string) error {
	count := utf8.RuneCountInString(name)
	if count < 1 || count > 64 || !pluginNamePattern.MatchString(name) || strings.Contains(name, "--") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid plugin name %q", name)
	}
	return nil
}

func validateMCP(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return fmt.Errorf("mcp.json must be a JSON object")
	}
	if len(raw) != 2 || raw["$schema"] == nil || raw["mcpServers"] == nil {
		return fmt.Errorf("mcp.json must contain only $schema and mcpServers")
	}
	schema, err := requiredString(raw, "$schema")
	if err != nil || schema != MCPSchemaID {
		return fmt.Errorf("mcp.json: $schema must be %q", MCPSchemaID)
	}
	var servers map[string]json.RawMessage
	if !isObject(raw["mcpServers"]) || json.Unmarshal(raw["mcpServers"], &servers) != nil {
		return fmt.Errorf("mcp.json: mcpServers must be an object")
	}
	for name, server := range servers {
		if err := validateServer(server); err != nil {
			return fmt.Errorf("mcp.json: server %q: %w", name, err)
		}
	}
	return nil
}

func validateServer(data json.RawMessage) error {
	var fields map[string]json.RawMessage
	if !isObject(data) || json.Unmarshal(data, &fields) != nil {
		return fmt.Errorf("entry must be an object")
	}
	kind, err := requiredString(fields, "type")
	if err != nil {
		return err
	}
	switch kind {
	case "stdio":
		if err := onlyFields(fields, "type", "command", "args", "env", "cwd"); err != nil {
			return err
		}
		command, err := requiredString(fields, "command")
		if err != nil {
			return err
		}
		if command == "" || strings.ContainsAny(command, " \t\r\n") || (!strings.HasPrefix(command, "./") && strings.ContainsAny(command, "/\\")) || command == "./" {
			return fmt.Errorf("command must be one bare or ./-relative executable token")
		}
		if args, ok := fields["args"]; ok {
			var v []string
			if json.Unmarshal(args, &v) != nil {
				return fmt.Errorf("args must be an array of strings")
			}
		}
		if env, ok := fields["env"]; ok {
			var v map[string]string
			if json.Unmarshal(env, &v) != nil || !isObject(env) {
				return fmt.Errorf("env must be an object of strings")
			}
			for key := range v {
				if strings.EqualFold(key, "PLUGIN_ROOT") || strings.EqualFold(key, "PLUGIN_DATA") {
					return fmt.Errorf("env %q is reserved", key)
				}
			}
		}
		if cwdRaw, ok := fields["cwd"]; ok {
			cwd, err := stringValue(cwdRaw, "cwd")
			if err != nil {
				return err
			}
			if !(strings.HasPrefix(cwd, "./") || cwd == "${PLUGIN_ROOT}" || strings.HasPrefix(cwd, "${PLUGIN_ROOT}/") || cwd == "${PLUGIN_DATA}" || strings.HasPrefix(cwd, "${PLUGIN_DATA}/")) {
				return fmt.Errorf("invalid cwd")
			}
		}
	case "streamable-http", "sse":
		if err := onlyFields(fields, "type", "url", "headers"); err != nil {
			return err
		}
		rawURL, err := requiredString(fields, "url")
		if err != nil {
			return err
		}
		if err := validateURL(rawURL); err != nil {
			return err
		}
		if headersRaw, ok := fields["headers"]; ok {
			var headers map[string]string
			if json.Unmarshal(headersRaw, &headers) != nil || !isObject(headersRaw) {
				return fmt.Errorf("headers must be an object of strings")
			}
			seen := map[string]bool{}
			for key, value := range headers {
				lower := strings.ToLower(key)
				if !validHeader(key, value) || seen[lower] {
					return fmt.Errorf("invalid or duplicate header %q", key)
				}
				seen[lower] = true
			}
		}
	default:
		return fmt.Errorf("unsupported transport %q", kind)
	}
	return nil
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" || u.Hostname() == "" {
		return fmt.Errorf("invalid MCP URL")
	}
	if u.Scheme == "http" {
		host := u.Hostname()
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return fmt.Errorf("non-loopback MCP URL must use HTTPS")
		}
	}
	return nil
}

func validHeader(name, value string) bool {
	return httpguts.ValidHeaderFieldName(name) && httpguts.ValidHeaderFieldValue(value)
}

func onlyFields(fields map[string]json.RawMessage, allowed ...string) error {
	set := map[string]bool{}
	for _, key := range allowed {
		set[key] = true
	}
	for key := range fields {
		if !set[key] {
			return fmt.Errorf("field %q is not permitted", key)
		}
	}
	return nil
}

func requiredString(raw map[string]json.RawMessage, key string) (string, error) {
	value, ok := raw[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	return stringValue(value, key)
}

func stringValue(raw json.RawMessage, key string) (string, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", key)
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return value, nil
}

func validateStringObject(raw json.RawMessage, name string, allowed map[string]bool) error {
	var fields map[string]json.RawMessage
	if !isObject(raw) || json.Unmarshal(raw, &fields) != nil {
		return fmt.Errorf("%s must be an object", name)
	}
	for key, value := range fields {
		if !allowed[key] {
			return fmt.Errorf("%s contains unknown field %q", name, key)
		}
		if _, err := stringValue(value, name+"."+key); err != nil {
			return err
		}
	}
	return nil
}

func isObject(raw json.RawMessage) bool {
	return len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '{'
}

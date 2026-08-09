package agentplugin

import "testing"

func TestValidatePackage(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string][]byte
		wantErr bool
	}{
		{name: "minimal", files: map[string][]byte{"plugin.json": []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"a"}`)}},
		{name: "period name and MCP", files: map[string][]byte{
			"plugin.json": []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"acme.tools"}`),
			"mcp.json":    []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"api":{"type":"streamable-http","url":"https://example.com/mcp"}}}`),
		}},
		{name: "legacy manifest", files: map[string][]byte{"plugin.json": []byte(`{"name":"old","version":"1.0.0"}`)}, wantErr: true},
		{name: "invalid name", files: map[string][]byte{"plugin.json": []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"bad_name"}`)}, wantErr: true},
		{name: "path traversal", files: map[string][]byte{"plugin.json": []byte(`{}`), "../escape": []byte("x")}, wantErr: true},
		{name: "invalid MCP entry", files: map[string][]byte{
			"plugin.json": []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"mcp"}`),
			"mcp.json":    []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"bad":{"type":"stdio","command":"sh -c bad"}}}`),
		}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidatePackage(tt.files)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePackageManifestVersionOptional(t *testing.T) {
	manifest, err := ValidatePackage(map[string][]byte{"plugin.json": []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"optional"}`)})
	if err != nil || manifest.Version != "" {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
}

package service

import (
	"fmt"
	"testing"

	"github.com/saker-ai/skillhub/pkg/agentplugin"
	"gorm.io/gorm"
)

func TestComputePluginFingerprint_Deterministic(t *testing.T) {
	files := map[string][]byte{
		"plugin.json":       []byte(`{"name":"test","version":"1.0.0"}`),
		"skills/a/SKILL.md": []byte("# skill a"),
	}

	fp1 := computeFingerprint(files)
	fp2 := computeFingerprint(files)

	if fp1 != fp2 {
		t.Errorf("fingerprint not deterministic: %s != %s", fp1, fp2)
	}
	if len(fp1) != 64 {
		t.Errorf("fingerprint length = %d, want 64 (sha256 hex)", len(fp1))
	}
}

func TestComputePluginFingerprint_DifferentFiles(t *testing.T) {
	files1 := map[string][]byte{
		"plugin.json": []byte(`{"name":"a","version":"1.0.0"}`),
	}
	files2 := map[string][]byte{
		"plugin.json": []byte(`{"name":"b","version":"1.0.0"}`),
	}

	fp1 := computeFingerprint(files1)
	fp2 := computeFingerprint(files2)

	if fp1 == fp2 {
		t.Error("different files should produce different fingerprints")
	}
}

func TestComputePluginFingerprint_OrderIndependent(t *testing.T) {
	files1 := map[string][]byte{
		"a.txt": []byte("aaa"),
		"b.txt": []byte("bbb"),
		"c.txt": []byte("ccc"),
	}
	files2 := map[string][]byte{
		"c.txt": []byte("ccc"),
		"a.txt": []byte("aaa"),
		"b.txt": []byte("bbb"),
	}

	fp1 := computeFingerprint(files1)
	fp2 := computeFingerprint(files2)

	if fp1 != fp2 {
		t.Errorf("fingerprint should be order-independent: %s != %s", fp1, fp2)
	}
}

func TestBuildFilesManifest(t *testing.T) {
	files := map[string][]byte{
		"plugin.json":       []byte(`{"name":"test"}`),
		"skills/a/SKILL.md": []byte("content"),
	}

	manifest := buildFilesManifest(files)
	if len(manifest) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(manifest))
	}

	// Should be sorted alphabetically
	if manifest[0].Path != "plugin.json" {
		t.Errorf("manifest[0].Path = %q, want %q", manifest[0].Path, "plugin.json")
	}
	if manifest[1].Path != "skills/a/SKILL.md" {
		t.Errorf("manifest[1].Path = %q", manifest[1].Path)
	}

	// Size should match
	if manifest[0].Size != len(`{"name":"test"}`) {
		t.Errorf("manifest[0].Size = %d, want %d", manifest[0].Size, len(`{"name":"test"}`))
	}

	// SHA256 should be non-empty
	if len(manifest[0].SHA256) != 64 {
		t.Errorf("manifest[0].SHA256 length = %d, want 64", len(manifest[0].SHA256))
	}
}

func TestValidatePluginManifest_Valid(t *testing.T) {
	manifest := []byte(`{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test-plugin",
		"version": "1.0.0"
	}`)
	files := map[string][]byte{
		"plugin.json":           manifest,
		"skills/greet/SKILL.md": []byte("# greet"),
	}

	_, err := agentplugin.ValidatePackage(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePluginManifest_MissingName(t *testing.T) {
	manifest := []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","version": "1.0.0"}`)
	files := map[string][]byte{"plugin.json": manifest}

	_, err := agentplugin.ValidatePackage(files)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestValidatePluginManifest_VersionOptional(t *testing.T) {
	manifest := []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name": "test"}`)
	files := map[string][]byte{"plugin.json": manifest}

	_, err := agentplugin.ValidatePackage(files)
	if err != nil {
		t.Fatalf("version must be optional: %v", err)
	}
}

func TestValidatePluginManifest_RejectsLegacyInlineComponents(t *testing.T) {
	manifest := []byte(`{
		"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test",
		"skills": {"entries": ["missing"]}
	}`)
	files := map[string][]byte{"plugin.json": manifest}

	parsed, err := agentplugin.ValidatePackage(files)
	if err != nil || parsed.Name != "test" {
		t.Fatalf("unknown top-level fields are report-and-ignore: parsed=%+v err=%v", parsed, err)
	}
}

func TestValidatePluginManifest_MCPFixedLocation(t *testing.T) {
	manifest := []byte(`{
		"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test"
	}`)
	files := map[string][]byte{
		"plugin.json": manifest,
		"mcp.json":    []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{}}`),
	}

	_, err := agentplugin.ValidatePackage(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePluginManifest_InvalidJSON(t *testing.T) {
	_, err := agentplugin.ParseManifest([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestComputeContentFingerprint_BackwardCompatible(t *testing.T) {
	files := map[string][]byte{
		"SKILL.md": []byte("# hello"),
		"lib.py":   []byte("print('hi')"),
	}
	// Content fingerprint must NOT change when files are renamed
	renamed := map[string][]byte{
		"README.md": []byte("# hello"),
		"main.py":   []byte("print('hi')"),
	}
	fp1 := computeContentFingerprint(files)
	fp2 := computeContentFingerprint(renamed)
	if fp1 != fp2 {
		t.Errorf("computeContentFingerprint should be name-independent: %s != %s", fp1, fp2)
	}

	// But computeFingerprint (plugin) MUST differ when names change
	pfp1 := computeFingerprint(files)
	pfp2 := computeFingerprint(renamed)
	if pfp1 == pfp2 {
		t.Error("computeFingerprint should differ when file names change")
	}

	// Both must be deterministic and 64 chars
	if fp1 != computeContentFingerprint(files) {
		t.Error("computeContentFingerprint not deterministic")
	}
	if len(fp1) != 64 {
		t.Errorf("length = %d, want 64", len(fp1))
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{gorm.ErrRecordNotFound, true},
		{fmt.Errorf("wrap: %w", gorm.ErrRecordNotFound), true},
		{fmt.Errorf("record not found"), false},
		{fmt.Errorf("other error"), false},
	}

	for _, tt := range tests {
		got := isNotFound(tt.err)
		if got != tt.want {
			t.Errorf("isNotFound(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

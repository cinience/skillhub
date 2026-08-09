package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/saker-ai/skillhub/pkg/cli"
)

func TestReadPluginDirFilesIncludesAgentPluginManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo"}`)
	mustWriteFile(t, filepath.Join(dir, ".git", "config"), "ignored")
	mustWriteFile(t, filepath.Join(dir, ".env"), "ignored")
	mustWriteFile(t, filepath.Join(dir, "skills", "demo", "SKILL.md"), "---\nname: demo\n---\n")

	files, err := cli.ReadPluginDirFiles(dir)
	if err != nil {
		t.Fatalf("ReadPluginDirFiles: %v", err)
	}
	if string(files["plugin.json"]) == "" {
		t.Fatalf("plugin.json was not included: %+v", files)
	}
	if string(files["skills/demo/SKILL.md"]) == "" {
		t.Fatalf("skills/demo/SKILL.md was not included: %+v", files)
	}
	if _, ok := files[".git/config"]; ok {
		t.Fatalf(".git/config should be ignored")
	}
	if _, ok := files[".env"]; ok {
		t.Fatalf(".env should be ignored")
	}
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

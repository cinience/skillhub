package cli

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallPluginArchiveAtomicValidation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "plugin")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "old.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	invalid := zipBytes(t, map[string]string{"plugin.json": `{}`})
	if err := installPluginArchive(invalid, target, nil); err == nil {
		t.Fatal("invalid package installed")
	}
	if data, err := os.ReadFile(filepath.Join(target, "old.txt")); err != nil || string(data) != "old" {
		t.Fatalf("existing install changed: %q %v", data, err)
	}

	valid := zipBytes(t, map[string]string{
		"plugin.json":          `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"atomic"}`,
		"skills/demo/SKILL.md": "---\nname: demo\ndescription: demo\n---\n",
	})
	if err := installPluginArchive(valid, target, []byte(`{"version":"1.0.0"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old file survived atomic replacement: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "demo", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestExtractZipRejectsTraversalAndSymlink(t *testing.T) {
	t.Run("traversal", func(t *testing.T) {
		if err := extractZipToDir(zipBytes(t, map[string]string{"../escape": "x"}), t.TempDir()); err == nil {
			t.Fatal("traversal accepted")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		header := &zip.FileHeader{Name: "link", Method: zip.Store}
		header.SetMode(os.ModeSymlink | 0o777)
		w, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("../target"))
		_ = zw.Close()
		if err := extractZipToDir(buf.Bytes(), t.TempDir()); err == nil {
			t.Fatal("symlink accepted")
		}
	})
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

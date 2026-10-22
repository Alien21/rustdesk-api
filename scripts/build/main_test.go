package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactDirectoryDefaultsAndCentralNamespace(t *testing.T) {
	source := t.TempDir()
	t.Setenv("RUSTDESK_ARTIFACTS_DIR", "")
	got, err := artifactDirectory(source)
	if err != nil || got != filepath.Join(source, "artifacts") {
		t.Fatalf("standalone artifacts = %q, %v", got, err)
	}
	base := t.TempDir()
	t.Setenv("RUSTDESK_ARTIFACTS_DIR", base)
	got, err = artifactDirectory(source)
	if err != nil || got != filepath.Join(base, "rustdesk-api") {
		t.Fatalf("central namespace = %q, %v", got, err)
	}
}

func TestArtifactDirectoryRejectsUnsafeRoots(t *testing.T) {
	source := t.TempDir()
	for _, base := range []string{"relative-artifacts", source} {
		t.Setenv("RUSTDESK_ARTIFACTS_DIR", base)
		if _, err := artifactDirectory(source); err == nil {
			t.Fatalf("unsafe base accepted: %s", base)
		}
	}
	base := t.TempDir()
	source = filepath.Join(base, "rustdesk-api", "source")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RUSTDESK_ARTIFACTS_DIR", base)
	if _, err := artifactDirectory(source); err == nil {
		t.Fatal("central namespace containing source was accepted")
	}
}

func TestArtifactDirectoryRejectsRedirectedBase(t *testing.T) {
	source := t.TempDir()
	parent := t.TempDir()
	base := filepath.Join(parent, "redirected")
	if err := os.Symlink(t.TempDir(), base); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	t.Setenv("RUSTDESK_ARTIFACTS_DIR", base)
	if _, err := artifactDirectory(source); err == nil {
		t.Fatal("redirected central base was accepted")
	}
}

package config

import (
	"path/filepath"
	"testing"
)

func TestEnvironmentOverridesFile(t *testing.T) {
	t.Setenv("VENDRA_CONFIG", "")
	t.Setenv("VENDRA_STATE_DIR", t.TempDir())
	t.Setenv("VENDRA_BASE_DOMAIN", "override.test")
	// Loads the implicit default path, which may be absent — the point here is
	// that environment variables win, not that a named file can be missing.
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseDomain != "override.test" {
		t.Fatalf("got %s", cfg.BaseDomain)
	}
}

func TestLoadRejectsAnExplicitConfigThatDoesNotExist(t *testing.T) {
	// Silently falling back to defaults gave the caller no images and the wrong
	// state_dir, surfacing much later as a Compose interpolation error.
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("expected an error for a --config path that does not exist")
	}
}

func TestLoadStillToleratesTheImplicitDefaultBeingAbsent(t *testing.T) {
	t.Setenv("VENDRA_CONFIG", "")
	if _, err := Load(""); err != nil {
		t.Fatalf("an absent default config must not be an error: %v", err)
	}
}

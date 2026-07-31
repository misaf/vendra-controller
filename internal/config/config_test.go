package config

import "testing"

func TestEnvironmentOverridesFile(t *testing.T) {
	t.Setenv("VENDRA_STATE_DIR", t.TempDir())
	t.Setenv("VENDRA_BASE_DOMAIN", "override.test")
	cfg, err := Load("/does/not/exist")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseDomain != "override.test" {
		t.Fatalf("got %s", cfg.BaseDomain)
	}
}

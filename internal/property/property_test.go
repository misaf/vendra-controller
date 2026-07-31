package property

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/misaf/vendra-controller/internal/config"
	"github.com/misaf/vendra-controller/internal/renderer"
)

func validSpec() Spec {
	return Spec{Slug: "acme-flowers", Domain: "acme.test", Image: "ghcr.io/misaf/storefront@sha256:abc", Theme: "default", ConfigurationBase64: base64.StdEncoding.EncodeToString([]byte(`{"slug":"acme-flowers","domain":"acme.test","theme":"default"}`))}
}
func TestRenderIsIdempotentAndPreservesUnknownEnvironment(t *testing.T) {
	cfg := config.Defaults()
	cfg.StateDir = t.TempDir()
	manager := Manager{Config: cfg, Renderer: renderer.Renderer{}}
	spec := validSpec()
	if err := manager.Render(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	env := filepath.Join(manager.Dir(spec.Slug), ".env")
	file, err := os.OpenFile(env, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.WriteString("CUSTOM=value\n")
	_ = file.Close()
	spec.Image = "ghcr.io/misaf/storefront@sha256:def"
	if err := manager.Render(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(env)
	text := string(data)
	if !contains(text, "CUSTOM=value") || !contains(text, spec.Image) {
		t.Fatalf("unexpected env: %s", text)
	}
}
func TestValidateRejectsMismatchedIdentity(t *testing.T) {
	spec := validSpec()
	spec.Domain = "other.test"
	if Validate(spec) == nil {
		t.Fatal("expected identity validation error")
	}
}
func contains(value, part string) bool { return len(value) >= len(part) && stringContains(value, part) }
func stringContains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}

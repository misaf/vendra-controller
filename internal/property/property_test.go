package property

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/misaf/vendra-controller/internal/config"
	"github.com/misaf/vendra-controller/internal/renderer"
)

// A configuration carrying every field the storefront image requires at boot.
// ogImage is intentionally empty: Vendra's console sends "" rather than omitting
// the key, and the image treats that as unset.
const validConfiguration = `{
	"slug": "acme-flowers", "domain": "acme.test", "theme": "default",
	"siteUrl": "https://acme.test", "businessType": "Florist",
	"priceCurrency": "IRR", "ogImage": "",
	"name": {"en": "Acme Flowers", "fa": "گل آکمه"},
	"address": {"locality": "Tehran", "country": "IR"},
	"contact": {
		"mobilePhone": "0912-0000000", "officePhone": "021-00000000",
		"email": "hello@acme.test", "hoursOpen": "08:00", "hoursClose": "21:00",
		"mapQuery": "35.7,51.4"
	},
	"social": {
		"whatsappPhone": "+989120000000", "telegramUsername": "acme",
		"instagramUsername": "acme"
	}
}`

func configurationWithout(t *testing.T, keys ...string) string {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(validConfiguration), &value); err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		delete(value, key)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(encoded)
}

func validSpec() Spec {
	return Spec{Slug: "acme-flowers", Domain: "acme.test", Image: "ghcr.io/misaf/storefront@sha256:abc", Theme: "default", ConfigurationBase64: base64.StdEncoding.EncodeToString([]byte(validConfiguration))}
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

// The quickstart used to document {"slug","domain"} as a whole configuration.
// That rendered fine and then crash-looped the container, so it must be rejected
// here instead.
func TestValidateRejectsIdentityOnlyConfiguration(t *testing.T) {
	spec := validSpec()
	spec.ConfigurationBase64 = base64.StdEncoding.EncodeToString(
		[]byte(`{"slug":"acme-flowers","domain":"acme.test"}`))
	err := Validate(spec)
	if err == nil {
		t.Fatal("expected missing-field validation error")
	}
	for _, field := range []string{"siteUrl", "businessType", "priceCurrency", "name", "address", "contact", "social"} {
		if !stringContains(err.Error(), field) {
			t.Errorf("error should name the missing field %q, got: %v", field, err)
		}
	}
}

func TestValidateNamesNestedMissingFields(t *testing.T) {
	spec := validSpec()
	spec.ConfigurationBase64 = base64.StdEncoding.EncodeToString([]byte(`{
		"slug":"acme-flowers","domain":"acme.test","theme":"default",
		"siteUrl":"https://acme.test","businessType":"Florist","priceCurrency":"IRR",
		"name":{"en":"Acme"},"address":{"locality":"Tehran"},
		"contact":{"mobilePhone":"1","officePhone":"2","email":"a@b.co","hoursOpen":"08:00","hoursClose":"21:00","mapQuery":"1,2"},
		"social":{"whatsappPhone":"+98","telegramUsername":"a","instagramUsername":"b"}}`))
	err := Validate(spec)
	if err == nil || !stringContains(err.Error(), "address.country") {
		t.Fatalf("expected address.country to be reported, got: %v", err)
	}
}

// ogImage is optional, and the console sends "" for it. Requiring it here would
// reject a configuration the storefront image accepts.
func TestValidateAcceptsEmptyAndAbsentOgImage(t *testing.T) {
	spec := validSpec()
	if err := Validate(spec); err != nil {
		t.Fatalf("empty ogImage should be accepted: %v", err)
	}
	spec.ConfigurationBase64 = configurationWithout(t, "ogImage")
	if err := Validate(spec); err != nil {
		t.Fatalf("absent ogImage should be accepted: %v", err)
	}
}

func TestRenderWritesTheImageHealthPath(t *testing.T) {
	cfg := config.Defaults()
	cfg.StateDir = t.TempDir()
	manager := Manager{Config: cfg, Renderer: renderer.Renderer{}}
	spec := validSpec()
	if err := manager.Render(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(manager.Dir(spec.Slug), ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "STOREFRONT_HEALTH_PATH=" + DefaultHealthPath; !stringContains(string(data), want) {
		t.Fatalf("expected %q in rendered env, got: %s", want, data)
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

func TestRegistryRecordsAndRemovesProperties(t *testing.T) {
	cfg := config.Defaults()
	cfg.StateDir = t.TempDir()
	manager := Manager{Config: cfg, Renderer: renderer.Renderer{}}
	spec := validSpec()
	if err := manager.Render(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	entries, err := manager.Registry()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Slug != spec.Slug || entries[0].Domain != spec.Domain {
		t.Fatalf("unexpected registry: %+v", entries)
	}
	// The registry must never carry the configuration: it exists to be copied
	// into backups, and the configuration is the property's private data.
	data, _ := os.ReadFile(manager.RegistryPath())
	if stringContains(string(data), spec.ConfigurationBase64) {
		t.Fatal("registry leaked the encoded configuration")
	}
	// Re-rendering updates in place rather than appending a duplicate.
	spec.Image = "ghcr.io/misaf/storefront@sha256:def"
	if err := manager.Render(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	entries, _ = manager.Registry()
	if len(entries) != 1 || entries[0].Image != spec.Image {
		t.Fatalf("expected one updated entry, got %+v", entries)
	}
	if err := manager.DeregisterProperty(spec.Slug); err != nil {
		t.Fatal(err)
	}
	if entries, _ = manager.Registry(); len(entries) != 0 {
		t.Fatalf("expected empty registry, got %+v", entries)
	}
}

func TestFleetReportsDriftBothWays(t *testing.T) {
	cfg := config.Defaults()
	cfg.StateDir = t.TempDir()
	manager := Manager{Config: cfg, Renderer: renderer.Renderer{}}
	spec := validSpec()
	if err := manager.Render(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	// Registered but never rendered on this host.
	if err := manager.RegisterProperty(Spec{Slug: "ghost", Domain: "ghost.test", Image: "img", Theme: "default"}); err != nil {
		t.Fatal(err)
	}
	// Rendered but absent from the registry — a restore would skip it silently.
	if err := os.MkdirAll(manager.Dir("orphan"), 0o755); err != nil {
		t.Fatal(err)
	}
	fleet, err := manager.Fleet()
	if err != nil {
		t.Fatal(err)
	}
	state := map[string][2]bool{}
	for _, entry := range fleet {
		state[entry.Slug] = [2]bool{entry.Registry, entry.Rendered}
	}
	for slug, want := range map[string][2]bool{
		spec.Slug: {true, true}, "ghost": {true, false}, "orphan": {false, true},
	} {
		if state[slug] != want {
			t.Errorf("%s: got registry/rendered %v, want %v", slug, state[slug], want)
		}
	}
}

func TestSyncReportsPropertiesWithNoStoredConfiguration(t *testing.T) {
	cfg := config.Defaults()
	cfg.StateDir = t.TempDir()
	manager := Manager{Config: cfg, Renderer: renderer.Renderer{}}
	if err := manager.Render(context.Background(), validSpec()); err != nil {
		t.Fatal(err)
	}
	if err := manager.RegisterProperty(Spec{Slug: "ghost", Domain: "ghost.test", Image: "img", Theme: "default"}); err != nil {
		t.Fatal(err)
	}
	unresolved, err := manager.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(unresolved) != 1 || unresolved[0] != "ghost" {
		t.Fatalf("expected ghost to be unresolved, got %v", unresolved)
	}
}

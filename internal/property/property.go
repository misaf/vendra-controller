package property

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/misaf/vendra-controller/internal/certificates"
	"github.com/misaf/vendra-controller/internal/compose"
	"github.com/misaf/vendra-controller/internal/config"
	"github.com/misaf/vendra-controller/internal/docker"
	"github.com/misaf/vendra-controller/internal/envfile"
	"github.com/misaf/vendra-controller/internal/renderer"
)

// DefaultHealthPath is the storefront image's dedicated health endpoint. It
// reports the server process only, so a check against it cannot be satisfied by
// an unrelated page rendering — which is what "/" would allow.
const DefaultHealthPath = "/api/health"

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var domainPattern = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+)$`)

type Spec struct{ Slug, Domain, Image, Theme, ConfigurationBase64 string }
type Manager struct {
	Config   config.Config
	Docker   docker.Service
	Renderer renderer.Renderer
	// NoPull skips `docker compose pull` so a locally built storefront image can
	// be started without a registry. See Controller.NoPull.
	NoPull bool
}

func Validate(spec Spec) error {
	if !slugPattern.MatchString(spec.Slug) {
		return errors.New("slug must contain lowercase letters, digits, and hyphens")
	}
	if !domainPattern.MatchString(spec.Domain) {
		return errors.New("domain is invalid")
	}
	if strings.TrimSpace(spec.Image) == "" {
		return errors.New("image is required")
	}
	if spec.Theme == "" {
		spec.Theme = "default"
	}
	// A theme is a property of the *image*: the storefront resolves it at build
	// time from the bundled property, so a different theme means a different
	// image — which `--image` already supports per property. The controller
	// therefore records the theme for inventory and checks the configuration
	// agrees with the request, but cannot change it at deploy time.
	//
	// Only "default" ships today. When a second theme exists, widen this list
	// and publish an image containing it; nothing else here needs to change.
	if spec.Theme != "default" {
		return errors.New(`unsupported theme: only "default" is published today`)
	}
	if spec.ConfigurationBase64 == "" {
		return errors.New("configuration is required")
	}
	decoded, err := base64.StdEncoding.DecodeString(spec.ConfigurationBase64)
	if err != nil {
		return errors.New("configuration_base64 is invalid")
	}
	var value map[string]any
	if json.Unmarshal(decoded, &value) != nil {
		return errors.New("configuration_base64 must contain JSON")
	}
	if value["slug"] != spec.Slug || value["domain"] != spec.Domain {
		return errors.New("configuration identity does not match property")
	}
	return validateConfiguration(value)
}

// Required storefront configuration fields, mirroring properties/schema.json in
// the storefront image.
//
// Deliberately absent: ogImage. It is optional there, and Vendra's console sends
// an empty string rather than omitting the key when a property has no share
// image, so requiring it here would reject a configuration the image accepts.
var (
	requiredStrings = []string{"slug", "theme", "domain", "siteUrl", "businessType", "priceCurrency"}
	requiredObjects = map[string][]string{
		"name":    {},
		"address": {"locality", "country"},
		"contact": {"mobilePhone", "officePhone", "email", "hoursOpen", "hoursClose", "mapQuery"},
		"social":  {"whatsappPhone", "telegramUsername", "instagramUsername"},
	}
)

// validateConfiguration checks the decoded storefront configuration against the
// fields the image requires at boot.
//
// The container refuses to render without them, so validating only slug and
// domain let an unusable property render and then crash-loop. Failing here turns
// that into a rejected API call with a specific field name.
func validateConfiguration(value map[string]any) error {
	var missing []string

	for _, key := range requiredStrings {
		if text, ok := value[key].(string); !ok || strings.TrimSpace(text) == "" {
			missing = append(missing, key)
		}
	}

	for _, key := range slices.Sorted(maps.Keys(requiredObjects)) {
		nested, ok := value[key].(map[string]any)
		if !ok || len(nested) == 0 {
			missing = append(missing, key)
			continue
		}
		for _, field := range requiredObjects[key] {
			if text, ok := nested[field].(string); !ok || strings.TrimSpace(text) == "" {
				missing = append(missing, key+"."+field)
			}
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("configuration is missing required fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (m Manager) Add(ctx context.Context, spec Spec) error {
	dir := m.Dir(spec.Slug)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("property %s already exists", spec.Slug)
	}
	return m.Render(ctx, spec)
}
func (m Manager) Render(_ context.Context, spec Spec) error {
	if spec.Theme == "" {
		spec.Theme = "default"
	}
	if err := Validate(spec); err != nil {
		return err
	}
	dir := m.Dir(spec.Slug)
	if err := m.Renderer.Project("property", dir); err != nil {
		return err
	}
	if err := envfile.Upsert(filepath.Join(dir, ".env"), map[string]string{"DOMAIN": spec.Domain, "ROUTER_NAME": spec.Slug, "BASE_DOMAIN": m.Config.BaseDomain, "STOREFRONT_IMAGE": spec.Image, "STOREFRONT_CONFIG_BASE64": spec.ConfigurationBase64, "STOREFRONT_PORT": "3000", "STOREFRONT_HEALTH_PATH": DefaultHealthPath, "CERT_RESOLVER": resolver(m.Config), "VENDRA_STATE_DIR": m.Config.StateDir, "STOREFRONT_CA_FILE": certificateAuthority(m.Config)}); err != nil {
		return err
	}
	return m.RegisterProperty(spec)
}

// Sync re-renders every registered property from its recorded identity.
//
// The registry holds no configuration by design, so this recovers the ones whose
// .env survives and reports the rest: their configuration lives in Vendra's
// storefront_deployments table, and only re-provisioning through the provisioner
// can restore it. Returns the slugs it could not rebuild.
func (m Manager) Sync(ctx context.Context) ([]string, error) {
	entries, err := m.Registry()
	if err != nil {
		return nil, err
	}
	var unresolved []string
	for _, entry := range entries {
		values, err := envfile.Read(filepath.Join(m.Dir(entry.Slug), ".env"))
		if err != nil {
			return nil, err
		}
		configuration := values["STOREFRONT_CONFIG_BASE64"]
		if configuration == "" {
			unresolved = append(unresolved, entry.Slug)
			continue
		}
		spec := Spec{Slug: entry.Slug, Domain: entry.Domain, Image: entry.Image, Theme: entry.Theme, ConfigurationBase64: configuration}
		if err := m.Render(ctx, spec); err != nil {
			return nil, fmt.Errorf("sync %s: %w", entry.Slug, err)
		}
	}
	return unresolved, nil
}
func (m Manager) Up(ctx context.Context, slug string) error {
	if err := m.Docker.EnsureNetwork(ctx, m.Config.Network); err != nil {
		return err
	}
	p, err := m.Project(slug)
	if err != nil {
		return err
	}
	if !m.NoPull {
		if err = p.Pull(ctx, "web"); err != nil {
			return err
		}
	}
	return p.Up(ctx, true)
}
func (m Manager) Down(ctx context.Context, slug string) error {
	p, err := m.Project(slug)
	if err != nil {
		return err
	}
	return p.Down(ctx)
}
func (m Manager) Restart(ctx context.Context, slug string) error {
	p, err := m.Project(slug)
	if err != nil {
		return err
	}
	return p.Restart(ctx)
}
func (m Manager) Remove(ctx context.Context, slug string) error {
	p, err := m.Project(slug)
	if err != nil {
		return err
	}
	_ = p.Down(ctx)
	dir := m.Dir(slug)
	if filepath.Dir(dir) != m.Config.PropertiesDir() {
		return errors.New("unsafe property path")
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return m.DeregisterProperty(slug)
}
func (m Manager) Dir(slug string) string { return filepath.Join(m.Config.PropertiesDir(), slug) }
func (m Manager) Project(slug string) (compose.Project, error) {
	if !slugPattern.MatchString(slug) {
		return compose.Project{}, errors.New("invalid slug")
	}
	dir := m.Dir(slug)
	if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err != nil {
		return compose.Project{}, fmt.Errorf("property %s is not rendered", slug)
	}
	return compose.Project{Docker: m.Docker, Dir: dir, Name: slug, EnvFile: filepath.Join(dir, ".env"), Files: []string{filepath.Join(dir, "docker-compose.yml")}, NoPull: m.NoPull}, nil
}

// certificateAuthority is the in-container path the storefront's Node runtime
// trusts, or empty when the system roots suffice.
//
// It is the same certificate Traefik serves, mounted read-only: under
// certificate_mode: self-signed nothing public signs it, so without this every
// server-side call the storefront makes to the API is rejected before it is
// sent.
func certificateAuthority(cfg config.Config) string {
	if cfg.CertificateMode == "self-signed" {
		return "/certs/" + certificates.CAFileName
	}
	return ""
}

func resolver(cfg config.Config) string {
	if cfg.CertificateMode == "acme" {
		return "letsencrypt"
	}
	return ""
}

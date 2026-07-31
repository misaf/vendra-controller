package property

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/misaf/vendra-controller/internal/compose"
	"github.com/misaf/vendra-controller/internal/config"
	"github.com/misaf/vendra-controller/internal/docker"
	"github.com/misaf/vendra-controller/internal/envfile"
	"github.com/misaf/vendra-controller/internal/renderer"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var domainPattern = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+)$`)

type Spec struct{ Slug, Domain, Image, Theme, ConfigurationBase64 string }
type Manager struct {
	Config   config.Config
	Docker   docker.Service
	Renderer renderer.Renderer
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
	if spec.Theme != "default" {
		return errors.New("unsupported theme")
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
	return envfile.Upsert(filepath.Join(dir, ".env"), map[string]string{"DOMAIN": spec.Domain, "ROUTER_NAME": spec.Slug, "BASE_DOMAIN": m.Config.BaseDomain, "STOREFRONT_IMAGE": spec.Image, "STOREFRONT_CONFIG_BASE64": spec.ConfigurationBase64, "STOREFRONT_THEME": spec.Theme, "STOREFRONT_PORT": "3000", "STOREFRONT_HEALTH_PATH": "/", "CERT_RESOLVER": resolver(m.Config)})
}
func (m Manager) Up(ctx context.Context, slug string) error {
	if err := m.Docker.EnsureNetwork(ctx, m.Config.Network); err != nil {
		return err
	}
	p, err := m.Project(slug)
	if err != nil {
		return err
	}
	if err = p.Pull(ctx, "web"); err != nil {
		return err
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
	return os.RemoveAll(dir)
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
	return compose.Project{Docker: m.Docker, Dir: dir, Name: slug, EnvFile: filepath.Join(dir, ".env"), Files: []string{filepath.Join(dir, "docker-compose.yml")}}, nil
}
func resolver(cfg config.Config) string {
	if cfg.CertificateMode == "acme" {
		return "letsencrypt"
	}
	return ""
}

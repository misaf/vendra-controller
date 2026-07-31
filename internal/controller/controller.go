package controller

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/misaf/vendra-controller/internal/certificates"
	"github.com/misaf/vendra-controller/internal/compose"
	"github.com/misaf/vendra-controller/internal/config"
	"github.com/misaf/vendra-controller/internal/docker"
	"github.com/misaf/vendra-controller/internal/envfile"
	"github.com/misaf/vendra-controller/internal/hosts"
	"github.com/misaf/vendra-controller/internal/property"
	"github.com/misaf/vendra-controller/internal/renderer"
	"github.com/misaf/vendra-controller/internal/urls"
)

type Controller struct {
	Config     config.Config
	Docker     docker.Service
	Renderer   renderer.Renderer
	Properties property.Manager
}

func New(cfg config.Config, service docker.Service) *Controller {
	r := renderer.Renderer{}
	return &Controller{Config: cfg, Docker: service, Renderer: r, Properties: property.Manager{Config: cfg, Docker: service, Renderer: r}}
}
func (c *Controller) Init() error {
	for _, dir := range []string{c.Config.RuntimeDir(), c.Config.PropertiesDir(), c.Config.CertificatesDir(), filepath.Join(c.Config.StateDir, "acme")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	acmePath := filepath.Join(c.Config.StateDir, "acme", "acme.json")
	if _, err := os.Stat(acmePath); os.IsNotExist(err) {
		if err := os.WriteFile(acmePath, []byte("{}"), 0o600); err != nil {
			return err
		}
	}
	return c.RenderStack()
}
func (c *Controller) RenderStack() error {
	for _, name := range []string{"proxy", "platform", "website"} {
		if err := c.Renderer.Project(name, filepath.Join(c.Config.RuntimeDir(), name)); err != nil {
			return err
		}
	}
	values := map[string]string{"BASE_DOMAIN": c.Config.BaseDomain, "VENDRA_STATE_DIR": c.Config.StateDir, "VENDRA_PLATFORM_IMAGE": c.Config.Images.Platform, "VENDRA_WEBSITE_IMAGE": c.Config.Images.Website, "VENDRA_PROVISIONER_IMAGE": c.Config.Images.Provisioner, "ACME_EMAIL": c.Config.ACMEEmail}
	for _, key := range []string{"DB_DATABASE", "DB_USERNAME", "DB_PASSWORD", "DB_ROOT_PASSWORD"} {
		if value := os.Getenv(key); value != "" {
			values[key] = value
		}
	}
	if err := envfile.Upsert(filepath.Join(c.Config.RuntimeDir(), "proxy", ".env"), values); err != nil {
		return err
	}
	if err := envfile.Upsert(filepath.Join(c.Config.RuntimeDir(), "website", ".env"), values); err != nil {
		return err
	}
	if err := envfile.Upsert(filepath.Join(c.Config.RuntimeDir(), "platform", ".env"), values); err != nil {
		return err
	}
	platform := map[string]string{}
	for _, key := range []string{"APP_KEY", "DB_DATABASE", "DB_USERNAME", "DB_PASSWORD", "DB_ROOT_PASSWORD", "REDIS_PASSWORD", "MAIL_MAILER", "MAIL_HOST", "MAIL_PORT", "STOREFRONT_PROVISIONER_TOKEN"} {
		if value := os.Getenv(key); value != "" {
			platform[key] = value
		}
	}
	platform["VENDRA_PROVISIONER_TOKEN"] = c.Config.ProvisionerToken
	platform["VENDRA_BASE_DOMAIN"] = c.Config.BaseDomain
	platform["APP_URL"] = "https://" + c.Config.BaseDomain
	return envfile.Upsert(filepath.Join(c.Config.RuntimeDir(), "platform", "platform.env"), platform)
}
func (c *Controller) project(name string) compose.Project {
	dir := filepath.Join(c.Config.RuntimeDir(), name)
	return compose.Project{Docker: c.Docker, Dir: dir, Name: name, EnvFile: filepath.Join(dir, ".env"), Files: []string{filepath.Join(dir, "docker-compose.yml")}}
}
func (c *Controller) Up(ctx context.Context) error {
	if err := c.Docker.Available(ctx); err != nil {
		return err
	}
	if err := c.Init(); err != nil {
		return err
	}
	if err := c.Docker.EnsureNetwork(ctx, c.Config.Network); err != nil {
		return err
	}
	if c.Config.CertificateMode == "self-signed" {
		if err := c.RegenerateCertificate(); err != nil {
			return err
		}
	}
	for _, name := range []string{"proxy", "platform", "website"} {
		p := c.project(name)
		if err := p.Validate(ctx); err != nil {
			return err
		}
		if err := p.Pull(ctx); err != nil {
			return err
		}
		if err := p.Up(ctx, true); err != nil {
			return err
		}
	}
	return nil
}
func (c *Controller) Down(ctx context.Context) error {
	for _, slug := range c.PropertySlugs() {
		_ = c.Properties.Down(ctx, slug)
	}
	for _, name := range []string{"website", "platform", "proxy"} {
		_ = c.project(name).Down(ctx)
	}
	return nil
}
func (c *Controller) Restart(ctx context.Context) error {
	if err := c.Down(ctx); err != nil {
		return err
	}
	return c.Up(ctx)
}
func (c *Controller) PS(ctx context.Context) error {
	for _, name := range []string{"proxy", "platform", "website"} {
		fmt.Printf("%s:\n", name)
		if err := c.project(name).PS(ctx); err != nil {
			return err
		}
	}
	for _, slug := range c.PropertySlugs() {
		fmt.Printf("property %s:\n", slug)
		p, _ := c.Properties.Project(slug)
		if err := p.PS(ctx); err != nil {
			return err
		}
	}
	return nil
}
func (c *Controller) Logs(ctx context.Context, target string, stdin io.Reader) error {
	switch target {
	case "proxy":
		return c.project("proxy").Logs(ctx, stdin, "")
	case "website":
		return c.project("website").Logs(ctx, stdin, "")
	}
	if p, err := c.Properties.Project(target); err == nil {
		return p.Logs(ctx, stdin, "")
	}
	return c.project("platform").Logs(ctx, stdin, target)
}
func (c *Controller) URLs() map[string]string { return urls.Platform(c.Config.BaseDomain) }
func (c *Controller) PropertySlugs() []string {
	entries, _ := os.ReadDir(c.Config.PropertiesDir())
	var slugs []string
	for _, entry := range entries {
		if entry.IsDir() {
			slugs = append(slugs, entry.Name())
		}
	}
	sort.Strings(slugs)
	return slugs
}
func (c *Controller) PropertyDomains() map[string]string {
	result := map[string]string{}
	for _, slug := range c.PropertySlugs() {
		file, err := os.Open(filepath.Join(c.Config.PropertiesDir(), slug, ".env"))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if value, ok := strings.CutPrefix(scanner.Text(), "DOMAIN="); ok {
				result[slug] = value
			}
		}
		file.Close()
	}
	return result
}
func (c *Controller) HostNames() []string {
	return hosts.Names(c.Config.BaseDomain, c.PropertyDomains())
}
func (c *Controller) RegenerateCertificate() error {
	domains := []string{}
	for _, domain := range c.PropertyDomains() {
		domains = append(domains, domain)
	}
	return certificates.Generate(c.Config.CertificatesDir(), c.Config.BaseDomain, domains)
}

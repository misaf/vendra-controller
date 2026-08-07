package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Images struct {
	Platform    string `yaml:"platform"`
	Website     string `yaml:"website"`
	Storefront  string `yaml:"storefront"`
	Provisioner string `yaml:"provisioner"`
}

type Config struct {
	StateDir         string        `yaml:"state_dir"`
	BaseDomain       string        `yaml:"base_domain"`
	Network          string        `yaml:"network"`
	Listen           string        `yaml:"listen"`
	HealthTimeout    time.Duration `yaml:"-"`
	HealthTimeoutRaw string        `yaml:"health_timeout"`
	CertificateMode  string        `yaml:"certificate_mode"`
	ACMEEmail        string        `yaml:"acme_email"`
	Images           Images        `yaml:"images"`
	ProvisionerToken string        `yaml:"-"`
	// NoPull uses images already in the local Docker daemon instead of pulling
	// them. `compose pull` fails outright on an image that exists only locally,
	// so without this a locally built storefront can be rendered but never
	// started — which the provisioner surfaces only as a failed deployment.
	//
	// The CLI's --no-pull flag sets the same behaviour per invocation; the
	// provisioner is a long-running server with no flags, so config is its only
	// channel.
	NoPull bool `yaml:"no_pull"`
}

func Defaults() Config {
	return Config{
		StateDir: "/var/lib/vendra", BaseDomain: "vendra.test", Network: "traefik-public",
		Listen: ":8080", HealthTimeout: 2 * time.Minute, CertificateMode: "self-signed",
	}
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	loadEnvironmentFile("/etc/vendra/controller.env")
	if path == "" {
		path = os.Getenv("VENDRA_CONFIG")
	}
	if path == "" {
		path = "/etc/vendra/controller.yaml"
	}
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("decode config %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	override(&cfg.StateDir, "VENDRA_STATE_DIR")
	override(&cfg.BaseDomain, "VENDRA_BASE_DOMAIN")
	override(&cfg.Network, "VENDRA_NETWORK")
	override(&cfg.Listen, "VENDRA_PROVISIONER_LISTEN")
	override(&cfg.CertificateMode, "VENDRA_CERTIFICATE_MODE")
	override(&cfg.ACMEEmail, "VENDRA_ACME_EMAIL")
	override(&cfg.Images.Platform, "VENDRA_PLATFORM_IMAGE")
	override(&cfg.Images.Website, "VENDRA_WEBSITE_IMAGE")
	override(&cfg.Images.Storefront, "VENDRA_STOREFRONT_IMAGE")
	override(&cfg.Images.Provisioner, "VENDRA_PROVISIONER_IMAGE")
	cfg.ProvisionerToken = os.Getenv("VENDRA_PROVISIONER_TOKEN")
	if err := overrideBool(&cfg.NoPull, "VENDRA_NO_PULL"); err != nil {
		return Config{}, err
	}
	if cfg.HealthTimeoutRaw != "" {
		d, err := time.ParseDuration(cfg.HealthTimeoutRaw)
		if err != nil {
			return Config{}, fmt.Errorf("health_timeout: %w", err)
		}
		cfg.HealthTimeout = d
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadEnvironmentFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if found && os.Getenv(key) == "" {
			_ = os.Setenv(key, strings.Trim(value, `"'`))
		}
	}
}

func override(target *string, key string) {
	if value := os.Getenv(key); value != "" {
		*target = value
	}
}

// overrideBool rejects an unparseable value rather than treating it as false: a
// typo in VENDRA_NO_PULL would otherwise silently restore pulling, and the only
// symptom is a deployment that fails on an image the daemon already has.
func overrideBool(target *bool, key string) error {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	*target = parsed
	return nil
}

func (c Config) Validate() error {
	if !filepath.IsAbs(c.StateDir) {
		return errors.New("state_dir must be absolute")
	}
	if strings.TrimSpace(c.BaseDomain) == "" {
		return errors.New("base_domain is required")
	}
	if strings.TrimSpace(c.Network) == "" {
		return errors.New("network is required")
	}
	if c.CertificateMode != "self-signed" && c.CertificateMode != "acme" {
		return errors.New("certificate_mode must be self-signed or acme")
	}
	if c.CertificateMode == "acme" && strings.TrimSpace(c.ACMEEmail) == "" {
		return errors.New("acme_email is required in acme certificate mode")
	}
	return nil
}

func (c Config) RuntimeDir() string      { return filepath.Join(c.StateDir, "runtime") }
func (c Config) PropertiesDir() string   { return filepath.Join(c.StateDir, "properties") }
func (c Config) CertificatesDir() string { return filepath.Join(c.StateDir, "certificates") }

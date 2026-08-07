package property

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/misaf/vendra-controller/internal/filesystem"
	"gopkg.in/yaml.v3"
)

// RegistryFileName is the fleet registry, written beside the rendered property
// directories in the state dir.
//
// Those directories are generated, and a property's .env holds its encoded
// configuration and therefore its secrets. The registry deliberately holds
// neither: slug, domain, image and theme only, so it is safe to copy into a
// backup or a runbook, and it is the one small file that answers "what was
// running on this host" after the host is gone.
const RegistryFileName = "properties.yml"

// Entry is one property's durable identity. No configuration, no secrets.
type Entry struct {
	Slug   string `yaml:"slug"`
	Domain string `yaml:"domain"`
	Image  string `yaml:"image"`
	Theme  string `yaml:"theme"`
}

type registryFile struct {
	Properties []Entry `yaml:"properties"`
}

func (m Manager) RegistryPath() string {
	return filepath.Join(m.Config.StateDir, RegistryFileName)
}

// Registry returns every recorded property, ordered by slug.
func (m Manager) Registry() ([]Entry, error) {
	data, err := os.ReadFile(m.RegistryPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var file registryFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse %s: %w", RegistryFileName, err)
	}
	sort.Slice(file.Properties, func(i, j int) bool {
		return file.Properties[i].Slug < file.Properties[j].Slug
	})
	return file.Properties, nil
}

func (m Manager) writeRegistry(entries []Entry) error {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Slug < entries[j].Slug })
	data, err := yaml.Marshal(registryFile{Properties: entries})
	if err != nil {
		return err
	}
	header := "# Vendra storefront fleet registry — generated, do not edit by hand.\n" +
		"# Slug, domain, image and theme only: no secrets, so this file is safe to\n" +
		"# back up. `vendra property sync` re-renders every entry onto a fresh host.\n"
	return filesystem.AtomicWrite(m.RegistryPath(), append([]byte(header), data...), 0o644)
}

// RegisterProperty records or updates one property, keyed by slug.
func (m Manager) RegisterProperty(spec Spec) error {
	entries, err := m.Registry()
	if err != nil {
		return err
	}
	entry := Entry{Slug: spec.Slug, Domain: spec.Domain, Image: spec.Image, Theme: spec.Theme}
	for i, existing := range entries {
		if existing.Slug == spec.Slug {
			entries[i] = entry
			return m.writeRegistry(entries)
		}
	}
	return m.writeRegistry(append(entries, entry))
}

// DeregisterProperty drops one property from the registry. Absent is not an error.
func (m Manager) DeregisterProperty(slug string) error {
	entries, err := m.Registry()
	if err != nil {
		return err
	}
	kept := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Slug != slug {
			kept = append(kept, entry)
		}
	}
	if len(kept) == len(entries) {
		return nil
	}
	return m.writeRegistry(kept)
}

// Drift is one disagreement between the registry and what is on disk.
type Drift struct {
	Slug     string
	Registry bool // present in properties.yml
	Rendered bool // a directory exists under properties/
	Domain   string
	Image    string
	Theme    string
}

// Fleet reports every property known to either side, so drift is visible in both
// directions: registered but never rendered on this host, and rendered but
// missing from the registry (which a restore would silently skip).
func (m Manager) Fleet() ([]Drift, error) {
	entries, err := m.Registry()
	if err != nil {
		return nil, err
	}
	state := map[string]*Drift{}
	for _, entry := range entries {
		state[entry.Slug] = &Drift{
			Slug: entry.Slug, Registry: true,
			Domain: entry.Domain, Image: entry.Image, Theme: entry.Theme,
		}
	}
	dirs, err := os.ReadDir(m.Config.PropertiesDir())
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		if found, ok := state[dir.Name()]; ok {
			found.Rendered = true
			continue
		}
		state[dir.Name()] = &Drift{Slug: dir.Name(), Rendered: true}
	}
	slugs := make([]string, 0, len(state))
	for slug := range state {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	fleet := make([]Drift, 0, len(slugs))
	for _, slug := range slugs {
		fleet = append(fleet, *state[slug])
	}
	return fleet, nil
}

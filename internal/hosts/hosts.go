package hosts

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/misaf/vendra-controller/internal/filesystem"
)

const begin = "# BEGIN VENDRA"
const end = "# END VENDRA"

func Names(base string, properties map[string]string) []string {
	names := []string{base, "admin." + base, "console." + base, "reseller." + base, "api." + base, "traefik." + base}
	for slug, domain := range properties {
		names = append(names, domain, "www."+domain, slug+".admin."+base)
	}
	sort.Strings(names)
	return names
}
func Block(names []string) string {
	return begin + "\n127.0.0.1 " + strings.Join(names, " ") + "\n" + end + "\n"
}
func Write(path string, names []string) error {
	data, _ := os.ReadFile(path)
	text := string(data)
	if start := strings.Index(text, begin); start >= 0 {
		if stop := strings.Index(text[start:], end); stop >= 0 {
			text = text[:start] + text[start+stop+len(end):]
		}
	}
	text = strings.TrimRight(text, "\n") + "\n" + Block(names)
	if err := filesystem.AtomicWrite(path, []byte(text), 0o644); err != nil {
		return fmt.Errorf("write hosts file: %w", err)
	}
	return nil
}

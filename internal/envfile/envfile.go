package envfile

import (
	"os"
	"strings"

	"github.com/misaf/vendra-controller/internal/filesystem"
)

// Read parses an env file into a map. A missing file yields an empty map rather
// than an error: callers use this to recover values that may not exist yet.
func Read(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if key, value, found := strings.Cut(line, "="); found {
			values[strings.TrimSpace(key)] = value
		}
	}
	return values, nil
}

func Upsert(path string, values map[string]string) error {
	data, _ := os.ReadFile(path)
	remaining := make(map[string]string, len(values))
	for k, v := range values {
		remaining[k] = v
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(data) == 0 {
		lines = nil
	}
	for i, line := range lines {
		key, _, found := strings.Cut(line, "=")
		if found {
			if value, ok := remaining[key]; ok {
				lines[i] = key + "=" + value
				delete(remaining, key)
			}
		}
	}
	keys := []string{"DOMAIN", "ROUTER_NAME", "BASE_DOMAIN", "STOREFRONT_IMAGE", "STOREFRONT_CONFIG_BASE64", "STOREFRONT_PORT", "STOREFRONT_HEALTH_PATH", "CERT_RESOLVER"}
	for _, key := range keys {
		if value, ok := remaining[key]; ok {
			lines = append(lines, key+"="+value)
			delete(remaining, key)
		}
	}
	for key, value := range remaining {
		lines = append(lines, key+"="+value)
	}
	return filesystem.AtomicWrite(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

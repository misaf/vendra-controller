package assets

import (
	"strings"
	"testing"
)

func TestDockerSocketIsRestrictedToProxyAndProvisioner(t *testing.T) {
	proxy, _ := Files.ReadFile("compose/proxy/docker-compose.yml")
	platform, _ := Files.ReadFile("compose/platform/docker-compose.yml")
	if !strings.Contains(string(proxy), `"/var/run/docker.sock:/var/run/docker.sock:ro"`) {
		t.Fatal("proxy socket must be read-only")
	}
	if !strings.Contains(string(platform), `"/var/run/docker.sock:/var/run/docker.sock"`) || strings.Contains(string(platform), "docker.sock:ro") {
		t.Fatal("only provisioner may have write-capable socket")
	}
}

package provisioner

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/misaf/vendra-controller/internal/config"
	"github.com/misaf/vendra-controller/internal/controller"
	"github.com/misaf/vendra-controller/internal/docker"
)

func TestCapabilitiesAreAuthenticatedAndVersioned(t *testing.T) {
	cfg := config.Defaults()
	cfg.StateDir = t.TempDir()
	server := New(controller.New(cfg, docker.Service{}), "secret", "1.2.3", slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, test := range []struct {
		token  string
		status int
	}{{"", 401}, {"Bearer secret", 200}} {
		request := httptest.NewRequest(http.MethodGet, "/v1/capabilities", nil)
		request.Header.Set("Authorization", test.token)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("got %d", response.Code)
		}
	}
}
func TestStorefrontRejectsUnknownFields(t *testing.T) {
	cfg := config.Defaults()
	cfg.StateDir = t.TempDir()
	server := New(controller.New(cfg, docker.Service{}), "secret", "dev", slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodPost, "/v1/storefronts", io.NopCloser(io.LimitReader(&repeatingReader{}, MaxBodyBytes+1)))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", response.Code)
	}
}

type repeatingReader struct{}

func (*repeatingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

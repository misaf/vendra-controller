package provisioner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/misaf/vendra-controller/internal/auth"
	"github.com/misaf/vendra-controller/internal/controller"
	"github.com/misaf/vendra-controller/internal/property"
	"github.com/misaf/vendra-controller/pkg/api"
)

const MaxBodyBytes int64 = 1 << 20

type Server struct {
	Controller     *controller.Controller
	Token, Version string
	mu             sync.Mutex
	logger         *slog.Logger
}

func New(c *controller.Controller, token, version string, logger *slog.Logger) *Server {
	return &Server{Controller: c, Token: token, Version: version, logger: logger}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		write(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("GET /v1/capabilities", auth.Bearer(s.Token, http.HandlerFunc(s.capabilities)))
	mux.Handle("POST /v1/storefronts", auth.Bearer(s.Token, http.HandlerFunc(s.storefront)))
	return mux
}
func (s *Server) capabilities(w http.ResponseWriter, _ *http.Request) {
	write(w, http.StatusOK, api.Capabilities{APIVersion: "v1", Version: s.Version, Templates: []string{"vendra-storefront-florist"}, Themes: []string{"default"}, MaxBodyBytes: MaxBodyBytes})
}
func (s *Server) storefront(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request api.StorefrontRequest
	if err := decoder.Decode(&request); err != nil {
		write(w, http.StatusUnprocessableEntity, api.ErrorResponse{Error: "invalid request body"})
		return
	}
	spec := property.Spec{Slug: strings.ToLower(request.Slug), Domain: strings.ToLower(request.Domain), Image: request.Image, Theme: request.Theme, ConfigurationBase64: request.ConfigurationBase64}
	if err := validateRequest(request, spec); err != nil {
		write(w, http.StatusUnprocessableEntity, api.ErrorResponse{Error: err.Error()})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), s.Controller.Config.HealthTimeout)
	defer cancel()
	if err := s.Controller.Properties.Render(ctx, spec); err != nil {
		s.fail(w, err)
		return
	}
	if s.Controller.Config.CertificateMode == "self-signed" {
		if err := s.Controller.RegenerateCertificate(); err != nil {
			s.fail(w, err)
			return
		}
	}
	if err := s.Controller.Properties.Up(ctx, spec.Slug); err != nil {
		s.fail(w, err)
		return
	}
	digest := ""
	if before, after, ok := strings.Cut(request.Image, "@sha256:"); ok && before != "" {
		digest = "sha256:" + after
	}
	write(w, http.StatusOK, api.StorefrontResponse{Status: "ready", Reference: spec.Slug, ImageDigest: digest})
}
func validateRequest(request api.StorefrontRequest, spec property.Spec) error {
	if request.Template != "vendra-storefront-florist" {
		return errors.New("unsupported template")
	}
	if err := property.Validate(spec); err != nil {
		return err
	}
	decoded, err := base64.StdEncoding.DecodeString(request.ConfigurationBase64)
	if err != nil {
		return err
	}
	var value map[string]any
	if err = json.Unmarshal(decoded, &value); err != nil {
		return err
	}
	if value["theme"] != request.Theme {
		return errors.New("configuration theme does not match request")
	}
	return nil
}
func (s *Server) fail(w http.ResponseWriter, err error) {
	s.logger.Error("provisioning failed", "error", err)
	write(w, http.StatusInternalServerError, api.ErrorResponse{Error: "provisioning failed"})
}
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func HTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 3 * time.Minute, IdleTimeout: 60 * time.Second}
}
func Healthcheck(address string) error {
	address = strings.TrimPrefix(address, ":")
	response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/health", address))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health status %d", response.StatusCode)
	}
	return nil
}

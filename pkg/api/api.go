package api

import "encoding/json"

type StorefrontRequest struct {
	Template            string         `json:"template"`
	Image               string         `json:"image"`
	TenantID            int64          `json:"tenant_id"`
	Slug                string         `json:"slug"`
	Domain              string         `json:"domain"`
	Theme               string         `json:"theme"`
	Configuration       map[string]any `json:"configuration"`
	ConfigurationBase64 string         `json:"configuration_base64"`
}
type StorefrontResponse struct {
	Status      string `json:"status"`
	Reference   string `json:"reference"`
	ImageDigest string `json:"image_digest"`
}
type ErrorResponse struct {
	Error string `json:"error"`
}
type Capabilities struct {
	APIVersion   string   `json:"api_version"`
	Version      string   `json:"version"`
	Templates    []string `json:"templates"`
	Themes       []string `json:"themes"`
	MaxBodyBytes int64    `json:"max_body_bytes"`
}

func (r StorefrontRequest) ConfigurationJSON() ([]byte, error) { return json.Marshal(r.Configuration) }

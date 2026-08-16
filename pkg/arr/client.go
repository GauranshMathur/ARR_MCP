// Package arr provides HTTP clients for the services in an *arr media stack.
package arr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AuthKind selects how credentials are attached to outbound requests.
type AuthKind int

const (
	// AuthHeaderKey sends the API key in a service-specific header.
	AuthHeaderKey AuthKind = iota
	// AuthBasic sends HTTP basic credentials.
	AuthBasic
	// AuthNone sends no credentials.
	AuthNone
)

// ServiceSpec describes how one service exposes its API. Keeping the base path
// and auth scheme here rather than in the shared client is what lets services
// with different API versions and headers share a single transport.
type ServiceSpec struct {
	// Name identifies the service in logs and errors.
	Name string
	// BasePath prefixes every request path, e.g. "/api/v3".
	BasePath string
	// StatusPath is the health endpoint, relative to BasePath.
	StatusPath string
	// Auth selects the credential scheme.
	Auth AuthKind
	// AuthHeader names the header for AuthHeaderKey; defaults to X-Api-Key.
	AuthHeader string
}

// Specs for the services this build supports.
var (
	// SonarrSpec describes Sonarr's v3 API.
	SonarrSpec = ServiceSpec{Name: "sonarr", BasePath: "/api/v3", StatusPath: "/system/status", Auth: AuthHeaderKey}
	// RadarrSpec describes Radarr's v3 API.
	RadarrSpec = ServiceSpec{Name: "radarr", BasePath: "/api/v3", StatusPath: "/system/status", Auth: AuthHeaderKey}
	// ProwlarrSpec describes Prowlarr's v1 API, which differs from Sonarr/Radarr.
	ProwlarrSpec = ServiceSpec{Name: "prowlarr", BasePath: "/api/v1", StatusPath: "/system/status", Auth: AuthHeaderKey}
)

// Credentials carries the secrets for whichever auth scheme a spec selects.
type Credentials struct {
	APIKey   string
	Username string
	Password string
}

// Query is a set of URL query parameters.
type Query map[string]string

// Client performs authenticated HTTP requests against one service instance.
type Client struct {
	baseURL string
	spec    ServiceSpec
	creds   Credentials
	http    *http.Client
}

// NewClient creates a client for a single instance of the service in spec.
func NewClient(baseURL string, spec ServiceSpec, creds Credentials) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		spec:    spec,
		creds:   creds,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Spec returns the service description this client was built from.
func (c *Client) Spec() ServiceSpec { return c.spec }

// resolve builds an absolute URL, preserving any subpath in the configured base
// URL so services behind a reverse-proxy prefix keep working.
func (c *Client) resolve(path string, q Query) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid %s url %q: %w", c.spec.Name, c.baseURL, err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + c.spec.BasePath + "/" + strings.TrimLeft(path, "/")

	if len(q) > 0 {
		values := u.Query()
		for k, v := range q {
			values.Set(k, v)
		}
		u.RawQuery = values.Encode()
	}
	return u.String(), nil
}

// authorize attaches credentials according to the service spec.
func (c *Client) authorize(req *http.Request) {
	switch c.spec.Auth {
	case AuthHeaderKey:
		header := c.spec.AuthHeader
		if header == "" {
			header = "X-Api-Key"
		}
		req.Header.Set(header, c.creds.APIKey)
	case AuthBasic:
		req.SetBasicAuth(c.creds.Username, c.creds.Password)
	case AuthNone:
	}
}

// redact removes credentials from text bound for logs or model-visible errors.
func (c *Client) redact(s string) string {
	for _, secret := range []string{c.creds.APIKey, c.creds.Password} {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "***")
		}
	}
	return s
}

// do performs a request and returns the response body.
func (c *Client) do(ctx context.Context, method, path string, body any, q Query) ([]byte, error) {
	target, err := c.resolve(path, q)
	if err != nil {
		return nil, err
	}

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding %s request body: %w", c.spec.Name, err)
		}
		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, payload)
	if err != nil {
		return nil, fmt.Errorf("building %s request: %w", c.spec.Name, err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.authorize(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %s", c.spec.Name, c.redact(err.Error()))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s response: %w", c.spec.Name, err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s returned %d: %s",
			c.spec.Name, resp.StatusCode, c.redact(strings.TrimSpace(string(respBody))))
	}
	return respBody, nil
}

// Get performs a GET request with optional query parameters.
func (c *Client) Get(ctx context.Context, path string, q ...Query) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, nil, first(q))
}

// Post performs a POST request with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) ([]byte, error) {
	return c.do(ctx, http.MethodPost, path, body, nil)
}

// Put performs a PUT request with a JSON body.
func (c *Client) Put(ctx context.Context, path string, body any) ([]byte, error) {
	return c.do(ctx, http.MethodPut, path, body, nil)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string, q ...Query) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, path, nil, first(q))
}

// Ping checks that the instance is reachable and the credentials work.
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := c.Get(ctx, c.spec.StatusPath)
	return err
}

// first returns the single optional Query, if present.
func first(q []Query) Query {
	if len(q) == 0 {
		return nil
	}
	return q[0]
}

// GetJSON performs a GET and decodes the response into out.
func GetJSON[T any](ctx context.Context, c *Client, path string, q ...Query) (T, error) {
	var out T
	body, err := c.Get(ctx, path, q...)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("decoding %s response from %s: %w", c.spec.Name, path, err)
	}
	return out, nil
}

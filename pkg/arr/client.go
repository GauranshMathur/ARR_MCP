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
	"sync"
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
	// AuthSession logs in with username and password once, then replays the
	// session cookie the service issued. qBittorrent's WebUI works this way.
	AuthSession
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
	// Only the name is meaningful: net/http canonicalises header casing, so a
	// spec cannot request a differently-cased spelling of the same name.
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
	// BazarrSpec describes Bazarr, which serves /api rather than a versioned
	// path. It accepts the canonical X-Api-Key header, so no override is needed.
	BazarrSpec = ServiceSpec{
		Name: "bazarr", BasePath: "/api", StatusPath: "/system/status",
		Auth: AuthHeaderKey,
	}
	// QBittorrentSpec describes qBittorrent's WebUI API v2, which issues a
	// session cookie from a form login instead of accepting an API key.
	QBittorrentSpec = ServiceSpec{Name: "qbittorrent", BasePath: "/api/v2", StatusPath: "/app/version", Auth: AuthSession}
	// NZBGetSpec describes NZBGet's JSON-RPC endpoint. The base path is empty
	// because every call goes to /jsonrpc, which doubles as the status path
	// when suffixed with a method name.
	NZBGetSpec = ServiceSpec{Name: "nzbget", BasePath: "", StatusPath: "/jsonrpc/version", Auth: AuthBasic}
)

// defaultTimeout bounds ordinary reads and fire-and-forget commands.
const defaultTimeout = 30 * time.Second

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
		http:    &http.Client{Timeout: defaultTimeout},
	}
}

// Spec returns the service description this client was built from.
func (c *Client) Spec() ServiceSpec { return c.spec }

// WithTimeout returns a copy of the client using a different request timeout,
// for calls that legitimately run longer than a read. The original is unchanged.
func (c *Client) WithTimeout(d time.Duration) *Client {
	clone := *c
	clone.http = &http.Client{Timeout: d}
	return &clone
}

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

// authorize attaches credentials according to the service spec. Session auth
// is handled by transmit, because it needs a login round-trip first.
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
	case AuthNone, AuthSession:
	}
}

// session is the cached login state for one instance. Sessions are cached per
// instance rather than per Client because the server builds a fresh Client for
// every tool call; without the cache each call would log in again.
type session struct {
	mu       sync.Mutex
	sid      string
	loggedIn bool
}

// sessions holds session state keyed by instance URL and username.
var sessions sync.Map

// session returns the cached login state for this client's instance.
func (c *Client) session() *session {
	key := c.baseURL + "\x00" + c.creds.Username
	v, _ := sessions.LoadOrStore(key, &session{})
	return v.(*session)
}

// ensureSession returns a session id, logging in if none is cached. When stale
// names the id that was just rejected, it logs in again unless another caller
// already replaced it, so concurrent 403s trigger one login rather than many.
func (c *Client) ensureSession(ctx context.Context, stale string, retry bool) (string, error) {
	s := c.session()
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loggedIn && (!retry || s.sid != stale) {
		return s.sid, nil
	}
	sid, err := c.login(ctx)
	if err != nil {
		s.loggedIn = false
		return "", err
	}
	s.sid, s.loggedIn = sid, true
	return sid, nil
}

// formContentType is the body encoding qBittorrent expects for every POST.
const formContentType = "application/x-www-form-urlencoded"

// login performs the form login and returns the SID cookie. A service that
// bypasses authentication for this client answers Ok. without a cookie, which
// is accepted as an empty session.
func (c *Client) login(ctx context.Context) (string, error) {
	target, err := c.resolve("/auth/login", nil)
	if err != nil {
		return "", err
	}
	form := url.Values{"username": {c.creds.Username}, "password": {c.creds.Password}}
	resp, err := c.send(ctx, http.MethodPost, target, []byte(form.Encode()), formContentType, "")
	if err != nil {
		return "", err
	}
	if resp.status >= 400 {
		return "", fmt.Errorf("%s login returned %d: %s",
			c.spec.Name, resp.status, c.redact(strings.TrimSpace(string(resp.body))))
	}
	if strings.TrimSpace(string(resp.body)) != "Ok." {
		return "", fmt.Errorf("%s login failed: check username and password", c.spec.Name)
	}
	for _, ck := range resp.cookies {
		if ck.Name == "SID" {
			return ck.Value, nil
		}
	}
	return "", nil
}

// response is what send returns before status handling.
type response struct {
	status  int
	body    []byte
	cookies []*http.Cookie
}

// send performs one HTTP exchange. sid, when non-empty, is replayed as the
// session cookie; for session-authenticated services the Referer and Origin
// headers are set because qBittorrent rejects requests without them.
func (c *Client) send(ctx context.Context, method, target string, payload []byte, contentType, sid string) (*response, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("building %s request: %w", c.spec.Name, err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", contentType)
	}
	c.authorize(req)
	if c.spec.Auth == AuthSession {
		req.Header.Set("Referer", c.baseURL)
		req.Header.Set("Origin", c.baseURL)
		if sid != "" {
			// Set the header rather than building an http.Cookie: Secure,
			// HttpOnly and SameSite are response attributes a server sets, and
			// are never serialised on an outbound request cookie, so a Cookie
			// value here only invites a false "insecure cookie" report.
			req.Header.Set("Cookie", "SID="+sid)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %s", c.spec.Name, c.redact(err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s response: %w", c.spec.Name, err)
	}
	return &response{status: resp.StatusCode, body: respBody, cookies: resp.Cookies()}, nil
}

// minRedactable is the shortest credential worth removing from an error.
// Redaction is a substring replace, so a very short value rewrites unrelated
// words: a one-character key once turned "context deadline exceeded" into
// "conte***t deadline e***ceeded". Nothing this short is a real *arr API key,
// and a corrupted error message is worse than not masking a value that carries
// no secrecy anyway.
const minRedactable = 8

// redact removes credentials from text bound for logs or model-visible errors.
func (c *Client) redact(s string) string {
	for _, secret := range []string{c.creds.APIKey, c.creds.Password} {
		if len(secret) >= minRedactable {
			s = strings.ReplaceAll(s, secret, "***")
		}
	}
	return s
}

// do performs a request with an optional JSON body and returns the response body.
func (c *Client) do(ctx context.Context, method, path string, body any, q Query) ([]byte, error) {
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding %s request body: %w", c.spec.Name, err)
		}
		payload = encoded
	}
	return c.transmit(ctx, method, path, payload, "application/json", q)
}

// transmit resolves the path, attaches session state when the spec needs it,
// and turns error statuses into errors. A session-authenticated request that
// comes back 403 logs in again and is retried once, because qBittorrent
// expires sessions silently.
func (c *Client) transmit(ctx context.Context, method, path string, payload []byte, contentType string, q Query) ([]byte, error) {
	target, err := c.resolve(path, q)
	if err != nil {
		return nil, err
	}

	var sid string
	if c.spec.Auth == AuthSession {
		if sid, err = c.ensureSession(ctx, "", false); err != nil {
			return nil, err
		}
	}

	resp, err := c.send(ctx, method, target, payload, contentType, sid)
	if err != nil {
		return nil, err
	}
	if resp.status == http.StatusForbidden && c.spec.Auth == AuthSession {
		if sid, err = c.ensureSession(ctx, sid, true); err != nil {
			return nil, err
		}
		if resp, err = c.send(ctx, method, target, payload, contentType, sid); err != nil {
			return nil, err
		}
	}

	if resp.status >= 400 {
		return nil, fmt.Errorf("%s returned %d: %s",
			c.spec.Name, resp.status, c.redact(strings.TrimSpace(string(resp.body))))
	}
	return resp.body, nil
}

// PostForm performs a POST with a form-encoded body, which is how qBittorrent
// expresses every mutation.
func (c *Client) PostForm(ctx context.Context, path string, form url.Values) ([]byte, error) {
	return c.transmit(ctx, http.MethodPost, path, []byte(form.Encode()), formContentType, nil)
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

// Patch performs a PATCH request driven by query parameters, which is how
// Bazarr expresses its mutations.
func (c *Client) Patch(ctx context.Context, path string, q Query) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, path, nil, q)
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

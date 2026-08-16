package arr

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// capture records what the fake upstream received.
type capture struct {
	path   string
	method string
	header http.Header
	query  string
	method string
	body   string
}

// fakeService returns a test server recording the last request, plus the capture.
func fakeService(t *testing.T, status int, body string) (*httptest.Server, *capture) {
	t.Helper()
	got := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.method = r.Method
		got.header = r.Header.Clone()
		got.query = r.URL.RawQuery
		got.method = r.Method
		sent, _ := io.ReadAll(r.Body)
		got.body = string(sent)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func TestClientSendsAPIKeyHeaderFromSpec(t *testing.T) {
	srv, got := fakeService(t, 200, `{}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "secret"})

	if _, err := c.Get(context.Background(), "/series"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if v := got.header.Get("X-Api-Key"); v != "secret" {
		t.Errorf("X-Api-Key = %q, want %q", v, "secret")
	}
}

func TestClientPrefixesServiceBasePath(t *testing.T) {
	srv, got := fakeService(t, 200, `[]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	if _, err := c.Get(context.Background(), "/series"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.path != "/api/v3/series" {
		t.Errorf("path = %q, want %q", got.path, "/api/v3/series")
	}
}

// Prowlarr serves /api/v1, not /api/v3. The old shared client hardcoded v3 for
// health checks, so Prowlarr always reported unhealthy.
func TestProwlarrUsesV1BasePath(t *testing.T) {
	srv, got := fakeService(t, 200, `[]`)
	c := NewClient(srv.URL, ProwlarrSpec, Credentials{APIKey: "k"})

	if _, err := c.Get(context.Background(), "/indexer"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.path != "/api/v1/indexer" {
		t.Errorf("path = %q, want %q", got.path, "/api/v1/indexer")
	}
}

func TestProwlarrHealthCheckHitsV1StatusPath(t *testing.T) {
	srv, got := fakeService(t, 200, `{"version":"1.0"}`)
	c := NewClient(srv.URL, ProwlarrSpec, Credentials{APIKey: "k"})

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
	if got.path != "/api/v1/system/status" {
		t.Errorf("health path = %q, want %q", got.path, "/api/v1/system/status")
	}
}

// A service behind a reverse proxy subpath must keep that prefix. The old client
// assigned to url.Path directly, discarding it.
func TestClientPreservesBaseURLSubpath(t *testing.T) {
	srv, got := fakeService(t, 200, `[]`)
	c := NewClient(srv.URL+"/sonarr", SonarrSpec, Credentials{APIKey: "k"})

	if _, err := c.Get(context.Background(), "/series"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.path != "/sonarr/api/v3/series" {
		t.Errorf("path = %q, want %q", got.path, "/sonarr/api/v3/series")
	}
}

func TestClientTrimsTrailingSlashOnBaseURL(t *testing.T) {
	srv, got := fakeService(t, 200, `[]`)
	c := NewClient(srv.URL+"/", SonarrSpec, Credentials{APIKey: "k"})

	if _, err := c.Get(context.Background(), "/series"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.path != "/api/v3/series" {
		t.Errorf("path = %q, want %q", got.path, "/api/v3/series")
	}
}

func TestClientEncodesQueryParameters(t *testing.T) {
	srv, got := fakeService(t, 200, `[]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	if _, err := c.Get(context.Background(), "/series/lookup", Query{"term": "Mr. Robot & Co"}); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !strings.Contains(got.query, "term=Mr.+Robot+%26+Co") {
		t.Errorf("query = %q, want URL-encoded term", got.query)
	}
}

func TestClientErrorIncludesStatusAndBody(t *testing.T) {
	srv, _ := fakeService(t, 401, `{"message":"Unauthorized"}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "wrong"})

	_, err := c.Get(context.Background(), "/series")
	if err == nil {
		t.Fatal("expected an error for 401 response, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error %q does not include the status code", err)
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("error %q does not include the response body", err)
	}
}

func TestClientRedactsAPIKeyFromErrors(t *testing.T) {
	srv, _ := fakeService(t, 500, `boom`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "super-secret-key"})

	_, err := c.Get(context.Background(), "/series")
	if err == nil {
		t.Fatal("expected an error for 500 response, got nil")
	}
	if strings.Contains(err.Error(), "super-secret-key") {
		t.Errorf("error leaks the API key: %q", err)
	}
}

func TestBasicAuthCredentialsAreApplied(t *testing.T) {
	srv, got := fakeService(t, 200, `{}`)
	spec := ServiceSpec{Name: "nzbget", BasePath: "/jsonrpc", StatusPath: "/status", Auth: AuthBasic}
	c := NewClient(srv.URL, spec, Credentials{Username: "nzb", Password: "pw"})

	if _, err := c.Get(context.Background(), "/version"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	user, pass, ok := parseBasic(got.header.Get("Authorization"))
	if !ok || user != "nzb" || pass != "pw" {
		t.Errorf("basic auth = (%q,%q,%v), want (nzb,pw,true)", user, pass, ok)
	}
}

// parseBasic decodes an Authorization header for assertions.
func parseBasic(h string) (string, string, bool) {
	r, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	r.Header.Set("Authorization", h)
	return r.BasicAuth()
}

package arr

import (
	"context"
	"errors"
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
	body   string
	// paths accumulates every request, for calls that fan out over ids.
	paths []string
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
		got.paths = append(got.paths, r.Method+" "+r.URL.Path)
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

// Redaction is a substring replace, so a very short credential would rewrite
// unrelated words -- a one-character key turned "context deadline exceeded"
// into "conte***t deadline e***ceeded". Real keys are long; short ones are not
// worth corrupting error messages for.
func TestRedactIgnoresImplausiblyShortSecrets(t *testing.T) {
	c := NewClient("http://x", SonarrSpec, Credentials{APIKey: "x"})

	got := c.redact("context deadline exceeded")
	if got != "context deadline exceeded" {
		t.Errorf("redact mangled the message: %q", got)
	}
}

func TestRedactStillHidesRealLengthSecrets(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	c := NewClient("http://x", SonarrSpec, Credentials{APIKey: key})

	got := c.redact("request to /api?apikey=" + key + " failed")
	if strings.Contains(got, key) {
		t.Errorf("redact leaked the API key: %q", got)
	}
	if !strings.Contains(got, "***") {
		t.Errorf("redact did not mark the removal: %q", got)
	}
}

func TestRedactHidesPasswords(t *testing.T) {
	c := NewClient("http://x", ServiceSpec{Name: "nzb", BasePath: "/api", Auth: AuthBasic},
		Credentials{Username: "u", Password: "correct-horse-battery"})

	if got := c.redact("auth failed for correct-horse-battery"); strings.Contains(got, "correct-horse-battery") {
		t.Errorf("redact leaked the password: %q", got)
	}
}

// Callers that need to branch on the status must not have to match on the
// message text, which nothing stops a later change from rewording.
func TestErrorStatusIsInspectable(t *testing.T) {
	srv, _ := fakeService(t, http.StatusBadRequest, `[{"errorMessage":"no such host"}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "secret"})

	_, err := c.Get(context.Background(), "/series")
	if err == nil {
		t.Fatal("expected an error for a 400 response")
	}

	var se *StatusError
	if !errors.As(err, &se) {
		t.Fatalf("error %v is not a *StatusError", err)
	}
	if se.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", se.Status, http.StatusBadRequest)
	}
	if se.Service != SonarrSpec.Name {
		t.Errorf("Service = %q, want %q", se.Service, SonarrSpec.Name)
	}
	if se.Body != `[{"errorMessage":"no such host"}]` {
		t.Errorf("Body = %q, want the response body", se.Body)
	}
}

// Every failing status carries the type, not just the 400 the Prowlarr helpers
// happen to care about.
func TestErrorStatusIsInspectableForServerErrors(t *testing.T) {
	srv, _ := fakeService(t, http.StatusInternalServerError, "boom")
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "secret"})

	_, err := c.Get(context.Background(), "/series")
	var se *StatusError
	if !errors.As(err, &se) {
		t.Fatalf("error %v is not a *StatusError", err)
	}
	if se.Status != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", se.Status, http.StatusInternalServerError)
	}
}

// The body reaches the caller already redacted; a typed field must not become a
// way to read back a credential the formatted message would have hidden.
func TestStatusErrorBodyIsRedacted(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	srv, _ := fakeService(t, http.StatusBadRequest, "bad key "+key)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: key})

	_, err := c.Get(context.Background(), "/series")
	var se *StatusError
	if !errors.As(err, &se) {
		t.Fatalf("error %v is not a *StatusError", err)
	}
	if strings.Contains(se.Body, key) {
		t.Errorf("Body leaked the API key: %q", se.Body)
	}
	if strings.Contains(err.Error(), key) {
		t.Errorf("message leaked the API key: %q", err.Error())
	}
}

// The wording is unchanged, so existing messages and their tests still hold.
func TestStatusErrorMessageKeepsItsWording(t *testing.T) {
	se := &StatusError{Service: "sonarr", Status: 404, Body: "not found"}

	if got, want := se.Error(), "sonarr returned 404: not found"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

package arr

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// qbitFake imitates qBittorrent's WebUI: a form login that issues an SID
// cookie, and every other path requiring that cookie.
type qbitFake struct {
	srv        *httptest.Server
	logins     int
	loginForm  url.Values
	loginHdr   http.Header
	lastHdr    http.Header
	lastBody   string
	lastCT     string
	paths      []string
	sid        string
	loginBody  string
	issueSID   bool
	requireSID bool
}

func fakeQBit(t *testing.T) *qbitFake {
	t.Helper()
	f := &qbitFake{loginBody: "Ok.", issueSID: true, requireSID: true}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.paths = append(f.paths, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/api/v2/auth/login" {
			f.logins++
			_ = r.ParseForm()
			f.loginForm = r.PostForm
			f.loginHdr = r.Header.Clone()
			if f.issueSID {
				f.sid = fmt.Sprintf("sid-%d-0123456789", f.logins)
				http.SetCookie(w, &http.Cookie{Name: "SID", Value: f.sid, Path: "/"})
			}
			_, _ = w.Write([]byte(f.loginBody))
			return
		}
		f.lastHdr = r.Header.Clone()
		f.lastCT = r.Header.Get("Content-Type")
		_ = r.ParseForm()
		f.lastBody = r.PostForm.Encode()
		if f.requireSID {
			ck, err := r.Cookie("SID")
			if err != nil || ck.Value != f.sid {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("Forbidden"))
				return
			}
		}
		_, _ = w.Write([]byte("v5.1.2"))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func qbitClient(f *qbitFake) *Client {
	return NewClient(f.srv.URL, QBittorrentSpec, Credentials{Username: "admin", Password: "hunter2hunter2"})
}

func TestSessionLoginSendsFormWithRefererThenReplaysCookie(t *testing.T) {
	f := fakeQBit(t)
	c := qbitClient(f)

	body, err := c.Get(context.Background(), "/app/version")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if string(body) != "v5.1.2" {
		t.Errorf("body = %q, want v5.1.2", body)
	}
	if f.loginForm.Get("username") != "admin" || f.loginForm.Get("password") != "hunter2hunter2" {
		t.Errorf("login form = %v, want username=admin password=hunter2hunter2", f.loginForm)
	}
	if got := f.loginHdr.Get("Referer"); got != f.srv.URL {
		t.Errorf("login Referer = %q, want %q", got, f.srv.URL)
	}
	if got := f.lastHdr.Get("Cookie"); !strings.Contains(got, "SID="+f.sid) {
		t.Errorf("Cookie = %q, want SID=%s", got, f.sid)
	}
	want := []string{"POST /api/v2/auth/login", "GET /api/v2/app/version"}
	if strings.Join(f.paths, ",") != strings.Join(want, ",") {
		t.Errorf("requests = %v, want %v", f.paths, want)
	}
}

func TestSessionIsSharedAcrossClientsForTheSameInstance(t *testing.T) {
	f := fakeQBit(t)

	if _, err := qbitClient(f).Get(context.Background(), "/app/version"); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if _, err := qbitClient(f).Get(context.Background(), "/app/version"); err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if f.logins != 1 {
		t.Errorf("logins = %d, want 1 (session should be cached per instance)", f.logins)
	}
}

func TestSessionRelogsInOnceWhenCookieExpires(t *testing.T) {
	f := fakeQBit(t)
	c := qbitClient(f)

	if _, err := c.Get(context.Background(), "/app/version"); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	f.sid = "rotated-by-server" // the old cookie is now rejected with 403

	if _, err := c.Get(context.Background(), "/app/version"); err != nil {
		t.Fatalf("Get after expiry: %v", err)
	}
	if f.logins != 2 {
		t.Errorf("logins = %d, want 2", f.logins)
	}
	want := "POST /api/v2/auth/login,GET /api/v2/app/version,GET /api/v2/app/version,POST /api/v2/auth/login,GET /api/v2/app/version"
	if got := strings.Join(f.paths, ","); got != want {
		t.Errorf("requests = %s, want %s", got, want)
	}
}

func TestSessionLoginFailureIsClearAndDoesNotLeakPassword(t *testing.T) {
	f := fakeQBit(t)
	f.loginBody = "Fails."
	f.issueSID = false

	_, err := qbitClient(f).Get(context.Background(), "/app/version")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "login failed") {
		t.Errorf("error = %q, want it to mention login failed", err)
	}
	if strings.Contains(err.Error(), "hunter2hunter2") {
		t.Errorf("error leaks the password: %q", err)
	}
	if len(f.paths) != 1 {
		t.Errorf("requests = %v, want only the login attempt", f.paths)
	}
}

func TestSessionAcceptsLoginWithoutCookieWhenAuthIsBypassed(t *testing.T) {
	f := fakeQBit(t)
	f.issueSID = false
	f.requireSID = false

	if _, err := qbitClient(f).Get(context.Background(), "/app/version"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got := f.lastHdr.Get("Cookie"); got != "" {
		t.Errorf("Cookie = %q, want none when the server issued no SID", got)
	}
}

func TestPostFormSendsURLEncodedBody(t *testing.T) {
	f := fakeQBit(t)

	_, err := qbitClient(f).PostForm(context.Background(), "/torrents/stop", url.Values{"hashes": {"a|b"}})
	if err != nil {
		t.Fatalf("PostForm returned error: %v", err)
	}
	if f.lastCT != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", f.lastCT)
	}
	if f.lastBody != "hashes=a%7Cb" {
		t.Errorf("body = %q, want hashes=a%%7Cb", f.lastBody)
	}
}

func TestQBittorrentPingLogsInThenHitsAppVersion(t *testing.T) {
	f := fakeQBit(t)

	if err := qbitClient(f).Ping(context.Background()); err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
	if got := f.paths[len(f.paths)-1]; got != "GET /api/v2/app/version" {
		t.Errorf("last request = %q, want GET /api/v2/app/version", got)
	}
}

func TestNZBGetPingUsesJSONRPCVersionWithBasicAuth(t *testing.T) {
	srv, got := fakeService(t, 200, `{"version":"1.1","result":"26.3"}`)
	c := NewClient(srv.URL, NZBGetSpec, Credentials{Username: "nzb", Password: "pw"})

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
	if got.path != "/jsonrpc/version" {
		t.Errorf("path = %q, want /jsonrpc/version", got.path)
	}
	if user, pass, ok := parseBasic(got.header.Get("Authorization")); !ok || user != "nzb" || pass != "pw" {
		t.Errorf("basic auth = (%q,%q,%v), want (nzb,pw,true)", user, pass, ok)
	}
}

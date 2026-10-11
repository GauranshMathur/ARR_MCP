package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// The DSM password in these tests carries a backslash and a parenthesis, as the
// real one does: both must survive form encoding untouched.
const (
	dsmTestUser = "svc-admin"
	dsmTestPass = `pa\ss(w0rd)+x`
)

// dsmReq is one request the fake DSM received.
type dsmReq struct {
	method, path, rawQuery, contentType string
	form                                url.Values
}

func (r dsmReq) key() string { return r.form.Get("api") + "/" + r.form.Get("method") }

// dsmFake imitates DSM's /webapi/entry.cgi: a form-encoded login that issues a
// sid, and every other call needing that sid, with failures answered as HTTP
// 200 and {"success":false,"error":{"code":N}}.
type dsmFake struct {
	srv    *httptest.Server
	mu     sync.Mutex
	reqs   []dsmReq
	logins int
	sid    string
	// handlers maps "api/method" to the raw JSON answered; unlisted calls answer
	// success with empty data.
	handlers map[string]func(call int, form url.Values) string
	calls    map[string]int
	// loginBody overrides the login answer when set.
	loginBody string
}

func newDSM(t *testing.T) *dsmFake {
	t.Helper()
	f := &dsmFake{handlers: map[string]func(int, url.Values) string{}, calls: map[string]int{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(raw))
		f.mu.Lock()
		defer f.mu.Unlock()
		req := dsmReq{method: r.Method, path: r.URL.Path, rawQuery: r.URL.RawQuery, contentType: r.Header.Get("Content-Type"), form: form}
		f.reqs = append(f.reqs, req)
		w.Header().Set("Content-Type", "application/json")
		if form.Get("api") == "SYNO.API.Auth" && form.Get("method") == "login" {
			f.logins++
			switch {
			case f.loginBody != "":
				_, _ = w.Write([]byte(f.loginBody))
			case form.Get("account") == dsmTestUser && form.Get("passwd") == dsmTestPass:
				f.sid = fmt.Sprintf("SID-%d-abcdefghijklmnopqrstuvwxyz0123456789", f.logins)
				_, _ = fmt.Fprintf(w, `{"success":true,"data":{"sid":%q}}`, f.sid)
			default:
				_, _ = w.Write([]byte(`{"success":false,"error":{"code":400}}`))
			}
			return
		}
		if form.Get("_sid") != f.sid || f.sid == "" {
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":119}}`))
			return
		}
		k := req.key()
		f.calls[k]++
		if h, ok := f.handlers[k]; ok {
			_, _ = w.Write([]byte(h(f.calls[k], form)))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{}}`))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// on registers a fixed answer for one api/method.
func (f *dsmFake) on(key, body string) {
	f.handlers[key] = func(int, url.Values) string { return body }
}

func (f *dsmFake) client() *Client {
	return NewClient(f.srv.URL, SynologySpec, Credentials{Username: dsmTestUser, Password: dsmTestPass})
}

// seen returns the api/method of every non-login request, in order.
func (f *dsmFake) seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.reqs {
		if r.key() != "SYNO.API.Auth/login" {
			out = append(out, r.key())
		}
	}
	return out
}

func (f *dsmFake) requests() []dsmReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]dsmReq(nil), f.reqs...)
}

func TestSynologyLoginPostsAFormWithThePasswordIntact(t *testing.T) {
	f := newDSM(t)
	if _, err := synoCall[map[string]any](context.Background(), f.client(), synoDSMInfo, nil); err != nil {
		t.Fatalf("synoCall: %v", err)
	}
	reqs := f.requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want login then call", len(reqs))
	}
	login := reqs[0]
	if login.method != http.MethodPost || login.path != "/webapi/entry.cgi" {
		t.Errorf("login = %s %s, want POST /webapi/entry.cgi", login.method, login.path)
	}
	if login.rawQuery != "" {
		t.Errorf("login URL carries a query %q: credentials must travel in the body", login.rawQuery)
	}
	if login.contentType != "application/x-www-form-urlencoded" {
		t.Errorf("login Content-Type = %q", login.contentType)
	}
	want := map[string]string{"api": "SYNO.API.Auth", "version": "7", "method": "login", "account": dsmTestUser, "passwd": dsmTestPass, "format": "sid"}
	for k, v := range want {
		if got := login.form.Get(k); got != v {
			t.Errorf("login %s = %q, want %q", k, got, v)
		}
	}
}

func TestSynologyCallSendsApiVersionMethodAndSidInTheBody(t *testing.T) {
	f := newDSM(t)
	f.on("SYNO.FileStation.List/list", `{"success":true,"data":{"files":[]}}`)
	_, err := synoCall[map[string]any](context.Background(), f.client(), synoFileList, url.Values{"folder_path": {"/NAS"}})
	if err != nil {
		t.Fatalf("synoCall: %v", err)
	}
	call := f.requests()[1]
	if call.method != http.MethodPost || call.path != "/webapi/entry.cgi" || call.rawQuery != "" {
		t.Errorf("call = %s %s ?%s, want a bare POST /webapi/entry.cgi", call.method, call.path, call.rawQuery)
	}
	for k, v := range map[string]string{"api": "SYNO.FileStation.List", "version": "2", "method": "list", "folder_path": "/NAS", "_sid": f.sid} {
		if got := call.form.Get(k); got != v {
			t.Errorf("form %s = %q, want %q", k, got, v)
		}
	}
}

func TestSynologySessionIsSharedAcrossClients(t *testing.T) {
	f := newDSM(t)
	for i := 0; i < 2; i++ {
		if _, err := synoCall[map[string]any](context.Background(), f.client(), synoDSMInfo, nil); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if f.logins != 1 {
		t.Errorf("logins = %d, want 1: the session must be cached per instance", f.logins)
	}
}

func TestSynologyExpiredSessionLogsInAgainAndRetriesExactlyOnce(t *testing.T) {
	f := newDSM(t)
	c := f.client()
	if _, err := synoCall[map[string]any](context.Background(), c, synoDSMInfo, nil); err != nil {
		t.Fatalf("first call: %v", err)
	}
	f.mu.Lock()
	f.sid = "rotated-by-dsm" // the cached sid now answers 119
	f.mu.Unlock()

	if _, err := synoCall[map[string]any](context.Background(), c, synoDSMInfo, nil); err != nil {
		t.Fatalf("call after expiry: %v", err)
	}
	if f.logins != 2 {
		t.Errorf("logins = %d, want 2", f.logins)
	}
	if got := strings.Join(f.seen(), ","); got != "SYNO.DSM.Info/getinfo,SYNO.DSM.Info/getinfo,SYNO.DSM.Info/getinfo" {
		t.Errorf("calls = %s, want the call, the rejected call, and one retry", got)
	}
}

func TestSynologyDoesNotRetryAFailedRetry(t *testing.T) {
	f := newDSM(t)
	f.handlers["SYNO.DSM.Info/getinfo"] = func(int, url.Values) string { return `{"success":false,"error":{"code":106}}` }
	_, err := synoCall[map[string]any](context.Background(), f.client(), synoDSMInfo, nil)
	if err == nil || !strings.Contains(err.Error(), "session") {
		t.Fatalf("error = %v, want a session error", err)
	}
	if f.logins != 2 || len(f.seen()) != 2 {
		t.Errorf("logins = %d, calls = %d, want one retry only (2 and 2)", f.logins, len(f.seen()))
	}
}

func TestSynologyCallErrorCodesMapToSpecificMessages(t *testing.T) {
	cases := []struct {
		op   synoOp
		code int
		want string
	}{
		{synoDSMInfo, 105, "permission"},
		{synoDSMInfo, 103, "does not exist"},
		{synoDSMInfo, 104, "version"},
		{synoDSMInfo, 101, "invalid parameter"},
		{synoFileList, 408, "no such file or directory"},
		{synoFileList, 418, "illegal name or path"},
		{synoFileList, 402, "too busy"},
		{synoFileList, 407, "not permitted"},
		{synoDirSizeStatus, 599, "no such task"},
		{synoStorageLoadInfo, 4242, "code 4242"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_%d", tc.op.method, tc.code), func(t *testing.T) {
			f := newDSM(t)
			f.on(tc.op.api+"/"+tc.op.method, fmt.Sprintf(`{"success":false,"error":{"code":%d}}`, tc.code))
			_, err := synoCall[map[string]any](context.Background(), f.client(), tc.op, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.op.api) || !strings.Contains(err.Error(), tc.op.method) {
				t.Errorf("error %q should name the api and method", err)
			}
		})
	}
}

// File Station's 402 is "too busy" but other DSM APIs reuse the number, so a
// non-File-Station API must not borrow File Station's wording.
func TestSynologyFileStationCodesApplyOnlyToFileStation(t *testing.T) {
	f := newDSM(t)
	f.on("SYNO.DSM.Info/getinfo", `{"success":false,"error":{"code":408}}`)
	_, err := synoCall[map[string]any](context.Background(), f.client(), synoDSMInfo, nil)
	if err == nil || strings.Contains(err.Error(), "no such file") || !strings.Contains(err.Error(), "code 408") {
		t.Errorf("error = %v, want the generic code 408", err)
	}
}

func TestSynologyLoginErrorsAreSpecificAndLeakNothing(t *testing.T) {
	cases := map[int]string{
		400: "wrong username or password",
		401: "disabled",
		403: "2-factor",
		406: "2-factor",
		407: "blocked",
	}
	for code, want := range cases {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			f := newDSM(t)
			f.loginBody = fmt.Sprintf(`{"success":false,"error":{"code":%d}}`, code)
			_, err := synoCall[map[string]any](context.Background(), f.client(), synoDSMInfo, nil)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %v, want it to contain %q", err, want)
			}
			if strings.Contains(err.Error(), dsmTestPass) || strings.Contains(err.Error(), dsmTestUser+"&") {
				t.Errorf("login error leaks the credentials: %v", err)
			}
		})
	}
}

func TestSynologyWrongPasswordIsTriedOnceNotRetried(t *testing.T) {
	f := newDSM(t)
	c := NewClient(f.srv.URL, SynologySpec, Credentials{Username: dsmTestUser, Password: "not-the-password"})
	if _, err := synoCall[map[string]any](context.Background(), c, synoDSMInfo, nil); err == nil {
		t.Fatal("expected a login error")
	}
	if f.logins != 1 {
		t.Errorf("logins = %d, want exactly 1: DSM blocks an IP after repeated failures", f.logins)
	}
}

// A proxy or DSM itself may answer a non-200 with a page that echoes the form.
// Neither the password nor the sid may reach the error.
func TestSynologyHTTPErrorBodyIsRedacted(t *testing.T) {
	f := newDSM(t)
	var sid string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(raw))
		if form.Get("method") == "login" {
			sid = "SID-0123456789abcdefghijklmnopqrstuvwxyz"
			_, _ = fmt.Fprintf(w, `{"success":true,"data":{"sid":%q}}`, sid)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprintf(w, "bad gateway for %s sid %s password %s", form.Get("api"), form.Get("_sid"), dsmTestPass)
	}))
	t.Cleanup(srv.Close)
	_ = f
	c := NewClient(srv.URL, SynologySpec, Credentials{Username: dsmTestUser, Password: dsmTestPass})
	_, err := synoCall[map[string]any](context.Background(), c, synoDSMInfo, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), dsmTestPass) || strings.Contains(err.Error(), sid) {
		t.Errorf("error leaks a secret: %v", err)
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("error %q should carry the HTTP status", err)
	}
}

func TestSynologyUndecodableAnswerIsAnError(t *testing.T) {
	f := newDSM(t)
	f.on("SYNO.DSM.Info/getinfo", `<html>not json</html>`)
	_, err := synoCall[map[string]any](context.Background(), f.client(), synoDSMInfo, nil)
	if err == nil || !strings.Contains(err.Error(), "decoding") {
		t.Errorf("error = %v, want a decoding error", err)
	}
}

// What the code can call is the only limit on a password that is full control
// of the NAS, so synoCall refuses anything outside the fixed read set before it
// sends a byte -- not even a login.
func TestSynologyCallRefusesAnythingOutsideTheReadSet(t *testing.T) {
	f := newDSM(t)
	for _, op := range []synoOp{
		{"SYNO.Core.User", "list", 1},
		{"SYNO.Core.ISCSI.LUN", "list", 1},
		{"SYNO.Core.Share", "delete", 1},
		{"SYNO.FileStation.List", "list", 9},
		{"SYNO.FileStation.Delete", "start", 2},
		{"SYNO.DSM.Info", "reboot", 2},
	} {
		if _, err := synoCall[map[string]any](context.Background(), f.client(), op, nil); err == nil {
			t.Errorf("synoCall accepted %+v", op)
		}
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("%d requests reached the NAS, want none", n)
	}
}

func TestSynologyPingLogsInAndReadsDSMInfo(t *testing.T) {
	f := newDSM(t)
	if err := f.client().Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if got := strings.Join(f.seen(), ","); got != "SYNO.DSM.Info/getinfo" {
		t.Errorf("ping calls = %s", got)
	}
	bad := NewClient(f.srv.URL, SynologySpec, Credentials{Username: dsmTestUser, Password: "wrong-password"})
	if err := bad.Ping(context.Background()); err == nil || !strings.Contains(err.Error(), "wrong username or password") {
		t.Errorf("Ping with a wrong password = %v", err)
	}
}

func TestSynologyBaseURLSubpathIsKept(t *testing.T) {
	f := newDSM(t)
	c := NewClient(f.srv.URL+"/", SynologySpec, Credentials{Username: dsmTestUser, Password: dsmTestPass})
	if _, err := synoCall[map[string]any](context.Background(), c, synoDSMInfo, nil); err != nil {
		t.Fatalf("synoCall: %v", err)
	}
	if p := f.requests()[0].path; p != "/webapi/entry.cgi" {
		t.Errorf("path = %q", p)
	}
}

// DSM holds a failed login for several seconds on purpose, to slow guessing. A
// 5 second health check would report that as a timeout and hide the real
// cause, so the check waits long enough to hear "wrong username or password".
func TestSynologyPingWaitsLongerThanTheDefaultForAFailedLogin(t *testing.T) {
	if got, def := NewClient("http://x", SynologySpec, Credentials{}).pingTimeout(), NewClient("http://x", SonarrSpec, Credentials{}).pingTimeout(); got <= def {
		t.Errorf("synology ping timeout = %v, want more than the default %v", got, def)
	}
}

// --- redaction ---

// redactPass has the characters that encode differently in a form, a path and
// a JSON string: a space, a double quote, a backslash and a parenthesis.
const (
	redactPass = `p"a\ss w0rd(1)`
	redactSID  = "SID-0123456789abcdefghijklmnopqrstuvwxyzABCDEF/+="
)

// spellings are the ways an upstream could echo a secret back.
func spellings(secret string) map[string]string {
	j, _ := json.Marshal(secret)
	q := url.QueryEscape(secret)
	return map[string]string{
		"raw":       secret,
		"query":     q,
		"path":      url.PathEscape(secret),
		"json":      strings.Trim(string(j), `"`),
		"lower-hex": strings.ToLower(q),
		"upper-hex": strings.ToUpper(q),
	}
}

// echoServer logs in normally, then answers every other request with an error
// that echoes whatever the test wants.
func echoServer(t *testing.T, echo func() string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(raw))
		if form.Get("method") == "login" {
			if form.Get("passwd") != redactPass {
				_, _ = w.Write([]byte(`{"success":false,"error":{"code":400}}`))
				return
			}
			_, _ = fmt.Fprintf(w, `{"success":true,"data":{"sid":%q}}`, redactSID)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("gateway: " + echo()))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func assertMasked(t *testing.T, where string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected an error", where)
	}
	for _, secret := range []string{redactPass, redactSID} {
		for name, sp := range spellings(secret) {
			if strings.Contains(err.Error(), sp) {
				t.Errorf("%s: error leaks the %s spelling of a secret: %v", where, name, err)
			}
		}
	}
}

func TestSynologyRedactsEverySpellingOnEveryPath(t *testing.T) {
	for _, secret := range []string{redactPass, redactSID} {
		for name, sp := range spellings(secret) {
			t.Run(name, func(t *testing.T) {
				srv := echoServer(t, func() string { return sp })
				c := NewClient(srv.URL, SynologySpec, Credentials{Username: dsmTestUser, Password: redactPass})
				_, err := synoCall[map[string]any](context.Background(), c, synoDSMInfo, nil)
				assertMasked(t, "synoCall", err)
				// The generic transport path, with a session already cached.
				_, err = c.Get(context.Background(), "/entry.cgi")
				assertMasked(t, "generic Get", err)
			})
		}
	}
}

func TestSynologyRedactsFromTheLoginPath(t *testing.T) {
	for _, secret := range []string{redactPass} {
		for name, sp := range spellings(secret) {
			t.Run(name, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte("login echo " + sp))
				}))
				t.Cleanup(srv.Close)
				c := NewClient(srv.URL, SynologySpec, Credentials{Username: dsmTestUser, Password: redactPass})
				_, err := synoCall[map[string]any](context.Background(), c, synoDSMInfo, nil)
				assertMasked(t, "login", err)
			})
		}
	}
}

func TestSynologyRedactMasksTheSessionIDInTransportErrors(t *testing.T) {
	f := newDSM(t)
	c := f.client()
	if _, err := synoCall[map[string]any](context.Background(), c, synoDSMInfo, nil); err != nil {
		t.Fatal(err)
	}
	for name, sp := range spellings(f.sid) {
		if got := c.redact("Post failed: " + sp); strings.Contains(got, sp) {
			t.Errorf("redact left the %s spelling of the sid: %q", name, got)
		}
	}
}

// A sid being replaced during a re-login is a secret too.
func TestSynologyRedactMasksTheReplacedSessionID(t *testing.T) {
	f := newDSM(t)
	c := f.client()
	if _, err := synoCall[map[string]any](context.Background(), c, synoDSMInfo, nil); err != nil {
		t.Fatal(err)
	}
	old := f.sid
	f.mu.Lock()
	f.sid = "rotated"
	f.mu.Unlock()
	if _, err := synoCall[map[string]any](context.Background(), c, synoDSMInfo, nil); err != nil {
		t.Fatal(err)
	}
	if got := c.redact("old " + old); strings.Contains(got, old) {
		t.Errorf("replaced sid not masked: %q", got)
	}
}

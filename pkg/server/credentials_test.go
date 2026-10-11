package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestInstanceCredentialsCarryEveryConfiguredSecret(t *testing.T) {
	inst := &config.Instance{APIKey: "key", Username: "user", Password: "pw"}

	got := InstanceCredentials(inst)
	if got.APIKey != "key" || got.Username != "user" || got.Password != "pw" {
		t.Errorf("InstanceCredentials = %+v, want key/user/pw", got)
	}
}

// The Synology password is full control of the NAS. Neither it nor the session
// id may reach a tool result, whether DSM refuses the login or a proxy in front
// of it answers an error page that echoes the request form.
func TestSynologyErrorsNeverCarryThePasswordOrTheSessionID(t *testing.T) {
	t.Run("login refused", func(t *testing.T) {
		srv, _ := synologyUpstream(t, nil)
		cfg := cfgWith(map[string][]config.Instance{
			"synology": {{Name: "nas", URL: srv.URL, Username: synoTestUser, Password: "not-the-password", Default: true}},
		}, permsFull)
		res := callSynologyRaw(t, connect(t, cfg), "synology_system_info")
		if !res.IsError || !strings.Contains(contentText(res), "wrong username or password") {
			t.Fatalf("result = %s, want a wrong-password error", contentText(res))
		}
		if strings.Contains(contentText(res), "not-the-password") {
			t.Errorf("error leaks the password: %s", contentText(res))
		}
	})

	t.Run("gateway page echoing the login form", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprintf(w, "upstream failed for %s", string(raw))
		}))
		t.Cleanup(srv.Close)
		res := callSynologyRaw(t, connect(t, synologyCfg(srv.URL, permsFull)), "synology_system_info")
		text := contentText(res)
		if !res.IsError || !strings.Contains(text, "502") {
			t.Fatalf("result = %s, want a 502 error", text)
		}
		for _, secret := range []string{synoTestPass, url.QueryEscape(synoTestPass)} {
			if strings.Contains(text, secret) {
				t.Errorf("error leaks %q: %s", secret, text)
			}
		}
	})

	t.Run("gateway page echoing the form", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			form, _ := url.ParseQuery(string(raw))
			if form.Get("method") == "login" {
				_, _ = fmt.Fprintf(w, `{"success":true,"data":{"sid":%q}}`, synoTestSID)
				return
			}
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprintf(w, "upstream failed for %s", string(raw))
		}))
		t.Cleanup(srv.Close)
		cfg := synologyCfg(srv.URL, permsFull)
		res := callSynologyRaw(t, connect(t, cfg), "synology_system_info")
		if !res.IsError {
			t.Fatalf("result = %s, want an error", contentText(res))
		}
		text := contentText(res)
		for _, secret := range []string{synoTestPass, url.QueryEscape(synoTestPass), synoTestSID} {
			if strings.Contains(text, secret) {
				t.Errorf("error leaks %q: %s", secret, text)
			}
		}
	})
}

func callSynologyRaw(t *testing.T, cs *mcp.ClientSession, name string) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	return res
}

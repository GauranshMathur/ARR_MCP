package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// qbitToolNames is every tool registerQBittorrent must advertise, by tier.
var qbitToolNames = map[Access][]string{
	AccessRead: {
		"qbittorrent_system_status", "qbittorrent_list_torrents", "qbittorrent_torrent_files",
		"qbittorrent_transfer_info", "qbittorrent_list_categories", "qbittorrent_list_tags",
	},
	AccessWrite: {
		"qbittorrent_add_torrent", "qbittorrent_stop_torrents", "qbittorrent_start_torrents",
		"qbittorrent_recheck_torrents", "qbittorrent_set_category", "qbittorrent_create_category",
		"qbittorrent_edit_category", "qbittorrent_add_tags", "qbittorrent_remove_tags",
		"qbittorrent_set_location", "qbittorrent_rename_torrent", "qbittorrent_set_priority",
		"qbittorrent_set_torrent_limits", "qbittorrent_set_global_limits",
	},
	AccessDestructive: {
		"qbittorrent_delete_torrents", "qbittorrent_delete_categories",
	},
}

// fakeQBitServer imitates qBittorrent's login flow: a form login that issues
// an SID cookie, then JSON on every other path provided the cookie comes back.
func fakeQBitServer(t *testing.T, body string) (*httptest.Server, *[]string) {
	t.Helper()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "test-sid-0123456789", Path: "/"})
			_, _ = w.Write([]byte("Ok."))
			return
		}
		if ck, err := r.Cookie("SID"); err != nil || ck.Value != "test-sid-0123456789" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &paths
}

func qbitCfg(url string, perms config.Permissions) *config.Config {
	return cfgWith(map[string][]config.Instance{
		"qbittorrent": {{Name: "main", URL: url, Username: "admin", Password: "test-password-value", Default: true}},
	}, perms)
}

func TestQBittorrentRegistersAllTwentyTwoTools(t *testing.T) {
	srv, _ := fakeQBitServer(t, `[]`)
	names := toolNames(t, connect(t, qbitCfg(srv.URL, permsFull)))

	total := 0
	for _, group := range qbitToolNames {
		for _, want := range group {
			total++
			if !has(names, want) {
				t.Errorf("tool %q not advertised", want)
			}
		}
	}
	if total != 22 {
		t.Fatalf("test expects %d tools, want 22; the table above is stale", total)
	}

	qbitCount := 0
	for _, n := range names {
		if strings.HasPrefix(n, "qbittorrent_") {
			qbitCount++
		}
	}
	if qbitCount != 22 {
		t.Errorf("advertised %d qbittorrent tools, want 22: %v", qbitCount, names)
	}
}

func TestQBittorrentToolsAbsentWhenOnlySonarrIsConfigured(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {{Name: "main", URL: srv.URL, APIKey: "k", Default: true}},
	}, permsFull)))

	for _, n := range names {
		if strings.HasPrefix(n, "qbittorrent_") {
			t.Errorf("tool %q exposed without a configured qbittorrent instance", n)
		}
	}
}

func TestQBittorrentReadOnlyModeHidesWriteAndDestructiveTools(t *testing.T) {
	srv, _ := fakeQBitServer(t, `[]`)
	perms := config.Permissions{Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny}
	names := toolNames(t, connect(t, qbitCfg(srv.URL, perms)))

	for _, want := range qbitToolNames[AccessRead] {
		if !has(names, want) {
			t.Errorf("readonly mode must still expose %q", want)
		}
	}
	for _, tier := range []Access{AccessWrite, AccessDestructive} {
		for _, unwanted := range qbitToolNames[tier] {
			if has(names, unwanted) {
				t.Errorf("readonly mode must not expose %q", unwanted)
			}
		}
	}
}

func TestQBittorrentListTorrentsLogsInThenFetchesWithTheCookie(t *testing.T) {
	srv, paths := fakeQBitServer(t, `[{"hash":"abc","name":"debian.iso","state":"downloading","tags":"iso"}]`)
	cs := connect(t, qbitCfg(srv.URL, permsFull))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "qbittorrent_list_torrents",
		Arguments: map[string]any{"filter": "downloading"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	// The session cache may satisfy the login from an earlier call against the
	// same URL, so assert on the tail rather than the absolute sequence.
	got := *paths
	if len(got) == 0 || got[len(got)-1] != "GET /api/v2/torrents/info" {
		t.Fatalf("upstream calls = %v, want them to end with GET /api/v2/torrents/info", got)
	}
	if got[0] != "POST /api/v2/auth/login" {
		t.Errorf("upstream calls = %v, want a login first", got)
	}
	if body := contentText(res); !strings.Contains(body, "debian.iso") {
		t.Errorf("result does not contain the torrent name: %s", body)
	}
}

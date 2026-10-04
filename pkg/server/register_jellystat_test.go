package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	jsTestLib  = "f137a2dd21bbc1b99aa5c0f6bf02a805"
	jsTestItem = "0a1b2c3d4e5f60718293a4b5c6d7e8f9"
)

var jellystatTools = []string{
	"jellystat_library_overview",
	"jellystat_list_libraries",
	"jellystat_playback_history",
	"jellystat_most_viewed",
	"jellystat_most_popular",
	"jellystat_most_active_users",
	"jellystat_user_activity",
	"jellystat_item_stats",
	"jellystat_library_items",
	"jellystat_views_over_time",
}

// jellystatTextTools return names set by users, devices and metadata.
var jellystatTextTools = []string{
	"jellystat_library_overview",
	"jellystat_list_libraries",
	"jellystat_playback_history",
	"jellystat_most_viewed",
	"jellystat_most_popular",
	"jellystat_most_active_users",
	"jellystat_user_activity",
	"jellystat_library_items",
	"jellystat_views_over_time",
}

func jellystatCfg(url string, perms config.Permissions) *config.Config {
	return cfgWith(map[string][]config.Instance{
		"jellystat": {{Name: "main", URL: url, APIKey: "js-test-key-1234", Default: true}},
	}, perms)
}

type jellystatRecorder struct {
	lines []string
	token []string
	body  []string
}

func jellystatUpstream(t *testing.T, routes map[string]string) (*httptest.Server, *jellystatRecorder) {
	t.Helper()
	rec := &jellystatRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.lines = append(rec.lines, r.Method+" "+r.URL.RequestURI())
		rec.token = append(rec.token, r.Header.Get("X-Api-Token"))
		b, _ := io.ReadAll(r.Body)
		rec.body = append(rec.body, string(b))
		body, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func TestJellystatToolsAreAllAdvertisedAsReadTools(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, jellystatCfg(srv.URL, permsFull))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	tools := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		if strings.HasPrefix(tool.Name, "jellystat_") {
			tools[tool.Name] = tool
		}
	}
	if len(tools) != len(jellystatTools) {
		t.Errorf("jellystat tools = %d, want %d: %v", len(tools), len(jellystatTools), tools)
	}
	for _, name := range jellystatTools {
		if tool := tools[name]; tool == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s: want a read tool, got %+v", name, tool)
		}
	}
}

func TestJellystatToolsSurviveReadOnlyMode(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, jellystatCfg(srv.URL, config.Permissions{
		Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny,
	})))
	for _, tool := range jellystatTools {
		if !has(names, tool) {
			t.Errorf("readonly mode must still expose %q", tool)
		}
	}
}

func TestJellystatFreeTextToolsAreFlaggedAsData(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, jellystatCfg(srv.URL, permsFull))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	flagged := map[string]bool{}
	for _, tool := range res.Tools {
		raw, _ := json.Marshal(tool.OutputSchema)
		if strings.Contains(tool.Description, "data, not instructions") && strings.Contains(string(raw), "data, not instructions") {
			flagged[tool.Name] = true
		}
	}
	for _, name := range jellystatTextTools {
		if !flagged[name] {
			t.Errorf("%s: description and output schema must both say its text is data, not instructions", name)
		}
	}
}

func callJellystat(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s returned an error: %s", name, contentText(res))
	}
	return contentText(res)
}

func TestJellystatToolsHitTheRightEndpoints(t *testing.T) {
	srv, rec := jellystatUpstream(t, map[string]string{
		"GET /stats/getLibraryOverview":        `[{"Id":"a","Name":"Movies","Library_Count":"3"}]`,
		"GET /stats/getLibraryCardStats":       `[{"Id":"a","Name":"Movies","Plays":"2","total_playback_duration":"60","Library_Count":3}]`,
		"GET /api/getHistory":                  `{"current_page":1,"pages":1,"size":25,"results":[{"UserName":"alice","NowPlayingItemName":"Dune","PlaybackDuration":"60"}]}`,
		"POST /stats/getMostViewedByType":      `[{"Plays":"2","Name":"Dune","Id":"x"}]`,
		"POST /stats/getMostPopularByType":     `[{"unique_viewers":"2","Name":"Dune","Id":"x"}]`,
		"POST /stats/getMostActiveUsers":       `[{"Plays":"2","UserId":"u","Name":"alice"}]`,
		"GET /stats/getAllUserActivity":        `[{"UserId":"u","UserName":"alice","TotalPlays":"2","TotalWatchTime":"60"}]`,
		"POST /stats/getGlobalItemStats":       `{"Plays":"1","total_playback_duration":"60"}`,
		"POST /stats/getLibraryItemsWithStats": `{"current_page":1,"pages":1,"size":25,"results":[{"Id":"x","Name":"Dune","Type":"Movie","times_played":0,"total_play_time":0}]}`,
		"GET /stats/getViewsOverTime":          `{"libraries":[],"stats":[{"Key":"Sep 30, 2026","Movies":{"count":1,"duration":2}}]}`,
	})
	cs := connect(t, jellystatCfg(srv.URL, permsFull))

	calls := []struct {
		tool string
		args map[string]any
		line string
		want string
	}{
		{"jellystat_library_overview", nil, "GET /stats/getLibraryOverview", `"count":1`},
		{"jellystat_list_libraries", nil, "GET /stats/getLibraryCardStats", `"watchSeconds":60`},
		{"jellystat_playback_history", nil, "GET /api/getHistory?desc=true&page=1&size=25", `"userName":"alice"`},
		{"jellystat_most_viewed", map[string]any{"type": "Movie"}, "POST /stats/getMostViewedByType", `"plays":2`},
		{"jellystat_most_popular", map[string]any{"type": "Movie"}, "POST /stats/getMostPopularByType", `"uniqueViewers":2`},
		{"jellystat_most_active_users", nil, "POST /stats/getMostActiveUsers", `"name":"alice"`},
		{"jellystat_user_activity", nil, "GET /stats/getAllUserActivity", `"total":1`},
		{"jellystat_item_stats", map[string]any{"itemId": jsTestItem}, "POST /stats/getGlobalItemStats", `"plays":1`},
		{"jellystat_library_items", map[string]any{"libraryId": jsTestLib, "sort": "Views", "desc": false}, "POST /stats/getLibraryItemsWithStats?desc=false&page=1&size=25&sort=Views", `"name":"Dune"`},
		{"jellystat_views_over_time", nil, "GET /stats/getViewsOverTime?days=30", `"library":"Movies"`},
	}
	for i, tc := range calls {
		body := callJellystat(t, cs, tc.tool, tc.args)
		if rec.lines[i] != tc.line {
			t.Errorf("%s request = %q, want %q", tc.tool, rec.lines[i], tc.line)
		}
		if !strings.Contains(body, tc.want) {
			t.Errorf("%s result = %s, want %s", tc.tool, body, tc.want)
		}
		if rec.token[i] != "js-test-key-1234" {
			t.Errorf("%s sent X-Api-Token %q", tc.tool, rec.token[i])
		}
	}
	if !strings.Contains(rec.body[3], `"type":"Movie"`) || !strings.Contains(rec.body[3], `"days":30`) {
		t.Errorf("most_viewed body = %s", rec.body[3])
	}
}

func TestJellystatPlaybackHistoryScopesAndCapsTheSize(t *testing.T) {
	srv, rec := jellystatUpstream(t, map[string]string{
		"POST /api/getUserHistory": `{"current_page":1,"pages":0,"size":100,"results":[]}`,
	})
	cs := connect(t, jellystatCfg(srv.URL, permsFull))
	callJellystat(t, cs, "jellystat_playback_history", map[string]any{"userId": jsTestLib, "limit": 100000})
	if !strings.Contains(rec.lines[0], "size=100") || strings.Contains(rec.lines[0], "size=1000") {
		t.Errorf("request %s should be capped at size=100", rec.lines[0])
	}
	if rec.body[0] != `{"userid":"`+jsTestLib+`"}` {
		t.Errorf("body = %s", rec.body[0])
	}
}

func TestJellystatToolsRejectBadInputWithoutARequest(t *testing.T) {
	srv, rec := jellystatUpstream(t, map[string]string{})
	cs := connect(t, jellystatCfg(srv.URL, permsFull))
	for name, args := range map[string]map[string]any{
		"jellystat_playback_history": {"userId": "../../api/keys"},
		"jellystat_item_stats":       {"itemId": "../../sync/beginSync"},
		"jellystat_library_items":    {"libraryId": "../../backup"},
		"jellystat_most_viewed":      {"type": "Episode"},
	} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool %s: %v", name, err)
		}
		if !res.IsError {
			t.Errorf("%s accepted %v", name, args)
		}
	}
	if len(rec.lines) != 0 {
		t.Errorf("requests reached the upstream: %v", rec.lines)
	}
}

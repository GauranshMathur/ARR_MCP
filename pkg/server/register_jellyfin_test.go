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

var jellyfinReadTools = []string{
	"jellyfin_system_info",
	"jellyfin_list_libraries",
	"jellyfin_search_items",
	"jellyfin_get_item",
	"jellyfin_list_sessions",
	"jellyfin_list_users",
	"jellyfin_list_tasks",
	"jellyfin_activity_log",
}

var jellyfinWriteTools = []string{
	"jellyfin_scan_library",
	"jellyfin_refresh_item",
	"jellyfin_run_task",
}

func jellyfinCfg(url string, perms config.Permissions) *config.Config {
	return cfgWith(map[string][]config.Instance{
		"jellyfin": {{Name: "main", URL: url, APIKey: "jf-test-key-1234", Default: true}},
	}, perms)
}

// jellyfinRecorder serves a body per "METHOD /path" and records each request
// line, raw query and Authorization header.
type jellyfinRecorder struct {
	lines []string
	auth  []string
}

func jellyfinUpstream(t *testing.T, routes map[string]string) (*httptest.Server, *jellyfinRecorder) {
	t.Helper()
	rec := &jellyfinRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.lines = append(rec.lines, r.Method+" "+r.URL.RequestURI())
		rec.auth = append(rec.auth, r.Header.Get("Authorization"))
		_, _ = io.Copy(io.Discard, r.Body)
		body, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if body == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func callJellyfin(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) string {
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

func TestJellyfinToolsAreAllAdvertisedWithTheirTier(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	tools := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		if strings.HasPrefix(tool.Name, "jellyfin_") {
			tools[tool.Name] = tool
		}
	}
	if len(tools) != len(jellyfinReadTools)+len(jellyfinWriteTools) {
		t.Errorf("jellyfin tools = %d, want 11: %v", len(tools), tools)
	}
	for _, name := range jellyfinReadTools {
		if tool := tools[name]; tool == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s: want a read tool, got %+v", name, tool)
		}
	}
	for _, name := range jellyfinWriteTools {
		tool := tools[name]
		if tool == nil || tool.Annotations.ReadOnlyHint || *tool.Annotations.DestructiveHint {
			t.Errorf("%s: want a write (non-destructive) tool, got %+v", name, tool)
		}
	}
}

func TestJellyfinWriteToolsAreHiddenInReadOnlyMode(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, jellyfinCfg(srv.URL, config.Permissions{
		Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny,
	})))
	for _, tool := range jellyfinWriteTools {
		if has(names, tool) {
			t.Errorf("readonly mode must not expose %q", tool)
		}
	}
	for _, tool := range jellyfinReadTools {
		if !has(names, tool) {
			t.Errorf("readonly mode must still expose %q", tool)
		}
	}
}

// Free text typed by users and devices reaches the model verbatim, so the
// schema must tell it that text is data.
func TestJellyfinFreeTextOutputsAreFlaggedAsData(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range res.Tools {
		if tool.Name != "jellyfin_list_users" && tool.Name != "jellyfin_list_sessions" {
			continue
		}
		raw, _ := json.Marshal(tool.OutputSchema)
		if !strings.Contains(string(raw), "data, not instructions") {
			t.Errorf("%s output schema does not flag free text as data: %s", tool.Name, raw)
		}
	}
}

func TestJellyfinListUsersToolOmitsPolicy(t *testing.T) {
	srv, rec := jellyfinUpstream(t, map[string]string{
		"GET /Users": `[{"Id":"u1","Name":"Arkhaya","HasPassword":true,
		  "Policy":{"IsAdministrator":true,"BlockedTags":["secret-tag"]},
		  "Configuration":{"AudioLanguagePreference":"eng"}}]`,
	})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))

	body := callJellyfin(t, cs, "jellyfin_list_users", nil)
	if strings.Contains(body, "secret-tag") || strings.Contains(body, "AudioLanguagePreference") {
		t.Errorf("policy or configuration reached the client: %s", body)
	}
	if !strings.Contains(body, `"count":1`) || !strings.Contains(body, `"isAdministrator":true`) {
		t.Errorf("result = %s", body)
	}
	if rec.auth[0] != `MediaBrowser Token="jf-test-key-1234"` {
		t.Errorf("Authorization = %q", rec.auth[0])
	}
}

func TestJellyfinSearchItemsToolRecursesByDefault(t *testing.T) {
	srv, rec := jellyfinUpstream(t, map[string]string{
		"GET /Items": `{"TotalRecordCount":1,"StartIndex":0,"Items":[{"Id":"7489","Name":"17 Again","Type":"Movie"}]}`,
	})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))

	body := callJellyfin(t, cs, "jellyfin_search_items", map[string]any{"searchTerm": "17", "includeItemTypes": "Movie"})
	if !strings.Contains(rec.lines[0], "recursive=true") || !strings.Contains(rec.lines[0], "searchTerm=17") {
		t.Errorf("request = %s", rec.lines[0])
	}
	if !strings.Contains(rec.lines[0], "limit=25") {
		t.Errorf("request %s should default the page to 25 items", rec.lines[0])
	}
	if !strings.Contains(body, `"totalCount":1`) || !strings.Contains(body, `"count":1`) {
		t.Errorf("result = %s", body)
	}

	callJellyfin(t, cs, "jellyfin_search_items", map[string]any{"recursive": false, "limit": 3})
	if !strings.Contains(rec.lines[1], "recursive=false") || !strings.Contains(rec.lines[1], "limit=3") {
		t.Errorf("explicit values not honoured: %s", rec.lines[1])
	}
}

func TestJellyfinActivityLogToolDefaultsTheLimit(t *testing.T) {
	srv, rec := jellyfinUpstream(t, map[string]string{
		"GET /System/ActivityLog/Entries": `{"TotalRecordCount":0,"StartIndex":0,"Items":[]}`,
	})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))
	callJellyfin(t, cs, "jellyfin_activity_log", nil)
	if !strings.Contains(rec.lines[0], "limit=25") {
		t.Errorf("request = %s, want limit=25", rec.lines[0])
	}
}

func TestJellyfinWriteToolsHitTheRightEndpoints(t *testing.T) {
	srv, rec := jellyfinUpstream(t, map[string]string{
		"POST /Library/Refresh":                 "",
		"POST /Items/7489/Refresh":              "",
		"POST /ScheduledTasks/Running/ec2f221f": "",
	})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))

	callJellyfin(t, cs, "jellyfin_scan_library", nil)
	callJellyfin(t, cs, "jellyfin_refresh_item", map[string]any{"itemId": "7489", "mode": "FullRefresh"})
	callJellyfin(t, cs, "jellyfin_run_task", map[string]any{"taskId": "ec2f221f"})

	if len(rec.lines) != 3 || rec.lines[0] != "POST /Library/Refresh" ||
		!strings.HasPrefix(rec.lines[1], "POST /Items/7489/Refresh?") ||
		!strings.Contains(rec.lines[1], "metadataRefreshMode=FullRefresh") ||
		rec.lines[2] != "POST /ScheduledTasks/Running/ec2f221f" {
		t.Errorf("requests = %v", rec.lines)
	}
}

func TestJellyfinGetItemToolReportsTheUserItUsed(t *testing.T) {
	srv, rec := jellyfinUpstream(t, map[string]string{
		"GET /Users":      `[{"Id":"u1","Name":"a","Policy":{}}]`,
		"GET /Items/7489": `{"Id":"7489","Name":"17 Again","Type":"Movie","UserData":{"Played":true,"PlayCount":2}}`,
	})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))

	body := callJellyfin(t, cs, "jellyfin_get_item", map[string]any{"itemId": "7489"})
	if !strings.Contains(body, `"userId":"u1"`) || !strings.Contains(body, `"playCount":2`) {
		t.Errorf("result = %s", body)
	}
	if rec.lines[1] != "GET /Items/7489?userId=u1" {
		t.Errorf("requests = %v", rec.lines)
	}
}

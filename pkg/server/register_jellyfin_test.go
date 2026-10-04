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
}

// jellyfinDestructiveTools can delete data: scheduled tasks include cache, log,
// transcode and activity-log cleanups, and plugins add their own.
var jellyfinDestructiveTools = []string{"jellyfin_run_task"}

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
	for _, name := range jellyfinDestructiveTools {
		if tool := tools[name]; tool == nil || tool.Annotations.ReadOnlyHint || !*tool.Annotations.DestructiveHint {
			t.Errorf("%s: want a destructive tool, got %+v", name, tool)
		}
	}
	if len(tools) != len(jellyfinReadTools)+len(jellyfinWriteTools)+len(jellyfinDestructiveTools) {
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
	for _, tool := range append(append([]string{}, jellyfinWriteTools...), jellyfinDestructiveTools...) {
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
		"GET /Users": `[{"Id":"0259b8b9ba6540c7b927ec98efa5c2da","Name":"Arkhaya","HasPassword":true,
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
		"GET /Items": `{"TotalRecordCount":1,"StartIndex":0,"Items":[{"Id":"748950f8695760523d0361ea48e0953b","Name":"17 Again","Type":"Movie"}]}`,
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
		"POST /Library/Refresh":                                         "",
		"POST /Items/748950f8695760523d0361ea48e0953b/Refresh":          "",
		"POST /ScheduledTasks/Running/ec2f221fd8e7706b3d3afd2c4591b4d7": "",
	})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))

	callJellyfin(t, cs, "jellyfin_scan_library", nil)
	callJellyfin(t, cs, "jellyfin_refresh_item", map[string]any{"itemId": "748950f8695760523d0361ea48e0953b", "mode": "FullRefresh"})
	callJellyfin(t, cs, "jellyfin_run_task", map[string]any{"taskId": "ec2f221fd8e7706b3d3afd2c4591b4d7"})

	if len(rec.lines) != 3 || rec.lines[0] != "POST /Library/Refresh" ||
		!strings.HasPrefix(rec.lines[1], "POST /Items/748950f8695760523d0361ea48e0953b/Refresh?") ||
		!strings.Contains(rec.lines[1], "metadataRefreshMode=FullRefresh") ||
		rec.lines[2] != "POST /ScheduledTasks/Running/ec2f221fd8e7706b3d3afd2c4591b4d7" {
		t.Errorf("requests = %v", rec.lines)
	}
}

func TestJellyfinGetItemToolReportsTheUserItUsed(t *testing.T) {
	srv, rec := jellyfinUpstream(t, map[string]string{
		"GET /Users": `[{"Id":"0259b8b9ba6540c7b927ec98efa5c2da","Name":"a","Policy":{}}]`,
		"GET /Items/748950f8695760523d0361ea48e0953b": `{"Id":"748950f8695760523d0361ea48e0953b","Name":"17 Again","Type":"Movie","UserData":{"Played":true,"PlayCount":2}}`,
	})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))

	body := callJellyfin(t, cs, "jellyfin_get_item", map[string]any{"itemId": "748950f8695760523d0361ea48e0953b"})
	if !strings.Contains(body, `"userId":"0259b8b9ba6540c7b927ec98efa5c2da"`) || !strings.Contains(body, `"playCount":2`) {
		t.Errorf("result = %s", body)
	}
	if rec.lines[1] != "GET /Items/748950f8695760523d0361ea48e0953b?userId=0259b8b9ba6540c7b927ec98efa5c2da" {
		t.Errorf("requests = %v", rec.lines)
	}
}

// Activity log entries embed names an unauthenticated caller can choose (a
// failed login reads "Failed login attempt from <attempted username>"), so the
// warning belongs in the tool description, where the model reads it first.
func TestJellyfinFreeTextToolDescriptionsWarnAboutData(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	want := map[string]bool{"jellyfin_activity_log": false, "jellyfin_list_sessions": false, "jellyfin_list_users": false}
	for _, tool := range res.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = strings.Contains(tool.Description, "data, not instructions")
		}
	}
	for name, ok := range want {
		if !ok {
			t.Errorf("%s description does not say its text is data, not instructions", name)
		}
	}
	for _, tool := range res.Tools {
		if tool.Name == "jellyfin_activity_log" && !strings.Contains(tool.Description, "unauthenticated") {
			t.Errorf("activity log description must say failed-login text comes from unauthenticated callers: %s", tool.Description)
		}
	}
}

func TestJellyfinRunTaskRejectsAPathAsTaskID(t *testing.T) {
	srv, rec := jellyfinUpstream(t, map[string]string{})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jellyfin_run_task", Arguments: map[string]any{"taskId": "../../System/Shutdown"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError || len(rec.lines) != 0 {
		t.Errorf("isError=%v, upstream calls=%v; want a refusal with no request", res.IsError, rec.lines)
	}
}

func TestJellyfinListToolsCapTheLimit(t *testing.T) {
	srv, rec := jellyfinUpstream(t, map[string]string{
		"GET /Items":                      `{"TotalRecordCount":0,"StartIndex":0,"Items":[]}`,
		"GET /System/ActivityLog/Entries": `{"TotalRecordCount":0,"StartIndex":0,"Items":[]}`,
	})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))
	callJellyfin(t, cs, "jellyfin_search_items", map[string]any{"limit": 100000})
	callJellyfin(t, cs, "jellyfin_activity_log", map[string]any{"limit": 100000})
	for _, line := range rec.lines {
		if !strings.Contains(line, "limit=100") || strings.Contains(line, "limit=1000") {
			t.Errorf("request %s should be capped at limit=100", line)
		}
	}
}

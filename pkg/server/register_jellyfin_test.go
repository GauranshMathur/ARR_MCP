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
	"jellyfin_list_media_segments",
	"jellyfin_list_episode_markers",
	"jellyfin_list_plugins",
	"jellyfin_list_introskipper_segments",
	"jellyfin_get_introskipper_analyzers",
	"jellyfin_get_introskipper_scan_status",
	"jellyfin_get_plugin_config",
}

var jellyfinWriteTools = []string{
	"jellyfin_scan_library",
	"jellyfin_refresh_item",
	"jellyfin_create_introskipper_segment",
	"jellyfin_update_introskipper_segment",
	"jellyfin_restore_introskipper_segment",
	"jellyfin_set_introskipper_analyzers",
}

// jellyfinDestructiveTools can delete data: scheduled tasks include cache, log,
// transcode and activity-log cleanups, and plugins add their own. Deleting a
// user segment is permanent, and a season scan erases every segment in the
// season, user segments included.
var jellyfinDestructiveTools = []string{
	"jellyfin_run_task",
	"jellyfin_delete_introskipper_segment",
	"jellyfin_scan_introskipper_season",
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
	for _, name := range jellyfinDestructiveTools {
		if tool := tools[name]; tool == nil || tool.Annotations.ReadOnlyHint || !*tool.Annotations.DestructiveHint {
			t.Errorf("%s: want a destructive tool, got %+v", name, tool)
		}
	}
	if want := len(jellyfinReadTools) + len(jellyfinWriteTools) + len(jellyfinDestructiveTools); len(tools) != want {
		t.Errorf("jellyfin tools = %d, want %d: %v", len(tools), want, tools)
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

func TestJellyfinMarkerToolsReachTheirEndpoints(t *testing.T) {
	srv, rec := jellyfinUpstream(t, map[string]string{
		"GET /MediaSegments/00cb39385fc82b4b4e9db35434770392": `{"Items":[{"Type":"Intro","StartTicks":1084000000,"EndTicks":12332320000}]}`,
		"GET /Items": `{"TotalRecordCount":249,"StartIndex":24,"Items":[{"Id":"00cb39385fc82b4b4e9db35434770392","Name":"Pilot",
		  "Type":"Episode","ParentIndexNumber":1,"IndexNumber":1,"Chapters":[{"StartPositionTicks":2438000000,"Name":"Intro"}]}]}`,
	})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))

	body := callJellyfin(t, cs, "jellyfin_list_media_segments", map[string]any{"itemId": "00cb39385fc82b4b4e9db35434770392"})
	if !strings.Contains(body, `"itemId":"00cb39385fc82b4b4e9db35434770392"`) || !strings.Contains(body, `"count":1`) ||
		!strings.Contains(body, `"startSeconds":108.4`) || !strings.Contains(body, `"endSeconds":1233.232`) {
		t.Errorf("segments result = %s", body)
	}

	body = callJellyfin(t, cs, "jellyfin_list_episode_markers", map[string]any{
		"parentId": "9993907cc9859aa5e0e31db2d60ce47a", "limit": 1, "startIndex": 24,
	})
	// The text content is re-encoded with sorted keys, so match fields, not order.
	if !strings.Contains(body, `"totalCount":249`) || !strings.Contains(body, `"chapters":[{"name":"Intro","startSeconds":243.8}]`) ||
		!strings.Contains(body, `"segments":[{"endSeconds":1233.232,"startSeconds":108.4,"type":"Intro"}]`) {
		t.Errorf("markers result = %s", body)
	}
	if len(rec.lines) != 3 || !strings.Contains(rec.lines[1], "parentId=9993907cc9859aa5e0e31db2d60ce47a") ||
		!strings.Contains(rec.lines[1], "limit=1") || !strings.Contains(rec.lines[1], "startIndex=24") {
		t.Errorf("requests = %v", rec.lines)
	}
}

func TestJellyfinListPluginsToolDropsDescriptions(t *testing.T) {
	srv, _ := jellyfinUpstream(t, map[string]string{
		"GET /Plugins": `[{"Name":"Intro Skipper","Version":"12.0.5.0","Description":"Long text a plugin author wrote.",
		  "Id":"c83d86bba1e04c35a113e2101cf4ee6b","CanUninstall":true,"HasImage":true,"Status":"Restart"}]`,
	})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))

	body := callJellyfin(t, cs, "jellyfin_list_plugins", nil)
	if strings.Contains(body, "plugin author wrote") || strings.Contains(body, "hasImage") {
		t.Errorf("description or image flag reached the client: %s", body)
	}
	if !strings.Contains(body, `"count":1`) || !strings.Contains(body, `"status":"Restart"`) || !strings.Contains(body, `"version":"12.0.5.0"`) {
		t.Errorf("result = %s", body)
	}
}

func TestJellyfinIntroSkipperToolsReachTheirEndpoints(t *testing.T) {
	const (
		ep     = "00cb39385fc82b4b4e9db35434770392"
		seg    = "01a14e5f37457e479775608f97681260"
		series = "9993907cc9859aa5e0e31db2d60ce47a"
		season = "5d6c3e1a9b8f4c2d8e7f6a5b4c3d2e1f"
	)
	segment := `{"Id":"` + seg + `","Type":"Credits","Start":1232.2,"End":1270.1,"Source":"User","Suppressed":false}`
	srv, rec := jellyfinUpstream(t, map[string]string{
		"GET /Episode/" + ep + "/Segments":                      `[` + segment + `]`,
		"POST /Episode/" + ep + "/Segments":                     segment,
		"PUT /Episode/" + ep + "/Segments/" + seg:               segment,
		"POST /Episode/" + ep + "/Segments/" + seg + "/Restore": segment,
		"DELETE /Episode/" + ep + "/Segments/" + seg:            "",
		"GET /Intros/AnalyzerActions/" + season:                 `{"Introduction":"Chromaprint","Credits":"Chapter"}`,
		"POST /Intros/AnalyzerActions/UpdateSeason":             "",
		"POST /Intros/ScanSeason/" + series + "/" + season:      "",
		"GET /Intros/ScanStatus/" + season:                      `{"isRunning":true,"isQueued":false,"failed":false}`,
	})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))

	body := callJellyfin(t, cs, "jellyfin_list_introskipper_segments", map[string]any{"itemId": ep, "includeSuppressed": true})
	if !strings.Contains(body, `"count":1`) || !strings.Contains(body, `"source":"User"`) {
		t.Errorf("list = %s", body)
	}
	body = callJellyfin(t, cs, "jellyfin_create_introskipper_segment", map[string]any{
		"itemId": ep, "type": "Credits", "startSeconds": 1232.2, "endSeconds": 1270.1,
	})
	if !strings.Contains(body, `"projection":"Applied"`) {
		t.Errorf("create = %s", body)
	}
	callJellyfin(t, cs, "jellyfin_update_introskipper_segment", map[string]any{
		"itemId": ep, "segmentId": seg, "startSeconds": 1230, "endSeconds": 1270.1,
	})
	callJellyfin(t, cs, "jellyfin_restore_introskipper_segment", map[string]any{"itemId": ep, "segmentId": seg})
	callJellyfin(t, cs, "jellyfin_delete_introskipper_segment", map[string]any{"itemId": ep, "segmentId": seg})
	body = callJellyfin(t, cs, "jellyfin_set_introskipper_analyzers", map[string]any{
		"seasonId": season, "actions": map[string]any{"Credits": "Chapter"},
	})
	if !strings.Contains(body, `"Credits":"Chapter"`) || !strings.Contains(body, `"seasonId":"`+season+`"`) {
		t.Errorf("set analyzers = %s", body)
	}
	callJellyfin(t, cs, "jellyfin_get_introskipper_analyzers", map[string]any{"seasonId": season})
	body = callJellyfin(t, cs, "jellyfin_scan_introskipper_season", map[string]any{"seriesId": series, "seasonId": season})
	if !strings.Contains(body, `"queued":true`) {
		t.Errorf("scan = %s", body)
	}
	body = callJellyfin(t, cs, "jellyfin_get_introskipper_scan_status", map[string]any{"seasonId": season})
	if !strings.Contains(body, `"scope":"season"`) || !strings.Contains(body, `"isRunning":true`) {
		t.Errorf("status = %s", body)
	}

	want := []string{
		"GET /Episode/" + ep + "/Segments?includeSuppressed=true",
		"POST /Episode/" + ep + "/Segments",
		"PUT /Episode/" + ep + "/Segments/" + seg,
		"POST /Episode/" + ep + "/Segments/" + seg + "/Restore",
		"DELETE /Episode/" + ep + "/Segments/" + seg,
		"GET /Intros/AnalyzerActions/" + season,
		"POST /Intros/AnalyzerActions/UpdateSeason",
		"GET /Intros/AnalyzerActions/" + season,
		"GET /Intros/AnalyzerActions/" + season,
		"POST /Intros/ScanSeason/" + series + "/" + season,
		"GET /Intros/ScanStatus/" + season,
	}
	if strings.Join(rec.lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("requests:\n%s\nwant:\n%s", strings.Join(rec.lines, "\n"), strings.Join(want, "\n"))
	}
}

// What makes the two destructive Intro Skipper tools destructive must be in
// their descriptions, where the model reads it before calling.
func TestJellyfinIntroSkipperDestructiveToolsSayWhatTheyErase(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	want := map[string][]string{
		"jellyfin_scan_introskipper_season":    {"erases every segment", "made by hand"},
		"jellyfin_delete_introskipper_segment": {"permanently", "restore"},
	}
	for _, tool := range res.Tools {
		for _, phrase := range want[tool.Name] {
			if !strings.Contains(tool.Description, phrase) {
				t.Errorf("%s description lacks %q: %s", tool.Name, phrase, tool.Description)
			}
		}
	}
}

// A secret under a key that is not allowlisted must not reach the client, and a
// plugin that is not allowlisted must not even be fetched.
func TestJellyfinPluginConfigToolNeverReturnsSecrets(t *testing.T) {
	srv, rec := jellyfinUpstream(t, map[string]string{
		"GET /Plugins/f69e946a4b3c4e9a8f0a8d7c1b2c4d9b/Configuration": `{"AutoSkipIntro":true,"JellyseerrApiKey":"sk-live-0123456789"}`,
		"GET /Plugins/958aad6637844d2ab89aa7b6fab6e25c/Configuration": `{"LdapBindPassword":"hunter2"}`,
	})
	cs := connect(t, jellyfinCfg(srv.URL, permsFull))

	body := callJellyfin(t, cs, "jellyfin_get_plugin_config", map[string]any{"pluginId": "f69e946a4b3c4e9a8f0a8d7c1b2c4d9b"})
	if strings.Contains(body, "sk-live") {
		t.Errorf("a withheld secret reached the client: %s", body)
	}
	if !strings.Contains(body, `"AutoSkipIntro":true`) || !strings.Contains(body, `"withheld":["JellyseerrApiKey"]`) {
		t.Errorf("result = %s", body)
	}

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "jellyfin_get_plugin_config", Arguments: map[string]any{"pluginId": "958aad6637844d2ab89aa7b6fab6e25c"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError || strings.Contains(contentText(res), "hunter2") {
		t.Errorf("isError=%v, text=%s; want a refusal without the password", res.IsError, contentText(res))
	}
	if len(rec.lines) != 1 {
		t.Errorf("requests = %v; the LDAP configuration must never be fetched", rec.lines)
	}
}

// These tools return names set by users and devices, so the warning belongs
// in the tool description, where the model reads it first.
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

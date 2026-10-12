package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	synoTestUser = "svc-admin"
	synoTestPass = `pa\ss(w0rd)+x`
	synoTestSID  = "SID-0123456789abcdefghijklmnopqrstuvwxyzABCDEF"
)

var synologyTools = []string{
	"synology_system_info",
	"synology_utilization",
	"synology_update_status",
	"synology_storage_overview",
	"synology_list_disks",
	"synology_list_shares",
	"synology_list_files",
	"synology_get_file_info",
	"synology_search_files",
	"synology_dir_size",
}

// synologyTextTools return share, file and folder names set by users.
var synologyTextTools = []string{
	"synology_list_shares",
	"synology_list_files",
	"synology_get_file_info",
	"synology_search_files",
	"synology_dir_size",
}

func synologyCfg(url string, perms config.Permissions) *config.Config {
	return cfgWith(map[string][]config.Instance{
		"synology": {{Name: "nas", URL: url, Username: synoTestUser, Password: synoTestPass, Default: true}},
	}, perms)
}

type synologyRecorder struct {
	mu    sync.Mutex
	forms []url.Values
	// deleted is set once a File Station delete start arrives: getinfo then
	// answers every path as missing, as DSM does after a delete.
	deleted bool
}

func (r *synologyRecorder) keys() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, f := range r.forms {
		out = append(out, f.Get("api")+" v"+f.Get("version")+" "+f.Get("method"))
	}
	return out
}

// synologyUpstream imitates DSM. routes maps "api method" to the raw answer.
func synologyUpstream(t *testing.T, routes map[string]string) (*httptest.Server, *synologyRecorder) {
	t.Helper()
	rec := &synologyRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(raw))
		rec.mu.Lock()
		rec.forms = append(rec.forms, form)
		if form.Get("api") == "SYNO.FileStation.Delete" && form.Get("method") == "start" {
			rec.deleted = true
		}
		deleted := rec.deleted
		rec.mu.Unlock()
		if r.URL.Path != "/webapi/entry.cgi" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if form.Get("method") == "login" {
			if form.Get("account") != synoTestUser || form.Get("passwd") != synoTestPass {
				_, _ = w.Write([]byte(`{"success":false,"error":{"code":400}}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"sid":"` + synoTestSID + `"}}`))
			return
		}
		if form.Get("_sid") != synoTestSID {
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":119}}`))
			return
		}
		body, ok := routes[form.Get("api")+" "+form.Get("method")]
		if ok && body == synologyIdentity {
			body = identityGetInfo(form.Get("path"), deleted)
		}
		if ok && body == synologyShares {
			body = `{"success":true,"data":{"total":3,"shares":[{"name":"docker","path":"/docker","additional":{"real_path":"/volume1/docker"}},{"name":"kube","path":"/kube","additional":{"real_path":"/volume1/kube"}},{"name":"NAS","path":"/NAS","additional":{"real_path":"/volume1/NAS"}}]}}`
		}
		if !ok {
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":103}}`))
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func callSynology(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) string {
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

func TestSynologyReadToolsAreAllAdvertisedAsRead(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, synologyCfg(srv.URL, permsFull))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	tools := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		if strings.HasPrefix(tool.Name, "synology_") && !has(synologyWriteTools, tool.Name) {
			tools[tool.Name] = tool
		}
	}
	if len(tools) != len(synologyTools) {
		t.Errorf("synology tools = %d, want %d: %v", len(tools), len(synologyTools), tools)
	}
	for _, name := range synologyTools {
		tool := tools[name]
		if tool == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s: want a read tool, got %+v", name, tool)
			continue
		}
		// synoCall is never a tool: nothing takes an arbitrary api or method.
		raw, _ := json.Marshal(tool.InputSchema)
		for _, banned := range []string{`"api"`, `"method"`, `"version"`, `"sid"`, `"password"`} {
			if strings.Contains(string(raw), banned) {
				t.Errorf("%s input schema exposes %s: %s", name, banned, raw)
			}
		}
	}
}

func TestSynologyToolsSurviveReadOnlyMode(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, synologyCfg(srv.URL, config.Permissions{
		Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny,
	})))
	for _, tool := range synologyTools {
		if !has(names, tool) {
			t.Errorf("readonly mode must still expose %q", tool)
		}
	}
}

func TestSynologyFreeTextToolsAreFlaggedAsData(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, synologyCfg(srv.URL, permsFull))
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
	for _, name := range synologyTextTools {
		if !flagged[name] {
			t.Errorf("%s: description and output schema must both say its text is data, not instructions", name)
		}
	}
}

func TestSynologyToolsCallTheRightApiVersionAndMethod(t *testing.T) {
	srv, rec := synologyUpstream(t, map[string]string{
		"SYNO.DSM.Info getinfo":              `{"success":true,"data":{"model":"DS423+","version_string":"DSM 7.4.1-90080","uptime":5}}`,
		"SYNO.Core.System info":              `{"success":true,"data":{"cpu_cores":"4"}}`,
		"SYNO.Core.System.Utilization get":   `{"success":true,"data":{"cpu":{"1min_load":173}}}`,
		"SYNO.Core.Upgrade.Server check":     `{"success":true,"data":{"available":false}}`,
		"SYNO.Storage.CGI.Storage load_info": `{"success":true,"data":{"storagePools":[{"id":"reuse_1","status":"normal"}],"volumes":[{"id":"volume_1","size":{"total":"100","used":"40"}}],"disks":[{"id":"sata1","name":"Drive 3","status":"normal"}]}}`,
		"SYNO.FileStation.List list_share":   `{"success":true,"data":{"total":1,"shares":[{"name":"NAS","path":"/NAS"}]}}`,
		"SYNO.FileStation.List list":         `{"success":true,"data":{"total":1,"files":[{"name":"Movies","path":"/NAS/Movies","isdir":true}]}}`,
		"SYNO.FileStation.List getinfo":      `{"success":true,"data":{"files":[{"name":"Movies","path":"/NAS/Movies","isdir":true}]}}`,
		"SYNO.FileStation.Search start":      `{"success":true,"data":{"taskid":"t1"}}`,
		"SYNO.FileStation.Search list":       `{"success":true,"data":{"finished":true,"total":1,"files":[{"name":"Movies","path":"/NAS/Movies","isdir":true}]}}`,
		"SYNO.FileStation.Search stop":       `{"success":true}`,
		"SYNO.FileStation.Search clean":      `{"success":true,"data":{}}`,
		"SYNO.FileStation.DirSize start":     `{"success":true,"data":{"taskid":"d1"}}`,
		"SYNO.FileStation.DirSize status":    `{"success":true,"data":{"finished":true,"num_dir":1,"num_file":2,"total_size":300}}`,
		"SYNO.FileStation.DirSize stop":      `{"success":true}`,
	})
	cs := connect(t, synologyCfg(srv.URL, permsFull))

	calls := []struct {
		tool string
		args map[string]any
		want []string
		out  string
	}{
		{"synology_system_info", nil, []string{"SYNO.DSM.Info v2 getinfo", "SYNO.Core.System v1 info"}, `"model":"DS423+"`},
		{"synology_utilization", nil, []string{"SYNO.Core.System.Utilization v1 get"}, `"loadAverage1m":1.73`},
		{"synology_update_status", nil, []string{"SYNO.Core.Upgrade.Server v1 check"}, `"updateAvailable":false`},
		{"synology_storage_overview", nil, []string{"SYNO.Storage.CGI.Storage v1 load_info"}, `"freeBytes":60`},
		{"synology_list_disks", nil, []string{"SYNO.Storage.CGI.Storage v1 load_info"}, `"name":"Drive 3"`},
		{"synology_list_shares", nil, []string{"SYNO.FileStation.List v2 list_share"}, `"name":"NAS"`},
		{"synology_list_files", map[string]any{"path": "/NAS"}, []string{"SYNO.FileStation.List v2 list"}, `"name":"Movies"`},
		{"synology_get_file_info", map[string]any{"path": "/NAS/Movies"}, []string{"SYNO.FileStation.List v2 getinfo"}, `"isDir":true`},
		{"synology_search_files", map[string]any{"path": "/NAS", "pattern": "mov"},
			[]string{"SYNO.FileStation.Search v2 start", "SYNO.FileStation.Search v2 list", "SYNO.FileStation.Search v2 stop", "SYNO.FileStation.Search v2 clean"}, `"total":1`},
		{"synology_dir_size", map[string]any{"path": "/NAS/Movies"},
			[]string{"SYNO.FileStation.List v2 getinfo", "SYNO.FileStation.DirSize v2 start", "SYNO.FileStation.DirSize v2 status", "SYNO.FileStation.DirSize v2 stop"}, `"totalBytes":300`},
	}
	for _, tc := range calls {
		before := len(rec.keys())
		body := callSynology(t, cs, tc.tool, tc.args)
		got := rec.keys()[before:]
		// The first tool call also logs in once; skip login lines.
		var calls []string
		for _, k := range got {
			if !strings.Contains(k, "SYNO.API.Auth") {
				calls = append(calls, k)
			}
		}
		if strings.Join(calls, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s calls = %v, want %v", tc.tool, calls, tc.want)
		}
		if !strings.Contains(body, tc.out) {
			t.Errorf("%s result = %s, want %s", tc.tool, body, tc.out)
		}
	}
	for _, k := range rec.keys() {
		if strings.Contains(k, "SYNO.API.Auth v7 login") {
			continue
		}
		if strings.Contains(k, "User") || strings.Contains(k, "ISCSI") || strings.Contains(k, "Group") || strings.Contains(k, "Permission") {
			t.Errorf("a tool reached %s", k)
		}
	}
}

func TestSynologyToolsRejectBadInputWithoutARequest(t *testing.T) {
	srv, rec := synologyUpstream(t, map[string]string{})
	cs := connect(t, synologyCfg(srv.URL, permsFull))
	for name, args := range map[string]map[string]any{
		"synology_list_files":    {"path": "/NAS/../kube"},
		"synology_get_file_info": {"path": "NAS/Movies"},
		"synology_dir_size":      {"path": "/"},
		"synology_search_files":  {"path": "../NAS", "pattern": "x"},
	} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool %s: %v", name, err)
		}
		if !res.IsError {
			t.Errorf("%s accepted %v", name, args)
		}
	}
	if keys := rec.keys(); len(keys) != 0 {
		t.Errorf("requests reached the NAS: %v", keys)
	}
}

// synologyIdentity as a route answers List getinfo the way DSM does when
// nothing is a link: every path's real_path is "/volume1" + path, except a name
// starting with "new", which does not exist yet (code 408).
const (
	synologyIdentity = "@identity"
	synologyShares   = "@shares"
)

func identityGetInfo(pathParam string, deleted bool) string {
	var paths []string
	_ = json.Unmarshal([]byte(pathParam), &paths)
	parts := make([]string, 0, len(paths))
	for _, p := range paths {
		if deleted || strings.HasPrefix(p[strings.LastIndex(p, "/")+1:], "new") {
			parts = append(parts, `{"path":`+strconv.Quote(p)+`,"code":408}`)
			continue
		}
		parts = append(parts, `{"path":`+strconv.Quote(p)+`,"isdir":true,"additional":{"real_path":`+strconv.Quote("/volume1"+p)+`,"type":"","mount_point_type":""}}`)
	}
	return `{"success":true,"data":{"files":[` + strings.Join(parts, ",") + `]}}`
}

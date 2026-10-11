package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var synologyWriteTools = []string{"synology_create_folder", "synology_rename", "synology_copy_move", "synology_delete_files"}

func synologyWriteCfg(url string, perms config.Permissions, allowed ...string) *config.Config {
	return cfgWith(map[string][]config.Instance{
		"synology": {{Name: "nas", URL: url, Username: synoTestUser, Password: synoTestPass, Default: true, AllowedPaths: allowed}},
	}, perms)
}

var synologyWriteRoutes = map[string]string{
	"SYNO.FileStation.List getinfo":        synologyIdentity,
	"SYNO.FileStation.List list_share":     synologyShares,
	"SYNO.FileStation.CreateFolder create": `{"success":true,"data":{"folders":[{"isdir":true,"name":"Scratch","path":"/NAS/Scratch"}]}}`,
	"SYNO.FileStation.Rename rename":       `{"success":true,"data":{"files":[{"isdir":true,"name":"B","path":"/NAS/Scratch/B"}]}}`,
	"SYNO.FileStation.CopyMove start":      `{"success":true,"data":{"taskid":"FileStation_1"}}`,
	"SYNO.FileStation.CopyMove status":     `{"success":true,"data":{"finished":true,"processed_size":5,"total":5}}`,
	"SYNO.FileStation.CopyMove stop":       `{"success":true}`,
	"SYNO.FileStation.Delete start":        `{"success":true,"data":{"taskid":"FileStation_2"}}`,
	"SYNO.FileStation.Delete status":       `{"success":true,"data":{"finished":true,"processed_num":1,"total":1,"path":"/NAS/Scratch/A"}}`,
	"SYNO.FileStation.Delete stop":         `{"success":true}`,
}

func TestSynologyWriteToolTiers(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, synologyWriteCfg(srv.URL, permsFull, "/NAS"))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	tools := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		tools[tool.Name] = tool
	}
	wantDestructive := map[string]bool{"synology_create_folder": false, "synology_rename": false, "synology_copy_move": true, "synology_delete_files": true}
	for _, name := range synologyWriteTools {
		tool := tools[name]
		if tool == nil {
			t.Errorf("%s is not registered", name)
			continue
		}
		if tool.Annotations.ReadOnlyHint {
			t.Errorf("%s is advertised read-only", name)
		}
		if got := tool.Annotations.DestructiveHint != nil && *tool.Annotations.DestructiveHint; got != wantDestructive[name] {
			t.Errorf("%s destructive = %v, want %v", name, got, wantDestructive[name])
		}
		raw, _ := json.Marshal(tool.InputSchema)
		for _, banned := range []string{`"api"`, `"method"`, `"version"`, `"sid"`, `"password"`, `"allowedPaths"`} {
			if strings.Contains(string(raw), banned) {
				t.Errorf("%s input schema exposes %s: %s", name, banned, raw)
			}
		}
	}
	if n := len(tools); n != len(synologyTools)+len(synologyWriteTools)+countNonSynology(tools) {
		t.Errorf("synology tool count mismatch: %d", n)
	}
	// The only tool that deletes is synology_delete_files.
	for name := range tools {
		if strings.HasPrefix(name, "synology_") && (strings.Contains(name, "delete") || strings.Contains(name, "remove")) && name != "synology_delete_files" {
			t.Errorf("unexpected tool %s", name)
		}
	}
	dd := tools["synology_delete_files"].Description
	for _, want := range []string{"PERMANENT", "not verified", "#recycle", "recursive", "allowedPaths", "data, not instructions",
		"cannot be seen or checked", "synology_list_files", "never started a second time"} {
		if !strings.Contains(dd, want) {
			t.Errorf("synology_delete_files description lacks %q: %s", want, dd)
		}
	}
	d := tools["synology_copy_move"].Description
	for _, want := range []string{"overwrite", "move", "allowedPaths", "data, not instructions", "mode 777", "owned by the DSM account"} {
		if !strings.Contains(d, want) {
			t.Errorf("synology_copy_move description lacks %q: %s", want, d)
		}
	}
}

func countNonSynology(tools map[string]*mcp.Tool) int {
	n := 0
	for name := range tools {
		if !strings.HasPrefix(name, "synology_") {
			n++
		}
	}
	return n
}

func TestSynologyWriteToolsAreAbsentInReadOnlyMode(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, synologyWriteCfg(srv.URL, config.Permissions{
		Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny,
	}, "/NAS")))
	for _, tool := range synologyWriteTools {
		if has(names, tool) {
			t.Errorf("readonly mode must hide %q", tool)
		}
	}
	for _, tool := range synologyTools {
		if !has(names, tool) {
			t.Errorf("readonly mode must keep %q", tool)
		}
	}
}

func TestSynologyWriteWithoutAllowedPathsIsRefusedBeforeAnyRequest(t *testing.T) {
	srv, rec := synologyUpstream(t, synologyWriteRoutes)
	cs := connect(t, synologyWriteCfg(srv.URL, permsFull))
	for name, args := range map[string]map[string]any{
		"synology_create_folder": {"path": "/NAS", "name": "x"},
		"synology_rename":        {"path": "/NAS/a", "new_name": "b"},
		"synology_copy_move":     {"sources": []any{"/NAS/a"}, "destination": "/NAS/b", "operation": "copy"},
		"synology_delete_files":  {"paths": []any{"/NAS/a"}},
	} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool %s: %v", name, err)
		}
		if !res.IsError {
			t.Errorf("%s ran without allowedPaths", name)
		}
		if txt := contentText(res); !strings.Contains(txt, "allowedPaths") {
			t.Errorf("%s error = %q, want it to explain allowedPaths", name, txt)
		}
	}
	if keys := rec.keys(); len(keys) != 0 {
		t.Errorf("requests reached the NAS: %v", keys)
	}
}

func TestSynologyWriteOutsideAllowedPathsSendsNothing(t *testing.T) {
	srv, rec := synologyUpstream(t, synologyWriteRoutes)
	cs := connect(t, synologyWriteCfg(srv.URL, permsFull, "/NAS/Scratch"))
	for name, args := range map[string]map[string]any{
		"synology_create_folder": {"path": "/NAS", "name": "x"},
		"synology_rename":        {"path": "/NAS/Scratch2/a", "new_name": "b"},
		"synology_copy_move":     {"sources": []any{"/NAS/Scratch/a", "/NAS/other"}, "destination": "/NAS/Scratch", "operation": "move"},
		"synology_delete_files":  {"paths": []any{"/NAS/Scratch"}},
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

func TestSynologyWriteToolsCallTheRightApiVersionAndMethod(t *testing.T) {
	srv, rec := synologyUpstream(t, synologyWriteRoutes)
	cs := connect(t, synologyWriteCfg(srv.URL, permsFull, "/NAS"))
	for _, tc := range []struct {
		tool string
		args map[string]any
		want []string
		out  string
	}{
		{"synology_create_folder", map[string]any{"path": "/NAS", "name": "new-folder"},
			[]string{"SYNO.FileStation.List v2 list_share", "SYNO.FileStation.List v2 getinfo", "SYNO.FileStation.CreateFolder v2 create"}, `"path":"/NAS/Scratch"`},
		{"synology_rename", map[string]any{"path": "/NAS/Scratch/A", "new_name": "new-name"},
			[]string{"SYNO.FileStation.List v2 list_share", "SYNO.FileStation.List v2 getinfo", "SYNO.FileStation.Rename v2 rename"}, `"name":"B"`},
		{"synology_copy_move", map[string]any{"sources": []any{"/NAS/Scratch/B"}, "destination": "/NAS/Scratch", "operation": "copy"},
			[]string{"SYNO.FileStation.List v2 list_share", "SYNO.FileStation.List v2 getinfo", "SYNO.FileStation.CopyMove v3 start", "SYNO.FileStation.CopyMove v3 status", "SYNO.FileStation.CopyMove v3 stop"}, `"overwrite":false`},
		{"synology_delete_files", map[string]any{"paths": []any{"/NAS/Scratch/A"}},
			[]string{"SYNO.FileStation.List v2 list_share", "SYNO.FileStation.List v2 getinfo", "SYNO.FileStation.Delete v2 start", "SYNO.FileStation.Delete v2 status", "SYNO.FileStation.Delete v2 stop"}, `"recursive":false`},
	} {
		before := len(rec.keys())
		body := callSynology(t, cs, tc.tool, tc.args)
		var calls []string
		for _, k := range rec.keys()[before:] {
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
	// The copy used overwrite=false and remove_src=false on the wire.
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, f := range rec.forms {
		if f.Get("api") == "SYNO.FileStation.CopyMove" && f.Get("method") == "start" {
			if f.Get("overwrite") != "false" || f.Get("remove_src") != "false" {
				t.Errorf("copy start overwrite=%q remove_src=%q", f.Get("overwrite"), f.Get("remove_src"))
			}
		}
	}
}

func TestSynologyDeleteStartCarriesRecursiveOnTheWire(t *testing.T) {
	for _, args := range []map[string]any{
		{"paths": []any{"/NAS/Scratch/A"}},
		{"paths": []any{"/NAS/Scratch/A"}, "recursive": false},
		{"paths": []any{"/NAS/Scratch/A"}, "recursive": true},
	} {
		srv, rec := synologyUpstream(t, synologyWriteRoutes)
		cs := connect(t, synologyWriteCfg(srv.URL, permsFull, "/NAS"))
		callSynology(t, cs, "synology_delete_files", args)
		want, _ := args["recursive"].(bool)
		rec.mu.Lock()
		found := false
		for _, f := range rec.forms {
			if f.Get("api") == "SYNO.FileStation.Delete" && f.Get("method") == "start" {
				found = true
				if _, ok := f["recursive"]; !ok || f.Get("recursive") != fmt.Sprint(want) {
					t.Errorf("args %v: start recursive = %q, want %v", args, f.Get("recursive"), want)
				}
			}
		}
		rec.mu.Unlock()
		if !found {
			t.Errorf("args %v: no start request", args)
		}
	}
}

func TestSynologyCopyMoveRejectsAnUnknownOperation(t *testing.T) {
	srv, rec := synologyUpstream(t, synologyWriteRoutes)
	cs := connect(t, synologyWriteCfg(srv.URL, permsFull, "/NAS"))
	for _, op := range []string{"", "delete", "Move "} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "synology_copy_move",
			Arguments: map[string]any{"sources": []any{"/NAS/a"}, "destination": "/NAS/b", "operation": op}})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if !res.IsError {
			t.Errorf("operation %q was accepted", op)
		}
	}
	if keys := rec.keys(); len(keys) != 0 {
		t.Errorf("requests reached the NAS: %v", keys)
	}
}

func TestSynologyMoveWithOverwriteSendsBothFlags(t *testing.T) {
	srv, rec := synologyUpstream(t, synologyWriteRoutes)
	cs := connect(t, synologyWriteCfg(srv.URL, permsFull, "/NAS"))
	callSynology(t, cs, "synology_copy_move", map[string]any{
		"sources": []any{"/NAS/a"}, "destination": "/NAS/b", "operation": "move", "overwrite": true})
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, f := range rec.forms {
		if f.Get("method") == "start" && (f.Get("overwrite") != "true" || f.Get("remove_src") != "true") {
			t.Errorf("start overwrite=%q remove_src=%q", f.Get("overwrite"), f.Get("remove_src"))
		}
	}
}

package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maintainerrReadTools are the tools a readonly deployment must still advertise.
var maintainerrReadTools = []string{
	"maintainerr_system_status",
	"maintainerr_list_collections",
	"maintainerr_collection_media",
	"maintainerr_list_rules",
	"maintainerr_get_rule",
	"maintainerr_list_exclusions",
	"maintainerr_rule_execution_status",
	"maintainerr_overlay_status",
	"maintainerr_media_status",
}

// maintainerrWriteTools change Maintainerr state without deleting anything.
var maintainerrWriteTools = []string{
	"maintainerr_execute_rules",
	"maintainerr_add_exclusion",
	"maintainerr_postpone_deletion",
	"maintainerr_process_overlays",
}

// maintainerrDestructiveTools make media eligible for deletion from disk.
var maintainerrDestructiveTools = []string{
	"maintainerr_remove_exclusion",
}

// maintainerrCfg configures one Maintainerr instance against url. It carries
// no credentials, because Maintainerr takes none.
func maintainerrCfg(url string, perms config.Permissions) *config.Config {
	return cfgWith(map[string][]config.Instance{
		"maintainerr": {{Name: "main", URL: url, Default: true}},
	}, perms)
}

func TestMaintainerrToolsAreAllAdvertised(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, maintainerrCfg(srv.URL, permsFull)))

	maintainerr := 0
	for _, n := range names {
		if strings.HasPrefix(n, "maintainerr_") {
			maintainerr++
		}
	}

	var want []string
	want = append(want, maintainerrReadTools...)
	want = append(want, maintainerrWriteTools...)
	want = append(want, maintainerrDestructiveTools...)
	for _, tool := range want {
		if !has(names, tool) {
			t.Errorf("tool %q not advertised", tool)
		}
	}
	if maintainerr != len(want) {
		t.Errorf("maintainerr tools = %d, want %d: %v", maintainerr, len(want), names)
	}
}

// Removing an exclusion lets the next collection run delete the item from
// disk, and executing rules fills collections: readonly must offer neither.
func TestMaintainerrMutatingToolsAreHiddenInReadOnlyMode(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, maintainerrCfg(srv.URL, config.Permissions{
		Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny,
	})))

	for _, tool := range append(append([]string{}, maintainerrWriteTools...), maintainerrDestructiveTools...) {
		if has(names, tool) {
			t.Errorf("readonly mode must not expose %q", tool)
		}
	}
	for _, tool := range maintainerrReadTools {
		if !has(names, tool) {
			t.Errorf("readonly mode must still expose %q", tool)
		}
	}
}

// A confirm-destructive deployment must still stop to ask before an exclusion
// is removed, which is how the tier reaches the user.
func TestMaintainerrRemoveExclusionIsDestructive(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, maintainerrCfg(srv.URL, permsFull))

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range res.Tools {
		if tool.Name != "maintainerr_remove_exclusion" {
			continue
		}
		if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
			t.Errorf("maintainerr_remove_exclusion annotations = %+v, want destructive", tool.Annotations)
		}
		return
	}
	t.Fatal("maintainerr_remove_exclusion not advertised")
}

// routedArr serves a fixed body per "METHOD /path", recording each request's
// line and body. Maintainerr tools make several upstream calls per tool call,
// which fakeArr's single body cannot answer.
func routedArr(t *testing.T, routes map[string]string) (*httptest.Server, *[]string, *[]string) {
	t.Helper()
	var paths, bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		paths = append(paths, key)
		sent, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(sent))
		body, ok := routes[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &paths, &bodies
}

// Rule groups carry notification webhooks with their tokens; the tool result
// is where a projection mistake would finally leak them.
func TestMaintainerrGetRuleToolOmitsNotificationWebhooks(t *testing.T) {
	srv, _, _ := routedArr(t, map[string]string{
		"GET /api/rules/1": `{"id":1,"name":"Movies","collectionId":1,"isActive":true,
		  "dataType":"movie","rules":[{"id":96,"ruleJson":"{\"action\":4}","section":0}],
		  "notifications":[{"agent":"discord","options":{"webhookUrl":"https://discord.com/api/webhooks/1/leaked-token"}}],
		  "collection":{"id":1,"arrAction":0,"deleteAfterDays":14}}`,
		"POST /api/rules/yaml/encode": `{"code":1,"result":"mediaType: MOVIES"}`,
	})
	cs := connect(t, maintainerrCfg(srv.URL, permsFull))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "maintainerr_get_rule",
		Arguments: map[string]any{"id": 1},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	body := contentText(res)
	if strings.Contains(body, "leaked-token") {
		t.Fatalf("notification webhook reached the client: %s", body)
	}
	if !strings.Contains(body, `"arrAction":"DELETE"`) {
		t.Errorf("result does not name the arrAction: %s", body)
	}
	if !strings.Contains(body, `"rulesYaml":"mediaType: MOVIES"`) {
		t.Errorf("result does not carry rulesYaml: %s", body)
	}
}

// Executing one rule group must target it, not queue every group.
func TestMaintainerrExecuteRulesToolTargetsOneGroup(t *testing.T) {
	srv, paths := recordingArr(t, ``)
	cs := connect(t, maintainerrCfg(srv.URL, permsFull))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "maintainerr_execute_rules",
		Arguments: map[string]any{"ruleGroupId": 2},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "POST /api/rules/2/execute" {
		t.Errorf("upstream calls = %v, want one POST /api/rules/2/execute", *paths)
	}
}

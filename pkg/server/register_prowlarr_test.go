package server

import (
	"context"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// prowlarrReadTools are the tools a readonly deployment must still advertise.
var prowlarrReadTools = []string{
	"prowlarr_list_indexers",
	"prowlarr_get_indexer",
	"prowlarr_list_indexer_schemas",
	"prowlarr_get_indexer_schema",
	"prowlarr_search",
	"prowlarr_list_applications",
	"prowlarr_list_download_clients",
	"prowlarr_list_app_profiles",
	"prowlarr_list_tags",
	"prowlarr_indexer_stats",
	"prowlarr_system_status",
	"prowlarr_health",
	"prowlarr_history",
}

// prowlarrWriteTools create or modify Prowlarr state.
var prowlarrWriteTools = []string{
	"prowlarr_add_indexer",
	"prowlarr_update_indexer",
	"prowlarr_test_indexer",
	"prowlarr_test_all_indexers",
	"prowlarr_sync_applications",
	"prowlarr_grab_release",
	"prowlarr_create_tag",
	"prowlarr_run_command",
}

// prowlarrDestructiveTools remove state and must never appear in readonly mode.
var prowlarrDestructiveTools = []string{
	"prowlarr_delete_indexer",
	"prowlarr_delete_tag",
}

// prowlarrCfg configures one Prowlarr instance against url.
func prowlarrCfg(url string, perms config.Permissions) *config.Config {
	return cfgWith(map[string][]config.Instance{
		"prowlarr": {{Name: "main", URL: url, APIKey: "k", Default: true}},
	}, perms)
}

func TestProwlarrToolsAreAllAdvertised(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, prowlarrCfg(srv.URL, permsFull)))

	prowlarr := 0
	for _, n := range names {
		if strings.HasPrefix(n, "prowlarr_") {
			prowlarr++
		}
	}

	var want []string
	want = append(want, prowlarrReadTools...)
	want = append(want, prowlarrWriteTools...)
	want = append(want, prowlarrDestructiveTools...)
	for _, tool := range want {
		if !has(names, tool) {
			t.Errorf("tool %q not advertised", tool)
		}
	}
	if prowlarr != len(want) {
		t.Errorf("prowlarr tools = %d, want %d: %v", prowlarr, len(want), names)
	}
}

// Editing an indexer or grabbing a release changes state, and deleting one
// unsyncs it from every connected app: a readonly deployment must offer neither.
func TestProwlarrMutatingToolsAreHiddenInReadOnlyMode(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, prowlarrCfg(srv.URL, config.Permissions{
		Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny,
	}))
	names := toolNames(t, cs)

	for _, tool := range append(append([]string{}, prowlarrWriteTools...), prowlarrDestructiveTools...) {
		if has(names, tool) {
			t.Errorf("readonly mode must not expose %q", tool)
		}
	}
	for _, tool := range prowlarrReadTools {
		if !has(names, tool) {
			t.Errorf("readonly mode must still expose %q", tool)
		}
	}
}

// The tool result is the last place a projection mistake shows up: an indexer's
// own API key must not reach the model even though the field name does.
func TestGetIndexerToolReturnsFieldNamesWithoutSecretValues(t *testing.T) {
	srv, _ := fakeArr(t, `{"id":15,"name":"altHUB","definitionName":"Newznab",
	  "implementation":"Newznab","protocol":"usenet","enable":true,"priority":24,
	  "fields":[
	    {"name":"baseUrl","label":"Url","privacy":"normal","value":"https://example.invalid"},
	    {"name":"apiKey","label":"API Key","privacy":"apiKey","value":"leaked-indexer-key"}
	  ]}`)
	cs := connect(t, prowlarrCfg(srv.URL, permsFull))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "prowlarr_get_indexer",
		Arguments: map[string]any{"id": 15},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	body := contentText(res)
	if !strings.Contains(body, "apiKey") {
		t.Errorf("result does not name the apiKey field: %s", body)
	}
	if strings.Contains(body, "leaked-indexer-key") {
		t.Fatalf("indexer credentials reached the client: %s", body)
	}
	if !strings.Contains(body, "https://example.invalid") {
		t.Errorf("non-secret field value was dropped: %s", body)
	}
}

// Grabbing must reach Prowlarr's search endpoint, not the download client.
func TestGrabReleaseToolPostsToTheSearchEndpoint(t *testing.T) {
	srv, paths := recordingArr(t, `{}`)
	cs := connect(t, prowlarrCfg(srv.URL, permsFull))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "prowlarr_grab_release",
		Arguments: map[string]any{"guid": "magnet:?xt=urn:btih:abc", "indexerId": 4},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "POST /api/v1/search" {
		t.Errorf("upstream calls = %v, want one POST /api/v1/search", *paths)
	}
}

package server

import (
	"context"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// nzbCfg configures one NZBGet instance against url. NZBGet authenticates
// with a username and password, not an API key.
func nzbCfg(url string, perms config.Permissions) *config.Config {
	return cfgWith(map[string][]config.Instance{
		"nzbget": {{Name: "main", URL: url, Username: "nzb", Password: "pw", Default: true}},
	}, perms)
}

// nzbReadTools are the read-tier NZBGet tools.
var nzbReadTools = []string{"nzbget_status", "nzbget_list_queue", "nzbget_history"}

// nzbWriteTools are the write-tier NZBGet tools.
var nzbWriteTools = []string{
	"nzbget_add_nzb", "nzbget_pause_download", "nzbget_resume_download",
	"nzbget_pause_items", "nzbget_resume_items", "nzbget_move_items",
	"nzbget_set_priority", "nzbget_set_category", "nzbget_rename_item",
	"nzbget_retry_history_items", "nzbget_mark_history_items",
	"nzbget_set_rate_limit", "nzbget_scan",
}

// nzbDestructiveTools are the destructive-tier NZBGet tools.
var nzbDestructiveTools = []string{"nzbget_delete_items", "nzbget_delete_history_items"}

func TestNZBGetToolsRegisterWhenConfigured(t *testing.T) {
	srv, _ := fakeArr(t, `{"version":"1.1","id":1,"result":[]}`)
	names := toolNames(t, connect(t, nzbCfg(srv.URL, permsFull)))

	for _, want := range nzbReadTools {
		if !has(names, want) {
			t.Errorf("read tool %q missing from %v", want, names)
		}
	}
	for _, want := range nzbWriteTools {
		if !has(names, want) {
			t.Errorf("write tool %q missing from %v", want, names)
		}
	}
	for _, want := range nzbDestructiveTools {
		if !has(names, want) {
			t.Errorf("destructive tool %q missing from %v", want, names)
		}
	}
	// NZBGet is configured but Sonarr is not, so no Sonarr tools may appear.
	if has(names, "sonarr_list_series") {
		t.Error("sonarr tools exposed without a configured sonarr instance")
	}
}

func TestNZBGetToolsAbsentWithoutAnInstance(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {{Name: "main", URL: srv.URL, APIKey: "k", Default: true}},
	}, permsFull))

	for _, name := range toolNames(t, cs) {
		if strings.HasPrefix(name, "nzbget_") {
			t.Errorf("tool %q exposed without a configured nzbget instance", name)
		}
	}
}

func TestNZBGetReadOnlyModeHidesMutatingTools(t *testing.T) {
	srv, _ := fakeArr(t, `{"version":"1.1","id":1,"result":[]}`)
	readonly := config.Permissions{Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny}
	names := toolNames(t, connect(t, nzbCfg(srv.URL, readonly)))

	for _, want := range nzbReadTools {
		if !has(names, want) {
			t.Errorf("readonly mode must still expose %q", want)
		}
	}
	for _, unwanted := range append(append([]string{}, nzbWriteTools...), nzbDestructiveTools...) {
		if has(names, unwanted) {
			t.Errorf("readonly mode must not expose %q", unwanted)
		}
	}
}

func TestNZBGetListQueueToolPostsJSONRPC(t *testing.T) {
	srv, paths := recordingArr(t, `{"version":"1.1","id":1,"result":[
	  {"NZBID":15803,"NZBName":"Some.Movie.2011","Status":"DOWNLOADING","Category":"Movies",
	   "FileSizeMB":34309,"RemainingSizeMB":1668,"Health":1000,"MaxPriority":0}]}`)
	cs := connect(t, nzbCfg(srv.URL, permsFull))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "nzbget_list_queue"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "POST /jsonrpc" {
		t.Errorf("upstream calls = %v, want one POST /jsonrpc", *paths)
	}
	body := contentText(res)
	for _, want := range []string{"Some.Movie.2011", "15803", "DOWNLOADING"} {
		if !strings.Contains(body, want) {
			t.Errorf("result does not contain %q: %s", want, body)
		}
	}
}

// Adding with both a URL and inline content (or neither) must be rejected
// before NZBGet is contacted.
func TestNZBGetAddNZBRequiresExactlyOneSource(t *testing.T) {
	srv, hits := fakeArr(t, `{"version":"1.1","id":1,"result":1}`)
	cs := connect(t, nzbCfg(srv.URL, permsFull))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "nzbget_add_nzb",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error when neither url nor content is given")
	}
	if *hits != 0 {
		t.Errorf("upstream contacted %d times for an invalid add", *hits)
	}
}

package server

import (
	"context"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// bazarrReadTools are the Bazarr tools that only read. They stay available in
// readonly mode.
var bazarrReadTools = []string{
	"bazarr_badges",
	"bazarr_wanted_episodes",
	"bazarr_wanted_movies",
	"bazarr_list_series",
	"bazarr_list_movies",
	"bazarr_list_episode_subtitles",
	"bazarr_list_providers",
	"bazarr_list_languages",
	"bazarr_health",
	"bazarr_system_status",
	"bazarr_list_language_profiles",
	"bazarr_manual_search_episode",
	"bazarr_manual_search_movie",
	"bazarr_episode_history",
	"bazarr_movie_history",
	"bazarr_list_blacklist",
	"bazarr_list_tasks",
	"bazarr_subtitle_info",
}

// bazarrMutatingTools change state, so readonly mode must not advertise them
// at all -- registering and then refusing would teach the model a tool exists
// when it does not.
var bazarrMutatingTools = []string{
	"bazarr_search_episode_subtitles",
	"bazarr_search_movie_subtitles",
	"bazarr_delete_episode_subtitle",
	"bazarr_delete_movie_subtitle",
	"bazarr_set_series_profile",
	"bazarr_set_movie_profile",
	"bazarr_series_action",
	"bazarr_movie_action",
	"bazarr_download_episode_subtitle",
	"bazarr_download_movie_subtitle",
	"bazarr_blacklist_subtitle",
	"bazarr_delete_blacklist_item",
	"bazarr_reset_providers",
	"bazarr_run_task",
	"bazarr_modify_subtitle",
}

func bazarrCfg(url string, perms config.Permissions) *config.Config {
	return cfgWith(map[string][]config.Instance{
		"bazarr": {{Name: "main", URL: url, APIKey: "k", Default: true}},
	}, perms)
}

func TestBazarrRegistersEveryTool(t *testing.T) {
	srv, _ := fakeArr(t, `{"data":[]}`)
	cs := connect(t, bazarrCfg(srv.URL, permsFull))

	names := toolNames(t, cs)
	want := append(append([]string{}, bazarrReadTools...), bazarrMutatingTools...)
	for _, tool := range want {
		if !has(names, tool) {
			t.Errorf("tool %q missing from %v", tool, names)
		}
	}

	registered := 0
	for _, n := range names {
		if strings.HasPrefix(n, "bazarr_") {
			registered++
		}
	}
	if registered != len(want) {
		t.Errorf("registered %d bazarr tools, want %d: %v", registered, len(want), names)
	}
}

func TestBazarrMutatingToolsHiddenInReadonlyMode(t *testing.T) {
	srv, _ := fakeArr(t, `{"data":[]}`)
	cs := connect(t, bazarrCfg(srv.URL, config.Permissions{
		Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny,
	}))

	names := toolNames(t, cs)
	for _, tool := range bazarrMutatingTools {
		if has(names, tool) {
			t.Errorf("readonly mode advertises the mutating tool %q", tool)
		}
	}
	for _, tool := range bazarrReadTools {
		if !has(names, tool) {
			t.Errorf("readonly mode hides the read tool %q", tool)
		}
	}
}

// Blacklisting deletes the subtitle file from disk before searching for a
// replacement, so it must be gated as destructive rather than as a write.
func TestBazarrBlacklistToolIsDestructive(t *testing.T) {
	srv, _ := fakeArr(t, `{"data":[]}`)
	cs := connect(t, bazarrCfg(srv.URL, permsFull))

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range res.Tools {
		if tool.Name != "bazarr_blacklist_subtitle" {
			continue
		}
		if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil ||
			!*tool.Annotations.DestructiveHint {
			t.Errorf("bazarr_blacklist_subtitle annotations = %+v, want a destructive hint", tool.Annotations)
		}
		return
	}
	t.Fatal("bazarr_blacklist_subtitle not advertised")
}

// The language profile ids are what the profile-assignment tools consume, so
// the listing must actually reach Bazarr and project its bare array.
func TestBazarrListLanguageProfilesCallsThrough(t *testing.T) {
	srv, paths := recordingArr(t, `[{"profileId":1,"name":"eng","cutoff":null,
	  "items":[{"id":1,"language":"en","audio_exclude":"False","hi":"False","forced":"False"}],
	  "mustContain":[],"mustNotContain":[],"originalFormat":0,"tag":null}]`)
	cs := connect(t, bazarrCfg(srv.URL, permsFull))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "bazarr_list_language_profiles",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "GET /api/system/languages/profiles" {
		t.Fatalf("upstream requests = %v, want one GET /api/system/languages/profiles", *paths)
	}
	body := contentText(res)
	for _, want := range []string{`"profileId":1`, `"name":"eng"`, `"language":"en"`, `"count":1`} {
		if !strings.Contains(body, want) {
			t.Errorf("result %s does not contain %s", body, want)
		}
	}
}

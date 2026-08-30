package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// profileToolSuffixes are the profile and configuration tools both media
// services must offer. Registering them from one factory is what keeps neither
// service quietly falling behind the other.
var profileToolSuffixes = []string{
	"_get_quality_profile", "_create_quality_profile", "_update_quality_profile",
	"_delete_quality_profile", "_get_custom_format", "_create_custom_format",
	"_update_custom_format", "_delete_custom_format", "_add_root_folder",
	"_delete_root_folder", "_update_naming_config", "_media_management_config",
	"_update_media_management_config", "_update_delay_profile",
}

func TestProfileToolsAreRegisteredForBothMediaServices(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, mediaCfg(srv.URL)))

	for _, suffix := range profileToolSuffixes {
		for _, svc := range []string{"sonarr", "radarr"} {
			if !has(names, svc+suffix) {
				t.Errorf("tool %q not advertised", svc+suffix)
			}
		}
	}
}

// Release profiles are managed for Sonarr only, so the write tools must not
// appear for Radarr even though both services answer the read.
func TestReleaseProfileWriteToolsAreSonarrOnly(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, mediaCfg(srv.URL)))

	for _, want := range []string{
		"sonarr_create_release_profile", "sonarr_update_release_profile",
		"sonarr_delete_release_profile",
	} {
		if !has(names, want) {
			t.Errorf("tool %q not advertised", want)
		}
	}
	for _, unwanted := range []string{
		"radarr_create_release_profile", "radarr_update_release_profile",
		"radarr_delete_release_profile",
	} {
		if has(names, unwanted) {
			t.Errorf("tool %q registered for the wrong service", unwanted)
		}
	}
}

// Deleting a quality profile or a root folder changes what the library does
// with every title using it, so those tools must never reach a readonly
// deployment.
func TestDestructiveProfileToolsAreHiddenInReadOnlyMode(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {{Name: "main", URL: srv.URL, APIKey: "k", Default: true}},
	}, config.Permissions{Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny}))

	names := toolNames(t, cs)
	for _, unwanted := range []string{
		"sonarr_delete_quality_profile", "sonarr_delete_custom_format",
		"sonarr_delete_root_folder", "sonarr_delete_release_profile",
		"sonarr_create_quality_profile", "sonarr_update_quality_profile",
		"sonarr_create_custom_format", "sonarr_update_custom_format",
		"sonarr_add_root_folder", "sonarr_update_naming_config",
		"sonarr_update_media_management_config", "sonarr_update_delay_profile",
		"sonarr_create_release_profile", "sonarr_update_release_profile",
	} {
		if has(names, unwanted) {
			t.Errorf("readonly mode must not expose %q", unwanted)
		}
	}
	for _, want := range []string{
		"sonarr_get_quality_profile", "sonarr_get_custom_format",
		"sonarr_media_management_config",
	} {
		if !has(names, want) {
			t.Errorf("readonly mode must still expose the read tool %q", want)
		}
	}
}

func TestGetQualityProfileToolReadsTheProfileRecord(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":7,"name":"WEB-1080p","items":[],"formatItems":[]}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_get_quality_profile",
		Arguments: map[string]any{"id": 7},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "GET /api/v3/qualityprofile/7" {
		t.Errorf("upstream calls = %v, want one GET /api/v3/qualityprofile/7", *paths)
	}
	if body := contentText(res); !strings.Contains(body, "WEB-1080p") {
		t.Errorf("result does not carry the profile: %s", body)
	}
}

// Creating a profile starts from the instance's own schema, so the qualities
// and per-service settings come from the service rather than from guesses.
func TestCreateQualityProfileToolReadsTheSchemaFirst(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":1,"name":"HD","cutoff":9,
	  "items":[{"quality":{"id":9,"name":"HDTV-1080p"},"items":[],"allowed":false}],
	  "formatItems":[]}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "radarr_create_quality_profile",
		Arguments: map[string]any{
			"name": "HD", "allowed": []any{"HDTV-1080p"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	want := []string{"GET /api/v3/qualityprofile/schema", "POST /api/v3/qualityprofile"}
	if len(*paths) != 2 || (*paths)[0] != want[0] || (*paths)[1] != want[1] {
		t.Errorf("upstream calls = %v, want %v", *paths, want)
	}
}

func TestUpdateQualityProfileToolReadsBeforeWriting(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":7,"name":"WEB-1080p","items":[],"formatItems":[]}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_update_quality_profile",
		Arguments: map[string]any{"id": 7, "name": "WEB-1080p v2"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	want := []string{"GET /api/v3/qualityprofile/7", "PUT /api/v3/qualityprofile/7"}
	if len(*paths) != 2 || (*paths)[0] != want[0] || (*paths)[1] != want[1] {
		t.Errorf("upstream calls = %v, want %v", *paths, want)
	}
}

func TestDeleteQualityProfileToolCallsTheProfileRoute(t *testing.T) {
	srv, paths := recordingArr(t, `{}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "radarr_delete_quality_profile",
		Arguments: map[string]any{"id": 3},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "DELETE /api/v3/qualityprofile/3" {
		t.Errorf("upstream calls = %v, want one DELETE /api/v3/qualityprofile/3", *paths)
	}
}

func TestCreateCustomFormatToolPostsTheFormat(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":9,"name":"x266","specifications":[]}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "sonarr_create_custom_format",
		Arguments: map[string]any{
			"name": "x266",
			"specifications": []any{map[string]any{
				"name":           "x266",
				"implementation": "ReleaseTitleSpecification",
				"required":       true,
				"fields":         map[string]any{"value": `\bx266\b`},
			}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "POST /api/v3/customformat" {
		t.Errorf("upstream calls = %v, want one POST /api/v3/customformat", *paths)
	}
}

func TestGetCustomFormatToolReadsTheFormatRecord(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":3,"name":"DV HDR10","specifications":[]}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "radarr_get_custom_format",
		Arguments: map[string]any{"id": 3},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "GET /api/v3/customformat/3" {
		t.Errorf("upstream calls = %v, want one GET /api/v3/customformat/3", *paths)
	}
}

func TestAddRootFolderToolPostsThePath(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":4,"path":"/NAS/Anime","accessible":true}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "radarr_add_root_folder",
		Arguments: map[string]any{"path": "/NAS/Anime"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "POST /api/v3/rootfolder" {
		t.Errorf("upstream calls = %v, want one POST /api/v3/rootfolder", *paths)
	}
}

func TestDeleteRootFolderToolCallsTheRootFolderRoute(t *testing.T) {
	srv, paths := recordingArr(t, `{}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_delete_root_folder",
		Arguments: map[string]any{"id": 4},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "DELETE /api/v3/rootfolder/4" {
		t.Errorf("upstream calls = %v, want one DELETE /api/v3/rootfolder/4", *paths)
	}
}

// The config editors read the record before writing it, or every setting they
// do not model would be reset on the instance.
func TestConfigUpdateToolsReadBeforeWriting(t *testing.T) {
	for _, tc := range []struct {
		tool, body string
		args       map[string]any
		want       []string
	}{
		{
			tool: "sonarr_update_naming_config",
			body: `{"id":1,"renameEpisodes":true,"standardEpisodeFormat":"{Series Title}"}`,
			args: map[string]any{"renameFiles": false},
			want: []string{"GET /api/v3/config/naming", "PUT /api/v3/config/naming/1"},
		},
		{
			tool: "radarr_update_naming_config",
			body: `{"id":1,"renameMovies":true,"standardMovieFormat":"{Movie Title}"}`,
			args: map[string]any{"renameFiles": false},
			want: []string{"GET /api/v3/config/naming", "PUT /api/v3/config/naming/1"},
		},
		{
			tool: "sonarr_update_media_management_config",
			body: `{"id":1,"recycleBin":"","copyUsingHardlinks":true}`,
			args: map[string]any{"recycleBin": "/NAS/.recycle"},
			want: []string{"GET /api/v3/config/mediamanagement", "PUT /api/v3/config/mediamanagement/1"},
		},
		{
			tool: "radarr_update_delay_profile",
			body: `{"id":1,"usenetDelay":0,"torrentDelay":0}`,
			args: map[string]any{"id": 1, "usenetDelay": 60},
			want: []string{"GET /api/v3/delayprofile/1", "PUT /api/v3/delayprofile/1"},
		},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			srv, paths := recordingArr(t, tc.body)
			cs := connect(t, mediaCfg(srv.URL))

			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
				Name: tc.tool, Arguments: tc.args,
			})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if res.IsError {
				t.Fatalf("tool returned an error: %s", contentText(res))
			}
			if len(*paths) != 2 || (*paths)[0] != tc.want[0] || (*paths)[1] != tc.want[1] {
				t.Errorf("upstream calls = %v, want %v", *paths, tc.want)
			}
		})
	}
}

func TestMediaManagementConfigToolReadsTheConfigRoute(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":1,"copyUsingHardlinks":true,"extraFileExtensions":"srt"}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "sonarr_media_management_config",
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "GET /api/v3/config/mediamanagement" {
		t.Errorf("upstream calls = %v, want one GET /api/v3/config/mediamanagement", *paths)
	}
	if body := contentText(res); !strings.Contains(body, "srt") {
		t.Errorf("result does not carry the config: %s", body)
	}
}

func TestCreateReleaseProfileToolPostsTheProfile(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":2,"name":"No x265","enabled":true,"ignored":["x265"],"tags":[]}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "sonarr_create_release_profile",
		Arguments: map[string]any{
			"name": "No x265", "ignored": []any{"x265"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "POST /api/v3/releaseprofile" {
		t.Errorf("upstream calls = %v, want one POST /api/v3/releaseprofile", *paths)
	}
}

func TestUpdateReleaseProfileToolReadsBeforeWriting(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":2,"name":"No x265","enabled":true,"ignored":["x265"],"tags":[]}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_update_release_profile",
		Arguments: map[string]any{"id": 2, "enabled": false},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	want := []string{"GET /api/v3/releaseprofile/2", "PUT /api/v3/releaseprofile/2"}
	if len(*paths) != 2 || (*paths)[0] != want[0] || (*paths)[1] != want[1] {
		t.Errorf("upstream calls = %v, want %v", *paths, want)
	}
}

func TestDeleteReleaseProfileToolCallsTheReleaseProfileRoute(t *testing.T) {
	srv, paths := recordingArr(t, `{}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_delete_release_profile",
		Arguments: map[string]any{"id": 2},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "DELETE /api/v3/releaseprofile/2" {
		t.Errorf("upstream calls = %v, want one DELETE /api/v3/releaseprofile/2", *paths)
	}
}

// A tool whose input schema cannot be built is dropped at registration with
// only a log line, so the custom format rules -- the one input carrying a free
// shape -- are checked to have made it into the advertised schema.
func TestCustomFormatToolAdvertisesItsSpecificationArgument(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range res.Tools {
		if tool.Name != "sonarr_create_custom_format" {
			continue
		}
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshalling input schema: %v", err)
		}
		for _, want := range []string{"specifications", "implementation", "fields"} {
			if !strings.Contains(string(schema), want) {
				t.Errorf("input schema has no %q argument: %s", want, schema)
			}
		}
		return
	}
	t.Fatal("sonarr_create_custom_format was not advertised at all")
}

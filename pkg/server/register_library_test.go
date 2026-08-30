package server

import (
	"context"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The point of registering these from one factory is that neither service can
// quietly fall behind the other.
func TestLibraryToolsAreRegisteredForBothMediaServices(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, mediaCfg(srv.URL)))

	for _, suffix := range []string{
		"_grab_release", "_grab_queue_item", "_mark_history_failed",
		"_manual_import_preview", "_manual_import", "_rename_files",
		"_update_files", "_update_tag", "_delete_queue_items",
	} {
		for _, svc := range []string{"sonarr", "radarr"} {
			if !has(names, svc+suffix) {
				t.Errorf("tool %q not advertised", svc+suffix)
			}
		}
	}
}

// Sonarr searches by episode and Radarr by movie, so each gets its own release
// search -- but both must exist.
func TestReleaseSearchIsRegisteredForBothMediaServices(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, mediaCfg(srv.URL)))

	for _, want := range []string{"sonarr_list_releases", "radarr_list_releases"} {
		if !has(names, want) {
			t.Errorf("tool %q not advertised", want)
		}
	}
}

// The episode-shaped and movie-shaped tools must not leak across: a
// radarr_get_series would take arguments Radarr cannot answer.
func TestServiceSpecificLibraryToolsAreNotCrossRegistered(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, mediaCfg(srv.URL)))

	for _, want := range []string{"sonarr_get_series", "radarr_get_movie", "radarr_update_collection"} {
		if !has(names, want) {
			t.Errorf("tool %q not advertised", want)
		}
	}
	for _, unwanted := range []string{
		"radarr_get_series", "sonarr_get_movie",
		"sonarr_update_collection", "sonarr_list_collections",
	} {
		if has(names, unwanted) {
			t.Errorf("tool %q registered for the wrong service", unwanted)
		}
	}
}

// Removing queue items can blocklist releases and tell the download client to
// delete data, so it is destructive and must never reach a readonly deployment.
func TestDeleteQueueItemsIsHiddenInReadOnlyMode(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {{Name: "main", URL: srv.URL, APIKey: "k", Default: true}},
	}, config.Permissions{Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny}))

	names := toolNames(t, cs)
	if has(names, "sonarr_delete_queue_items") {
		t.Error("readonly mode must not expose sonarr_delete_queue_items")
	}
	for _, want := range []string{"sonarr_list_releases", "sonarr_manual_import_preview", "sonarr_get_series"} {
		if !has(names, want) {
			t.Errorf("readonly mode must still expose the read tool %q", want)
		}
	}
	// Everything that writes to the library stays hidden too.
	for _, unwanted := range []string{
		"sonarr_grab_release", "sonarr_manual_import", "sonarr_rename_files",
		"sonarr_update_files", "sonarr_update_tag",
	} {
		if has(names, unwanted) {
			t.Errorf("readonly mode must not expose %q", unwanted)
		}
	}
}

func TestListReleasesToolSearchesTheReleaseEndpoint(t *testing.T) {
	srv, paths := recordingArr(t, `[{"guid":"g","indexerId":2,"title":"A release"}]`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_list_releases",
		Arguments: map[string]any{"episodeId": 13},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "GET /api/v3/release" {
		t.Errorf("upstream calls = %v, want one GET /api/v3/release", *paths)
	}
	if body := contentText(res); !strings.Contains(body, "A release") {
		t.Errorf("result does not carry the release: %s", body)
	}
}

func TestRadarrListReleasesToolSearchesTheReleaseEndpoint(t *testing.T) {
	srv, paths := recordingArr(t, `[]`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "radarr_list_releases",
		Arguments: map[string]any{"movieId": 1},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "GET /api/v3/release" {
		t.Errorf("upstream calls = %v, want one GET /api/v3/release", *paths)
	}
}

func TestGrabReleaseToolPostsToTheReleaseEndpoint(t *testing.T) {
	srv, paths := recordingArr(t, `{}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "radarr_grab_release",
		Arguments: map[string]any{"guid": "abc", "indexerId": 3},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "POST /api/v3/release" {
		t.Errorf("upstream calls = %v, want one POST /api/v3/release", *paths)
	}
}

// Importing runs the ManualImport command; a POST to /manualimport would only
// reprocess the candidates and import nothing.
func TestManualImportToolPostsTheManualImportCommand(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":1,"name":"ManualImport","status":"queued"}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "sonarr_manual_import",
		Arguments: map[string]any{
			"files": []any{map[string]any{
				"path":       "/downloads/Show/S01E01.mkv",
				"seriesId":   1,
				"episodeIds": []any{5},
			}},
			"importMode": "move",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "POST /api/v3/command" {
		t.Errorf("upstream calls = %v, want one POST /api/v3/command", *paths)
	}
}

func TestManualImportPreviewToolReadsTheManualImportEndpoint(t *testing.T) {
	srv, paths := recordingArr(t, `[]`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "radarr_manual_import_preview",
		Arguments: map[string]any{"downloadId": "ABC"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "GET /api/v3/manualimport" {
		t.Errorf("upstream calls = %v, want one GET /api/v3/manualimport", *paths)
	}
}

// The collection editor has to read the record before writing it, or the
// fields it does not model would be reset.
func TestUpdateCollectionToolReadsBeforeWriting(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":2,"title":"A Collection","monitored":true}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "radarr_update_collection",
		Arguments: map[string]any{"id": 2, "monitored": true},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	want := []string{"GET /api/v3/collection/2", "PUT /api/v3/collection/2"}
	if len(*paths) != 2 || (*paths)[0] != want[0] || (*paths)[1] != want[1] {
		t.Errorf("upstream calls = %v, want %v", *paths, want)
	}
}

func TestDeleteQueueItemsToolCallsTheBulkRoute(t *testing.T) {
	srv, paths := recordingArr(t, `{}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_delete_queue_items",
		Arguments: map[string]any{"ids": []any{1, 2}, "blocklist": true},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "DELETE /api/v3/queue/bulk" {
		t.Errorf("upstream calls = %v, want one DELETE /api/v3/queue/bulk", *paths)
	}
	if body := contentText(res); !strings.Contains(body, "2") {
		t.Errorf("result does not report how many were deleted: %s", body)
	}
}

func TestRenameFilesToolPostsACommand(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":1,"name":"RenameFiles","status":"queued"}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "radarr_rename_files",
		Arguments: map[string]any{"movieId": 3, "fileIds": []any{7}},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "POST /api/v3/command" {
		t.Errorf("upstream calls = %v, want one POST /api/v3/command", *paths)
	}
}

func TestUpdateTagToolPutsToTheTagRoute(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":3,"label":"kids"}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_update_tag",
		Arguments: map[string]any{"id": 3, "label": "kids"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "PUT /api/v3/tag/3" {
		t.Errorf("upstream calls = %v, want one PUT /api/v3/tag/3", *paths)
	}
}

func TestGetSeriesToolReadsTheSeriesRecord(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":1,"title":"Bob's Burgers","seasons":[]}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_get_series",
		Arguments: map[string]any{"id": 1},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "GET /api/v3/series/1" {
		t.Errorf("upstream calls = %v, want one GET /api/v3/series/1", *paths)
	}
}

func TestUpdateFilesToolUsesTheBulkFileEditor(t *testing.T) {
	srv, paths := recordingArr(t, `[]`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "radarr_update_files",
		Arguments: map[string]any{"fileIds": []any{7}, "releaseGroup": "ABM"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "PUT /api/v3/movieFile/bulk" {
		t.Errorf("upstream calls = %v, want one PUT /api/v3/movieFile/bulk", *paths)
	}
}

// The add tools gained options; the defaults they had before must survive an
// omitted argument, or every existing caller silently changes behaviour.
func TestAddSeriesKeepsItsDefaultsWhenTheNewOptionsAreOmitted(t *testing.T) {
	srv, paths := recordingArr(t, `{"id":1,"title":"X"}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "sonarr_add_series",
		Arguments: map[string]any{
			"tvdbId": 1, "qualityProfileId": 1, "rootFolderPath": "/tv",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if len(*paths) != 1 || (*paths)[0] != "POST /api/v3/series" {
		t.Errorf("upstream calls = %v, want one POST /api/v3/series", *paths)
	}
}

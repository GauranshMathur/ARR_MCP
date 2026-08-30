package arr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// exchange is one request a recording upstream received.
type exchange struct {
	method string
	path   string
	query  string
	body   string
}

// routedService answers each path from a routing table and records every
// request. The library calls fan out over several endpoints -- a manual import
// reads the quality and language tables before posting the command -- so
// fakeService, which keeps only the last body, cannot assert on them.
func routedService(t *testing.T, routes map[string]string) (*httptest.Server, *[]exchange) {
	t.Helper()
	var seen []exchange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent, _ := io.ReadAll(r.Body)
		seen = append(seen, exchange{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.RawQuery,
			body:   string(sent),
		})
		body, ok := routes[r.URL.Path]
		if !ok {
			body = "[]"
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

// sonarrClient builds a client for the fake upstream at url.
func sonarrClient(url string) *Client {
	return NewClient(url, SonarrSpec, Credentials{APIKey: "k"})
}

// radarrClient builds a Radarr client for the fake upstream at url.
func radarrClient(url string) *Client {
	return NewClient(url, RadarrSpec, Credentials{APIKey: "k"})
}

// find returns the first recorded request for method and path.
func find(t *testing.T, seen []exchange, method, path string) exchange {
	t.Helper()
	for _, e := range seen {
		if e.method == method && e.path == path {
			return e
		}
	}
	t.Fatalf("no %s %s in %v", method, path, seen)
	return exchange{}
}

// --- interactive release search ---

func TestSonarrListReleasesSearchesByEpisode(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/release": `[]`})
	id := 42

	if _, err := SonarrListReleases(context.Background(), sonarrClient(srv.URL), &id, nil, nil, 0); err != nil {
		t.Fatalf("SonarrListReleases: %v", err)
	}
	got := find(t, *seen, http.MethodGet, "/api/v3/release")
	if got.query != "episodeId=42" {
		t.Errorf("query = %q, want %q", got.query, "episodeId=42")
	}
}

func TestSonarrListReleasesSearchesBySeason(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/release": `[]`})
	series, season := 7, 3

	if _, err := SonarrListReleases(context.Background(), sonarrClient(srv.URL), nil, &series, &season, 0); err != nil {
		t.Fatalf("SonarrListReleases: %v", err)
	}
	got := find(t, *seen, http.MethodGet, "/api/v3/release")
	for _, want := range []string{"seriesId=7", "seasonNumber=3"} {
		if !strings.Contains(got.query, want) {
			t.Errorf("query %q does not contain %q", got.query, want)
		}
	}
	if strings.Contains(got.query, "episodeId") {
		t.Errorf("query %q sends an episode scope that was not asked for", got.query)
	}
}

// Without a scope Sonarr answers 404, which tells the model nothing about how
// to correct the call.
func TestSonarrListReleasesRequiresAScope(t *testing.T) {
	srv, seen := routedService(t, nil)

	_, err := SonarrListReleases(context.Background(), sonarrClient(srv.URL), nil, nil, nil, 0)
	if err == nil {
		t.Fatal("expected an error when no episode or season scope is given")
	}
	for _, want := range []string{"episodeId", "seriesId", "seasonNumber"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name the %q argument", err, want)
		}
	}
	if len(*seen) != 0 {
		t.Errorf("upstream contacted %d times for an unscoped search", len(*seen))
	}
}

func TestRadarrListReleasesSearchesByMovie(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/release": `[]`})

	if _, err := RadarrListReleases(context.Background(), radarrClient(srv.URL), 9, 0); err != nil {
		t.Fatalf("RadarrListReleases: %v", err)
	}
	if got := find(t, *seen, http.MethodGet, "/api/v3/release"); got.query != "movieId=9" {
		t.Errorf("query = %q, want %q", got.query, "movieId=9")
	}
}

// A real indexer search returns 200+ releases. Returning them all would spend
// the model's whole context on one call.
func TestListReleasesTrimsToTheRequestedLimit(t *testing.T) {
	srv, _ := routedService(t, map[string]string{"/api/v3/release": `[
	  {"guid":"a","indexerId":1,"title":"A"},
	  {"guid":"b","indexerId":1,"title":"B"},
	  {"guid":"c","indexerId":1,"title":"C"}]`})

	out, err := RadarrListReleases(context.Background(), radarrClient(srv.URL), 1, 2)
	if err != nil {
		t.Fatalf("RadarrListReleases: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("returned %d releases, want 2", len(out))
	}
}

// The trimmed view must keep exactly what a caller needs to choose a release
// and then grab it: the guid and indexer id are the grab arguments.
func TestListReleasesProjectsTheFieldsAGrabNeeds(t *testing.T) {
	srv, _ := routedService(t, map[string]string{"/api/v3/release": `[{
	  "guid":"https://indexer/1","indexerId":20,"indexer":"1337x",
	  "title":"Show.S16.1080p.WEB-DL","quality":{"quality":{"id":3,"name":"WEBDL-1080p"}},
	  "size":8286065655,"seeders":17,"leechers":19,"age":104,"protocol":"torrent",
	  "publishDate":"2026-05-17T16:00:00Z","approved":false,"rejected":true,
	  "rejections":["Wrong season"],"customFormatScore":1700,"seasonNumber":16,"fullSeason":true,
	  "overview":"a very long overview that must not survive the projection"}]`})

	out, err := SonarrListReleases(context.Background(), sonarrClient(srv.URL), ptr(1), nil, nil, 0)
	if err != nil {
		t.Fatalf("SonarrListReleases: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("returned %d releases, want 1", len(out))
	}
	got := out[0]
	if got.GUID != "https://indexer/1" || got.IndexerID != 20 {
		t.Errorf("grab arguments lost: guid=%q indexerId=%d", got.GUID, got.IndexerID)
	}
	if got.Quality != "WEBDL-1080p" {
		t.Errorf("quality = %q, want WEBDL-1080p", got.Quality)
	}
	if got.Seeders == nil || *got.Seeders != 17 {
		t.Errorf("seeders = %v, want 17", got.Seeders)
	}
	if len(got.Rejections) != 1 || got.Rejections[0] != "Wrong season" {
		t.Errorf("rejections = %v, want [Wrong season]", got.Rejections)
	}
	if got.SeasonNumber == nil || *got.SeasonNumber != 16 {
		t.Errorf("seasonNumber = %v, want 16", got.SeasonNumber)
	}
}

func TestGrabReleaseSendsGuidAndIndexerID(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/release": `{}`})

	if err := GrabRelease(context.Background(), sonarrClient(srv.URL), "abc-guid", 20); err != nil {
		t.Fatalf("GrabRelease: %v", err)
	}
	got := find(t, *seen, http.MethodPost, "/api/v3/release")
	var body map[string]any
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatalf("decoding grab body %q: %v", got.body, err)
	}
	if body["guid"] != "abc-guid" {
		t.Errorf("guid = %v, want abc-guid", body["guid"])
	}
	if body["indexerId"] != float64(20) {
		t.Errorf("indexerId = %v, want 20", body["indexerId"])
	}
}

func TestGrabQueueItemPostsToTheGrabRoute(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/queue/grab/5": `{}`})

	if err := GrabQueueItem(context.Background(), sonarrClient(srv.URL), 5); err != nil {
		t.Fatalf("GrabQueueItem: %v", err)
	}
	find(t, *seen, http.MethodPost, "/api/v3/queue/grab/5")
}

func TestMarkHistoryFailedPostsToTheFailedRoute(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/history/failed/8": `{}`})

	if err := MarkHistoryFailed(context.Background(), radarrClient(srv.URL), 8); err != nil {
		t.Fatalf("MarkHistoryFailed: %v", err)
	}
	find(t, *seen, http.MethodPost, "/api/v3/history/failed/8")
}

// --- manual import ---

func TestManualImportPreviewQueriesByDownloadID(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/manualimport": `[]`})

	_, err := ListManualImportCandidates(context.Background(), sonarrClient(srv.URL),
		ManualImportQuery{DownloadID: "ABC123", FilterExistingFiles: ptrBool(true)})
	if err != nil {
		t.Fatalf("ListManualImportCandidates: %v", err)
	}
	got := find(t, *seen, http.MethodGet, "/api/v3/manualimport")
	for _, want := range []string{"downloadId=ABC123", "filterExistingFiles=true"} {
		if !strings.Contains(got.query, want) {
			t.Errorf("query %q does not contain %q", got.query, want)
		}
	}
}

func TestManualImportPreviewQueriesByFolderAndSeries(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/manualimport": `[]`})

	_, err := ListManualImportCandidates(context.Background(), sonarrClient(srv.URL),
		ManualImportQuery{Folder: "/downloads/Show S01", SeriesID: ptr(4)})
	if err != nil {
		t.Fatalf("ListManualImportCandidates: %v", err)
	}
	got := find(t, *seen, http.MethodGet, "/api/v3/manualimport")
	for _, want := range []string{"folder=%2Fdownloads%2FShow+S01", "seriesId=4"} {
		if !strings.Contains(got.query, want) {
			t.Errorf("query %q does not contain %q", got.query, want)
		}
	}
}

// Sonarr answers a downloadId it cannot resolve to a path with a 500 and a
// stack trace, so the missing argument has to be caught here.
func TestManualImportPreviewRequiresFolderOrDownloadID(t *testing.T) {
	srv, seen := routedService(t, nil)

	_, err := ListManualImportCandidates(context.Background(), sonarrClient(srv.URL), ManualImportQuery{})
	if err == nil {
		t.Fatal("expected an error when neither folder nor downloadId is given")
	}
	for _, want := range []string{"folder", "downloadId"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name the %q argument", err, want)
		}
	}
	if len(*seen) != 0 {
		t.Errorf("upstream contacted %d times for an unscoped preview", len(*seen))
	}
}

func TestManualImportPreviewProjectsSeriesAndEpisodes(t *testing.T) {
	srv, _ := routedService(t, map[string]string{"/api/v3/manualimport": `[{
	  "id":607150990,"path":"/tv/Show/S08E01.mkv","relativePath":"Season 08/S08E01.mkv",
	  "size":301323920,"series":{"id":1,"title":"Bob's Burgers","overview":"long text"},
	  "seasonNumber":8,"episodes":[{"id":151,"seasonNumber":8,"episodeNumber":1}],
	  "quality":{"quality":{"id":3,"name":"WEBDL-1080p"}},
	  "languages":[{"id":1,"name":"English"}],"releaseGroup":"NTb","downloadId":"ABC",
	  "rejections":[]}]`})

	out, err := ListManualImportCandidates(context.Background(), sonarrClient(srv.URL),
		ManualImportQuery{Folder: "/tv/Show"})
	if err != nil {
		t.Fatalf("ListManualImportCandidates: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("returned %d candidates, want 1", len(out))
	}
	got := out[0]
	if got.SeriesID != 1 || got.SeriesTitle != "Bob's Burgers" {
		t.Errorf("series lost: id=%d title=%q", got.SeriesID, got.SeriesTitle)
	}
	if len(got.EpisodeIDs) != 1 || got.EpisodeIDs[0] != 151 {
		t.Errorf("episodeIds = %v, want [151]", got.EpisodeIDs)
	}
	if got.Quality != "WEBDL-1080p" {
		t.Errorf("quality = %q, want WEBDL-1080p", got.Quality)
	}
	if len(got.Languages) != 1 || got.Languages[0] != "English" {
		t.Errorf("languages = %v, want [English]", got.Languages)
	}
}

// The rejection field is a list of plain strings on /release but a list of
// {reason,type} objects on /manualimport. Decoding only one shape would make
// the whole call fail against the other.
func TestManualImportPreviewDecodesObjectRejections(t *testing.T) {
	srv, _ := routedService(t, map[string]string{"/api/v3/manualimport": `[{
	  "id":1,"path":"/tv/x.mkv",
	  "rejections":[{"reason":"Unknown series","type":"permanent"}]}]`})

	out, err := ListManualImportCandidates(context.Background(), sonarrClient(srv.URL),
		ManualImportQuery{Folder: "/tv"})
	if err != nil {
		t.Fatalf("ListManualImportCandidates: %v", err)
	}
	if len(out) != 1 || len(out[0].Rejections) != 1 {
		t.Fatalf("rejections = %+v, want one entry", out)
	}
	if out[0].Rejections[0] != "Unknown series" {
		t.Errorf("rejection = %q, want %q", out[0].Rejections[0], "Unknown series")
	}
}

// The import itself is the ManualImport *command*, not a POST to /manualimport
// -- that route only reprocesses candidates and imports nothing. Posting there
// would report success and leave the file where it was.
func TestManualImportPostsTheManualImportCommand(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/qualitydefinition": `[{"id":4,"title":"WEBDL-1080p","quality":{"id":3,"name":"WEBDL-1080p"}}]`,
		"/api/v3/language":          `[{"id":1,"name":"English"}]`,
		"/api/v3/command":           `{"id":9,"name":"ManualImport","status":"queued"}`,
	})

	_, err := ManualImport(context.Background(), sonarrClient(srv.URL), []ManualImportFile{{
		Path:         "/downloads/Show/S08E01.mkv",
		SeriesID:     1,
		EpisodeIDs:   []int{151},
		Quality:      "WEBDL-1080p",
		Languages:    []string{"English"},
		ReleaseGroup: "NTb",
		DownloadID:   "ABC",
	}}, "move")
	if err != nil {
		t.Fatalf("ManualImport: %v", err)
	}

	got := find(t, *seen, http.MethodPost, "/api/v3/command")
	var body struct {
		Name       string `json:"name"`
		ImportMode string `json:"importMode"`
		Files      []struct {
			Path       string `json:"path"`
			SeriesID   int    `json:"seriesId"`
			EpisodeIDs []int  `json:"episodeIds"`
			DownloadID string `json:"downloadId"`
			Quality    struct {
				Quality struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"quality"`
			} `json:"quality"`
			Languages []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"languages"`
			ReleaseGroup string `json:"releaseGroup"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatalf("decoding command body %q: %v", got.body, err)
	}

	if body.Name != "ManualImport" {
		t.Errorf("command name = %q, want ManualImport", body.Name)
	}
	if body.ImportMode != "move" {
		t.Errorf("importMode = %q, want move", body.ImportMode)
	}
	if len(body.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(body.Files))
	}
	f := body.Files[0]
	if f.Path != "/downloads/Show/S08E01.mkv" || f.SeriesID != 1 {
		t.Errorf("file identity lost: %+v", f)
	}
	if len(f.EpisodeIDs) != 1 || f.EpisodeIDs[0] != 151 {
		t.Errorf("episodeIds = %v, want [151]", f.EpisodeIDs)
	}
	// The name the caller gave has to become the {id,name} object the API wants.
	if f.Quality.Quality.ID != 3 || f.Quality.Quality.Name != "WEBDL-1080p" {
		t.Errorf("quality = %+v, want id 3 named WEBDL-1080p", f.Quality)
	}
	if len(f.Languages) != 1 || f.Languages[0].ID != 1 || f.Languages[0].Name != "English" {
		t.Errorf("languages = %+v, want [{1 English}]", f.Languages)
	}
	if f.ReleaseGroup != "NTb" || f.DownloadID != "ABC" {
		t.Errorf("releaseGroup/downloadId lost: %+v", f)
	}
}

// The quality id the API wants is quality.id inside a definition, not the
// definition's own id -- they differ (WEBDL-480p is definition 4, quality 8).
func TestManualImportUsesTheQualityIDNotTheDefinitionID(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/qualitydefinition": `[{"id":4,"title":"WEBDL-480p","quality":{"id":8,"name":"WEBDL-480p"}}]`,
		"/api/v3/command":           `{"id":1,"name":"ManualImport"}`,
	})

	_, err := ManualImport(context.Background(), radarrClient(srv.URL), []ManualImportFile{{
		Path: "/downloads/m.mkv", MovieID: 3, Quality: "WEBDL-480p",
	}}, "auto")
	if err != nil {
		t.Fatalf("ManualImport: %v", err)
	}
	got := find(t, *seen, http.MethodPost, "/api/v3/command")
	if !strings.Contains(got.body, `"id":8`) {
		t.Errorf("body %q does not carry quality id 8", got.body)
	}
	if strings.Contains(got.body, `"id":4`) {
		t.Errorf("body %q sent the quality definition id instead of the quality id", got.body)
	}
}

func TestManualImportRejectsAnUnknownQualityWithTheValidNames(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/qualitydefinition": `[{"id":4,"title":"WEBDL-1080p","quality":{"id":3,"name":"WEBDL-1080p"}}]`,
	})

	_, err := ManualImport(context.Background(), sonarrClient(srv.URL), []ManualImportFile{{
		Path: "/x.mkv", SeriesID: 1, Quality: "Bluray-2160p",
	}}, "auto")
	if err == nil {
		t.Fatal("expected an unknown quality name to be rejected")
	}
	if !strings.Contains(err.Error(), "WEBDL-1080p") {
		t.Errorf("error %q does not list the qualities the instance knows", err)
	}
	for _, e := range *seen {
		if e.method == http.MethodPost {
			t.Errorf("posted the import command despite an unresolvable quality: %v", e)
		}
	}
}

func TestManualImportRejectsAnUnknownImportMode(t *testing.T) {
	srv, seen := routedService(t, nil)

	_, err := ManualImport(context.Background(), sonarrClient(srv.URL),
		[]ManualImportFile{{Path: "/x.mkv", SeriesID: 1}}, "teleport")
	if err == nil {
		t.Fatal("expected an unknown import mode to be rejected")
	}
	if !strings.Contains(err.Error(), "move") {
		t.Errorf("error %q does not list the valid modes", err)
	}
	if len(*seen) != 0 {
		t.Errorf("upstream contacted %d times for an invalid mode", len(*seen))
	}
}

// --- collections ---

// The collection resource carries fields this package does not model. A typed
// round trip would drop them and silently reset them on the instance.
func TestRadarrUpdateCollectionPreservesUnknownKeys(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/collection/2": `{"id":2,"title":"Krrish Collection","monitored":false,
		  "qualityProfileId":1,"rootFolderPath":"/movies","searchOnAdd":false,
		  "minimumAvailability":"released","sortTitle":"krrish collection",
		  "overview":"upstream field this build does not model","movies":[{"tmdbId":1}]}`,
	})

	_, err := RadarrUpdateCollection(context.Background(), radarrClient(srv.URL), CollectionUpdate{
		ID: 2, Monitored: ptrBool(true), QualityProfileID: ptr(7),
	})
	if err != nil {
		t.Fatalf("RadarrUpdateCollection: %v", err)
	}

	got := find(t, *seen, http.MethodPut, "/api/v3/collection/2")
	var body map[string]any
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatalf("decoding collection body %q: %v", got.body, err)
	}
	if body["monitored"] != true {
		t.Errorf("monitored = %v, want true", body["monitored"])
	}
	if body["qualityProfileId"] != float64(7) {
		t.Errorf("qualityProfileId = %v, want 7", body["qualityProfileId"])
	}
	if body["sortTitle"] != "krrish collection" {
		t.Errorf("read-modify-write dropped sortTitle: %v", body)
	}
	if _, ok := body["overview"]; !ok {
		t.Errorf("read-modify-write dropped an unmodelled upstream key: %v", body)
	}
	// Untouched settings must survive unchanged rather than be reset.
	if body["minimumAvailability"] != "released" {
		t.Errorf("minimumAvailability = %v, want released", body["minimumAvailability"])
	}
	if body["searchOnAdd"] != false {
		t.Errorf("searchOnAdd = %v, want false", body["searchOnAdd"])
	}
}

// --- renames, file edits, tags and the queue ---

func TestSonarrRenameFilesPostsTheCommandWithSeriesAndFiles(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/command": `{"id":1,"name":"RenameFiles"}`})

	if _, err := SonarrRenameFiles(context.Background(), sonarrClient(srv.URL), 5, []int{11, 12}); err != nil {
		t.Fatalf("SonarrRenameFiles: %v", err)
	}
	got := find(t, *seen, http.MethodPost, "/api/v3/command")
	var body struct {
		Name     string `json:"name"`
		SeriesID int    `json:"seriesId"`
		Files    []int  `json:"files"`
	}
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatalf("decoding rename body %q: %v", got.body, err)
	}
	if body.Name != "RenameFiles" || body.SeriesID != 5 {
		t.Errorf("rename body = %+v, want RenameFiles for series 5", body)
	}
	if len(body.Files) != 2 || body.Files[0] != 11 {
		t.Errorf("files = %v, want [11 12]", body.Files)
	}
}

// Radarr keys the same command on movieId, not seriesId.
func TestRadarrRenameFilesPostsTheCommandWithMovieAndFiles(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/command": `{"id":1,"name":"RenameFiles"}`})

	if _, err := RadarrRenameFiles(context.Background(), radarrClient(srv.URL), 3, []int{7}); err != nil {
		t.Fatalf("RadarrRenameFiles: %v", err)
	}
	got := find(t, *seen, http.MethodPost, "/api/v3/command")
	var body struct {
		Name    string `json:"name"`
		MovieID int    `json:"movieId"`
		Files   []int  `json:"files"`
	}
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatalf("decoding rename body %q: %v", got.body, err)
	}
	if body.Name != "RenameFiles" || body.MovieID != 3 {
		t.Errorf("rename body = %+v, want RenameFiles for movie 3", body)
	}
	if len(body.Files) != 1 || body.Files[0] != 7 {
		t.Errorf("files = %v, want [7]", body.Files)
	}
}

func TestRenameFilesRequiresFileIDs(t *testing.T) {
	srv, seen := routedService(t, nil)

	if _, err := SonarrRenameFiles(context.Background(), sonarrClient(srv.URL), 5, nil); err == nil {
		t.Fatal("expected an error when no file ids are given")
	}
	if len(*seen) != 0 {
		t.Errorf("upstream contacted %d times with no file ids", len(*seen))
	}
}

// The bulk editor takes a bare array of partial file resources; every key the
// caller did not name must stay absent so the instance keeps its value.
func TestSonarrUpdateEpisodeFilesSendsPartialResources(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/qualitydefinition": `[{"id":4,"title":"Bluray-1080p","quality":{"id":7,"name":"Bluray-1080p"}}]`,
		"/api/v3/episodeFile/bulk":  `[{"id":11,"seriesId":1,"relativePath":"a.mkv"}]`,
	})

	group := "FLUX"
	_, err := SonarrUpdateEpisodeFiles(context.Background(), sonarrClient(srv.URL),
		[]int{11}, ptrString("Bluray-1080p"), nil, &group)
	if err != nil {
		t.Fatalf("SonarrUpdateEpisodeFiles: %v", err)
	}

	got := find(t, *seen, http.MethodPut, "/api/v3/episodeFile/bulk")
	var body []map[string]any
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatalf("decoding editor body %q: %v", got.body, err)
	}
	if len(body) != 1 {
		t.Fatalf("body carried %d files, want 1", len(body))
	}
	if body[0]["id"] != float64(11) {
		t.Errorf("id = %v, want 11", body[0]["id"])
	}
	if body[0]["releaseGroup"] != "FLUX" {
		t.Errorf("releaseGroup = %v, want FLUX", body[0]["releaseGroup"])
	}
	if _, ok := body[0]["languages"]; ok {
		t.Errorf("body sent languages the caller never named: %v", body[0])
	}
	quality, ok := body[0]["quality"].(map[string]any)
	if !ok {
		t.Fatalf("quality is not an object: %v", body[0]["quality"])
	}
	inner, ok := quality["quality"].(map[string]any)
	if !ok || inner["id"] != float64(7) {
		t.Errorf("quality = %v, want the nested {id:7} object", quality)
	}
}

func TestRadarrUpdateMovieFilesUsesTheMovieFileEditor(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/movieFile/bulk": `[]`})

	group := "ABM"
	if _, err := RadarrUpdateMovieFiles(context.Background(), radarrClient(srv.URL),
		[]int{7}, nil, nil, &group); err != nil {
		t.Fatalf("RadarrUpdateMovieFiles: %v", err)
	}
	find(t, *seen, http.MethodPut, "/api/v3/movieFile/bulk")
}

func TestUpdateFilesRequiresFileIDs(t *testing.T) {
	srv, seen := routedService(t, nil)

	if _, err := RadarrUpdateMovieFiles(context.Background(), radarrClient(srv.URL), nil, nil, nil, nil); err == nil {
		t.Fatal("expected an error when no file ids are given")
	}
	if len(*seen) != 0 {
		t.Errorf("upstream contacted %d times with no file ids", len(*seen))
	}
}

func TestUpdateTagPutsTheLabelToTheTagRoute(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/tag/3": `{"id":3,"label":"kids"}`})

	got, err := UpdateTag(context.Background(), sonarrClient(srv.URL), 3, "kids")
	if err != nil {
		t.Fatalf("UpdateTag: %v", err)
	}
	if got.Label != "kids" {
		t.Errorf("label = %q, want kids", got.Label)
	}
	sent := find(t, *seen, http.MethodPut, "/api/v3/tag/3")
	var body map[string]any
	if err := json.Unmarshal([]byte(sent.body), &body); err != nil {
		t.Fatalf("decoding tag body %q: %v", sent.body, err)
	}
	// Sonarr rejects the update when the body's id disagrees with the route.
	if body["id"] != float64(3) || body["label"] != "kids" {
		t.Errorf("tag body = %v, want id 3 labelled kids", body)
	}
}

// The bulk route takes the ids in a body and the flags in the query string.
func TestDeleteQueueItemsSendsIDsInTheBodyAndFlagsInTheQuery(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/queue/bulk": `{}`})

	n, err := DeleteQueueItems(context.Background(), sonarrClient(srv.URL), []int{1, 2, 3}, true, true)
	if err != nil {
		t.Fatalf("DeleteQueueItems: %v", err)
	}
	if n != 3 {
		t.Errorf("deleted = %d, want 3", n)
	}
	got := find(t, *seen, http.MethodDelete, "/api/v3/queue/bulk")
	for _, want := range []string{"removeFromClient=true", "blocklist=true"} {
		if !strings.Contains(got.query, want) {
			t.Errorf("query %q does not contain %q", got.query, want)
		}
	}
	var body struct {
		IDs []int `json:"ids"`
	}
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatalf("decoding bulk body %q: %v", got.body, err)
	}
	if len(body.IDs) != 3 || body.IDs[2] != 3 {
		t.Errorf("ids = %v, want [1 2 3]", body.IDs)
	}
}

func TestDeleteQueueItemsRequiresIDs(t *testing.T) {
	srv, seen := routedService(t, nil)

	if _, err := DeleteQueueItems(context.Background(), sonarrClient(srv.URL), nil, false, false); err == nil {
		t.Fatal("expected an error when no queue ids are given")
	}
	if len(*seen) != 0 {
		t.Errorf("upstream contacted %d times with no ids", len(*seen))
	}
}

// --- detail views ---

func TestSonarrGetSeriesKeepsSeasonsAndStatisticsAndDropsTheOverview(t *testing.T) {
	srv, _ := routedService(t, map[string]string{"/api/v3/series/1": `{
	  "id":1,"title":"Bob's Burgers","year":2011,"status":"continuing","monitored":true,
	  "tvdbId":194031,"path":"/tv/Bob","rootFolderPath":"/tv","qualityProfileId":7,
	  "seriesType":"standard","seasonFolder":true,"monitorNewItems":"all","network":"FOX",
	  "runtime":22,"tags":[9,8],"overview":"a paragraph that must not survive",
	  "seasons":[{"seasonNumber":1,"monitored":true,
	    "statistics":{"episodeCount":13,"episodeFileCount":13,"totalEpisodeCount":13,
	      "sizeOnDisk":6780861004,"percentOfEpisodes":100}}],
	  "statistics":{"seasonCount":16,"episodeCount":313,"episodeFileCount":313,
	    "totalEpisodeCount":323,"sizeOnDisk":218651985866,"percentOfEpisodes":100}}`})

	got, err := SonarrGetSeries(context.Background(), sonarrClient(srv.URL), 1)
	if err != nil {
		t.Fatalf("SonarrGetSeries: %v", err)
	}
	if got.Title != "Bob's Burgers" || got.SeriesType != "standard" {
		t.Errorf("series identity lost: %+v", got)
	}
	if len(got.Seasons) != 1 || got.Seasons[0].EpisodeFileCount != 13 {
		t.Errorf("seasons = %+v, want one season with 13 files", got.Seasons)
	}
	if got.Statistics.SeasonCount != 16 || got.Statistics.SizeOnDisk != 218651985866 {
		t.Errorf("statistics = %+v", got.Statistics)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshalling detail: %v", err)
	}
	if strings.Contains(string(raw), "must not survive") {
		t.Errorf("the overview reached the projection: %s", raw)
	}
}

func TestRadarrGetMovieSummarisesTheMovieFile(t *testing.T) {
	srv, _ := routedService(t, map[string]string{"/api/v3/movie/1": `{
	  "id":1,"title":"Whiplash","year":2014,"status":"released","monitored":true,
	  "hasFile":true,"tmdbId":244786,"imdbId":"tt2582802","path":"/movies/Whiplash",
	  "rootFolderPath":"/movies","qualityProfileId":9,"minimumAvailability":"released",
	  "runtime":107,"sizeOnDisk":13570037130,"tags":[9],"isAvailable":true,
	  "overview":"a paragraph that must not survive",
	  "collection":{"title":"A Collection","tmdbId":1},
	  "movieFile":{"id":7,"relativePath":"Whiplash.mkv","size":13570037130,
	    "quality":{"quality":{"id":15,"name":"WEBRip-1080p"}},
	    "languages":[{"id":1,"name":"English"}],"releaseGroup":"ABM"}}`})

	got, err := RadarrGetMovie(context.Background(), radarrClient(srv.URL), 1)
	if err != nil {
		t.Fatalf("RadarrGetMovie: %v", err)
	}
	if got.Title != "Whiplash" || got.MinimumAvailability != "released" {
		t.Errorf("movie identity lost: %+v", got)
	}
	if got.File == nil {
		t.Fatal("movie file summary missing")
	}
	if got.File.Quality != "WEBRip-1080p" || got.File.ReleaseGroup != "ABM" {
		t.Errorf("file summary = %+v", got.File)
	}
	if got.CollectionTitle != "A Collection" {
		t.Errorf("collectionTitle = %q, want A Collection", got.CollectionTitle)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshalling detail: %v", err)
	}
	if strings.Contains(string(raw), "must not survive") {
		t.Errorf("the overview reached the projection: %s", raw)
	}
}

// --- add-series and add-movie options ---

func TestAddSeriesCarriesMonitorSeriesTypeAndTags(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/series": `{"id":1,"title":"X"}`})

	seasonFolder := false
	req := AddSeriesRequest{
		TVDBID: 1, Title: "X", QualityProfileID: 2, RootFolderPath: "/tv",
		Monitored: true, SeasonFolder: &seasonFolder,
		SeriesType: "anime", Tags: []int{4, 5},
	}
	req.AddOptions.SearchForMissingEpisodes = true
	req.AddOptions.Monitor = "firstSeason"

	if _, err := SonarrAddSeries(context.Background(), sonarrClient(srv.URL), req); err != nil {
		t.Fatalf("SonarrAddSeries: %v", err)
	}

	got := find(t, *seen, http.MethodPost, "/api/v3/series")
	var body struct {
		SeriesType   string `json:"seriesType"`
		SeasonFolder *bool  `json:"seasonFolder"`
		Tags         []int  `json:"tags"`
		AddOptions   struct {
			Monitor                  string `json:"monitor"`
			SearchForMissingEpisodes bool   `json:"searchForMissingEpisodes"`
		} `json:"addOptions"`
	}
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatalf("decoding add body %q: %v", got.body, err)
	}
	if body.AddOptions.Monitor != "firstSeason" {
		t.Errorf("addOptions.monitor = %q, want firstSeason", body.AddOptions.Monitor)
	}
	if body.SeriesType != "anime" {
		t.Errorf("seriesType = %q, want anime", body.SeriesType)
	}
	if body.SeasonFolder == nil || *body.SeasonFolder {
		t.Errorf("seasonFolder = %v, want false", body.SeasonFolder)
	}
	if len(body.Tags) != 2 || body.Tags[0] != 4 {
		t.Errorf("tags = %v, want [4 5]", body.Tags)
	}
}

func TestAddMovieCarriesTagsAndMinimumAvailability(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/movie": `{"id":1,"title":"X"}`})

	req := AddMovieRequest{
		TMDBID: 1, Title: "X", QualityProfileID: 2, RootFolderPath: "/movies",
		Monitored: false, MinimumAvailability: "announced", Tags: []int{3},
	}
	if _, err := RadarrAddMovie(context.Background(), radarrClient(srv.URL), req); err != nil {
		t.Fatalf("RadarrAddMovie: %v", err)
	}

	got := find(t, *seen, http.MethodPost, "/api/v3/movie")
	var body struct {
		Monitored           bool   `json:"monitored"`
		MinimumAvailability string `json:"minimumAvailability"`
		Tags                []int  `json:"tags"`
	}
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatalf("decoding add body %q: %v", got.body, err)
	}
	if body.Monitored {
		t.Error("monitored = true, want false")
	}
	if body.MinimumAvailability != "announced" {
		t.Errorf("minimumAvailability = %q, want announced", body.MinimumAvailability)
	}
	if len(body.Tags) != 1 || body.Tags[0] != 3 {
		t.Errorf("tags = %v, want [3]", body.Tags)
	}
}

// ptr returns a pointer to i, for the optional integer arguments.
func ptr(i int) *int { return &i }

// ptrBool returns a pointer to b.
func ptrBool(b bool) *bool { return &b }

// ptrString returns a pointer to s.
func ptrString(s string) *string { return &s }

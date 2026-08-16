package arr

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSonarrWantedMissingUnwrapsAndReportsTheTotal(t *testing.T) {
	srv, got := fakeService(t, 200, `{
	  "page":1,"pageSize":2,"totalRecords":417,
	  "records":[{"id":1,"seriesId":9,"title":"Pilot","seasonNumber":1,"episodeNumber":1,
	    "airDateUtc":"2026-01-01T00:00:00Z","hasFile":false,"monitored":true,
	    "overview":"a very long paragraph that has no business being in a listing"}]
	}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	eps, total, err := SonarrWantedMissing(context.Background(), c, 2)
	if err != nil {
		t.Fatalf("SonarrWantedMissing returned error: %v", err)
	}
	if got.path != "/api/v3/wanted/missing" {
		t.Errorf("path = %q, want /api/v3/wanted/missing", got.path)
	}
	if !contains(got.query, "pageSize=2") {
		t.Errorf("query = %q, want pageSize=2", got.query)
	}
	// The page holds one record but the library is missing 417 episodes; a
	// count of 1 would be a badly misleading answer.
	if total != 417 {
		t.Errorf("total = %d, want 417", total)
	}
	if len(eps) != 1 || eps[0].Title != "Pilot" {
		t.Fatalf("episodes = %+v, want the Pilot", eps)
	}
	if encoded, _ := json.Marshal(eps); contains(string(encoded), "no business") {
		t.Errorf("episode overview leaked into the listing: %s", encoded)
	}
}

func TestSonarrWantedCutoffUsesTheCutoffEndpoint(t *testing.T) {
	srv, got := fakeService(t, 200, `{"totalRecords":4145,"records":[{"id":2,"title":"Ep"}]}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	_, total, err := SonarrWantedCutoff(context.Background(), c, 0)
	if err != nil {
		t.Fatalf("SonarrWantedCutoff returned error: %v", err)
	}
	if got.path != "/api/v3/wanted/cutoff" {
		t.Errorf("path = %q, want /api/v3/wanted/cutoff", got.path)
	}
	if total != 4145 {
		t.Errorf("total = %d, want 4145", total)
	}
}

// Radarr's wanted endpoints return whole movie resources — 3.4 KB per record
// on the live instance, including the overview and alternate titles.
func TestRadarrWantedMissingProjectsFullMovieResources(t *testing.T) {
	srv, got := fakeService(t, 200, `{
	  "totalRecords":2,
	  "records":[{"id":278,"title":"Spider-Man","year":2026,"monitored":true,"hasFile":false,
	    "tmdbId":1,"overview":"a paragraph","alternateTitles":[{"title":"Spidey"}],
	    "images":[{"remoteUrl":"https://image.tmdb.org/poster.jpg"}]}]
	}`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	movies, total, err := RadarrWantedMissing(context.Background(), c, 20)
	if err != nil {
		t.Fatalf("RadarrWantedMissing returned error: %v", err)
	}
	if got.path != "/api/v3/wanted/missing" {
		t.Errorf("path = %q, want /api/v3/wanted/missing", got.path)
	}
	if total != 2 || len(movies) != 1 || movies[0].Title != "Spider-Man" {
		t.Fatalf("movies = %+v total = %d, want one Spider-Man of 2", movies, total)
	}
	encoded, _ := json.Marshal(movies)
	for _, unwanted := range []string{"a paragraph", "Spidey", "image.tmdb.org"} {
		if contains(string(encoded), unwanted) {
			t.Errorf("movie listing leaked %q: %s", unwanted, encoded)
		}
	}
}

func TestRadarrWantedCutoffUsesTheCutoffEndpoint(t *testing.T) {
	srv, got := fakeService(t, 200, `{"totalRecords":115,"records":[]}`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	if _, _, err := RadarrWantedCutoff(context.Background(), c, 10); err != nil {
		t.Fatalf("RadarrWantedCutoff returned error: %v", err)
	}
	if got.path != "/api/v3/wanted/cutoff" {
		t.Errorf("path = %q, want /api/v3/wanted/cutoff", got.path)
	}
}

func TestListBlocklistTrimsToTheRejectedRelease(t *testing.T) {
	srv, got := fakeService(t, 200, `{
	  "totalRecords":1206,
	  "records":[{"id":44,"seriesId":60,"episodeIds":[5171],
	    "sourceTitle":"Friends.S02E21.2160p","date":"2026-01-01T00:00:00Z",
	    "protocol":"usenet","indexer":"NZBgeek","message":"failed",
	    "quality":{"quality":{"name":"WEBDL-2160p"}},
	    "customFormats":[{"id":53,"name":"x265"}],
	    "series":{"title":"Friends","overview":"six friends"}}]
	}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	items, total, err := ListBlocklist(context.Background(), c, 20)
	if err != nil {
		t.Fatalf("ListBlocklist returned error: %v", err)
	}
	if got.path != "/api/v3/blocklist" {
		t.Errorf("path = %q, want /api/v3/blocklist", got.path)
	}
	if total != 1206 || len(items) != 1 {
		t.Fatalf("items = %d total = %d, want 1 of 1206", len(items), total)
	}
	if items[0].SourceTitle != "Friends.S02E21.2160p" || items[0].Indexer != "NZBgeek" {
		t.Errorf("item = %+v, want the release title and indexer", items[0])
	}
	if items[0].Quality != "WEBDL-2160p" {
		t.Errorf("quality = %q, want the flattened quality name", items[0].Quality)
	}
	// The whole series record rides along on every blocklist row.
	if encoded, _ := json.Marshal(items); contains(string(encoded), "six friends") {
		t.Errorf("the embedded series resource leaked: %s", encoded)
	}
}

func TestDeleteBlocklistItemTargetsTheEntry(t *testing.T) {
	srv, got := fakeService(t, 200, ``)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	if err := DeleteBlocklistItem(context.Background(), c, 44); err != nil {
		t.Fatalf("DeleteBlocklistItem returned error: %v", err)
	}
	if got.method != "DELETE" || got.path != "/api/v3/blocklist/44" {
		t.Errorf("request = %s %s, want DELETE /api/v3/blocklist/44", got.method, got.path)
	}
}

func TestListTasksReportsSchedules(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"id":3,"name":"Backup","taskName":"Backup","interval":10080,
	  "lastExecution":"2026-08-16T03:14:18Z","nextExecution":"2026-08-23T03:14:18Z"}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	tasks, err := ListTasks(context.Background(), c)
	if err != nil {
		t.Fatalf("ListTasks returned error: %v", err)
	}
	if got.path != "/api/v3/system/task" {
		t.Errorf("path = %q, want /api/v3/system/task", got.path)
	}
	if len(tasks) != 1 || tasks[0].TaskName != "Backup" || tasks[0].Interval != 10080 {
		t.Errorf("tasks = %+v, want the Backup task every 10080 minutes", tasks)
	}
}

func TestListUpdatesReportsInstalledAndAvailable(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"version":"4.0.19.2979","branch":"main",
	  "releaseDate":"2026-06-26T17:24:52Z","installed":true,"installable":false,"latest":true,
	  "changes":{"new":["a very long changelog"],"fixed":["another one"]}}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	updates, err := ListUpdates(context.Background(), c)
	if err != nil {
		t.Fatalf("ListUpdates returned error: %v", err)
	}
	if got.path != "/api/v3/update" {
		t.Errorf("path = %q, want /api/v3/update", got.path)
	}
	if len(updates) != 1 || !updates[0].Installed || !updates[0].Latest {
		t.Errorf("updates = %+v, want the installed latest release", updates)
	}
	if encoded, _ := json.Marshal(updates); contains(string(encoded), "changelog") {
		t.Errorf("changelogs leaked into the result: %s", encoded)
	}
}

func TestGetQueueStatusSummarisesTheQueue(t *testing.T) {
	srv, got := fakeService(t, 200, `{"totalCount":8,"count":8,"unknownCount":0,"errors":false,"warnings":true}`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	status, err := GetQueueStatus(context.Background(), c)
	if err != nil {
		t.Fatalf("GetQueueStatus returned error: %v", err)
	}
	if got.path != "/api/v3/queue/status" {
		t.Errorf("path = %q, want /api/v3/queue/status", got.path)
	}
	if status.Count != 8 || !status.Warnings {
		t.Errorf("status = %+v, want 8 items with warnings", status)
	}
}

// The three Sonarr search commands take different parameters; picking the wrong
// one silently searches the wrong scope.
func TestSonarrTriggerSearchPicksTheCommandForTheScope(t *testing.T) {
	cases := []struct {
		name     string
		season   *int
		episodes []int
		wantCmd  string
		wantArg  string
	}{
		{"whole series", nil, nil, "SeriesSearch", `"seriesId":5`},
		{"one season", intPtr(3), nil, "SeasonSearch", `"seasonNumber":3`},
		{"specific episodes", nil, []int{11, 12}, "EpisodeSearch", `"episodeIds":[11,12]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, got := fakeService(t, 201, `{"id":1,"name":"`+tc.wantCmd+`","status":"queued"}`)
			c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

			res, err := SonarrTriggerSearch(context.Background(), c, 5, tc.season, tc.episodes)
			if err != nil {
				t.Fatalf("SonarrTriggerSearch returned error: %v", err)
			}
			if got.method != "POST" || got.path != "/api/v3/command" {
				t.Errorf("request = %s %s, want POST /api/v3/command", got.method, got.path)
			}
			if !contains(got.body, `"name":"`+tc.wantCmd+`"`) {
				t.Errorf("body = %q, want command %s", got.body, tc.wantCmd)
			}
			if !contains(got.body, tc.wantArg) {
				t.Errorf("body = %q, want %s", got.body, tc.wantArg)
			}
			if res.Name != tc.wantCmd {
				t.Errorf("result = %+v, want %s", res, tc.wantCmd)
			}
		})
	}
}

// An episode search ignores the series id entirely, so sending both would be
// ambiguous about which one the service honours.
func TestSonarrTriggerSearchOmitsTheSeriesForAnEpisodeSearch(t *testing.T) {
	srv, got := fakeService(t, 201, `{"id":1,"name":"EpisodeSearch"}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	if _, err := SonarrTriggerSearch(context.Background(), c, 5, nil, []int{11}); err != nil {
		t.Fatalf("SonarrTriggerSearch returned error: %v", err)
	}
	if contains(got.body, "seriesId") {
		t.Errorf("body = %q, must not carry a series id for an episode search", got.body)
	}
}

func TestRadarrTriggerSearchPostsMovieIDs(t *testing.T) {
	srv, got := fakeService(t, 201, `{"id":2,"name":"MoviesSearch","status":"queued"}`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	res, err := RadarrTriggerSearch(context.Background(), c, []int{9, 10})
	if err != nil {
		t.Fatalf("RadarrTriggerSearch returned error: %v", err)
	}
	if got.path != "/api/v3/command" {
		t.Errorf("path = %q, want /api/v3/command", got.path)
	}
	if !contains(got.body, `"name":"MoviesSearch"`) || !contains(got.body, `"movieIds":[9,10]`) {
		t.Errorf("body = %q, want MoviesSearch with both ids", got.body)
	}
	if res.Name != "MoviesSearch" {
		t.Errorf("result = %+v, want MoviesSearch", res)
	}
}

func TestRadarrTriggerSearchRejectsAnEmptyIDList(t *testing.T) {
	srv, got := fakeService(t, 201, `{}`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	if _, err := RadarrTriggerSearch(context.Background(), c, nil); err == nil {
		t.Fatal("expected an error when no movie ids are given")
	}
	if len(got.paths) != 0 {
		t.Errorf("upstream was contacted %v for an empty id list", got.paths)
	}
}

func TestSonarrRefreshSeriesPostsTheCommand(t *testing.T) {
	srv, got := fakeService(t, 201, `{"id":3,"name":"RefreshSeries"}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	if _, err := SonarrRefreshSeries(context.Background(), c, 5); err != nil {
		t.Fatalf("SonarrRefreshSeries returned error: %v", err)
	}
	if !contains(got.body, `"name":"RefreshSeries"`) || !contains(got.body, `"seriesId":5`) {
		t.Errorf("body = %q, want RefreshSeries for series 5", got.body)
	}
}

func TestRadarrRefreshMoviesPostsTheCommand(t *testing.T) {
	srv, got := fakeService(t, 201, `{"id":4,"name":"RefreshMovie"}`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	if _, err := RadarrRefreshMovies(context.Background(), c, []int{9}); err != nil {
		t.Fatalf("RadarrRefreshMovies returned error: %v", err)
	}
	if !contains(got.body, `"name":"RefreshMovie"`) || !contains(got.body, `"movieIds":[9]`) {
		t.Errorf("body = %q, want RefreshMovie for movie 9", got.body)
	}
}

// A collection carries every movie it contains plus artwork; the live Radarr
// returns 259 KB for the list.
func TestRadarrListCollectionsTrimsToCounts(t *testing.T) {
	srv, got := fakeService(t, 200, `[{
	  "id":4,"title":"Krrish Collection","tmdbId":246091,"monitored":false,
	  "qualityProfileId":7,"rootFolderPath":"/movies","searchOnAdd":true,
	  "minimumAvailability":"released","missingMovies":2,"tags":[1],
	  "overview":"Krrish is an Indian franchise",
	  "images":[{"remoteUrl":"https://image.tmdb.org/poster.jpg"}],
	  "movies":[{"title":"Koi Mil Gaya","overview":"long"},{"title":"Krrish","overview":"long"}]
	}]`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	collections, err := RadarrListCollections(context.Background(), c)
	if err != nil {
		t.Fatalf("RadarrListCollections returned error: %v", err)
	}
	if got.path != "/api/v3/collection" {
		t.Errorf("path = %q, want /api/v3/collection", got.path)
	}
	if len(collections) != 1 {
		t.Fatalf("collections = %d, want 1", len(collections))
	}
	col := collections[0]
	if col.Title != "Krrish Collection" || col.MovieCount != 2 || col.MissingMovies != 2 {
		t.Errorf("collection = %+v, want 2 movies, 2 missing", col)
	}
	encoded, _ := json.Marshal(collections)
	for _, unwanted := range []string{"Indian franchise", "image.tmdb.org", "Koi Mil Gaya"} {
		if contains(string(encoded), unwanted) {
			t.Errorf("collection listing leaked %q: %s", unwanted, encoded)
		}
	}
}

// intPtr is a test helper for the optional season argument.
func intPtr(i int) *int { return &i }

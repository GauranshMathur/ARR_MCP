package arr

import (
	"context"
	"testing"
)

func TestListHealthIssuesTrimsToActionableFields(t *testing.T) {
	srv, got := fakeService(t, 200, `[
	  {"source":"IndexerStatusCheck","type":"warning","message":"Indexers unavailable","wikiUrl":"https://wiki/x","extra":"ignored"}
	]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	issues, err := ListHealthIssues(context.Background(), c)
	if err != nil {
		t.Fatalf("ListHealthIssues returned error: %v", err)
	}
	if got.path != "/api/v3/health" {
		t.Errorf("path = %q, want /api/v3/health", got.path)
	}
	if len(issues) != 1 {
		t.Fatalf("issues = %d, want 1", len(issues))
	}
	if issues[0].Source != "IndexerStatusCheck" || issues[0].Type != "warning" {
		t.Errorf("issue = %+v, want source/type populated", issues[0])
	}
}

func TestListDiskSpaceReportsFreeAndTotal(t *testing.T) {
	srv, got := fakeService(t, 200, `[
	  {"path":"/tv","label":"media","freeSpace":1234,"totalSpace":9999}
	]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	disks, err := ListDiskSpace(context.Background(), c)
	if err != nil {
		t.Fatalf("ListDiskSpace returned error: %v", err)
	}
	if got.path != "/api/v3/diskspace" {
		t.Errorf("path = %q, want /api/v3/diskspace", got.path)
	}
	if len(disks) != 1 || disks[0].FreeSpace != 1234 || disks[0].TotalSpace != 9999 {
		t.Errorf("disks = %+v, want free=1234 total=9999", disks)
	}
}

// The queue endpoint wraps its results in a paging envelope, unlike /series.
func TestListQueueUnwrapsPagingEnvelope(t *testing.T) {
	srv, got := fakeService(t, 200, `{
	  "page":1,"pageSize":20,"totalRecords":2,
	  "records":[
	    {"id":7,"title":"Some.Release","status":"downloading","timeleft":"00:10:00","size":100,"sizeleft":40,"protocol":"usenet"},
	    {"id":8,"title":"Other.Release","status":"queued","protocol":"torrent"}
	  ]
	}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	items, err := ListQueue(context.Background(), c, 20)
	if err != nil {
		t.Fatalf("ListQueue returned error: %v", err)
	}
	if got.path != "/api/v3/queue" {
		t.Errorf("path = %q, want /api/v3/queue", got.path)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].ID != 7 || items[0].Status != "downloading" {
		t.Errorf("first item = %+v, want id 7 downloading", items[0])
	}
}

func TestListQueuePassesPageSize(t *testing.T) {
	srv, got := fakeService(t, 200, `{"records":[]}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	if _, err := ListQueue(context.Background(), c, 50); err != nil {
		t.Fatalf("ListQueue returned error: %v", err)
	}
	if got.query == "" {
		t.Fatal("no query parameters sent")
	}
	if want := "pageSize=50"; !contains(got.query, want) {
		t.Errorf("query = %q, want it to contain %q", got.query, want)
	}
}

func TestListHistoryUnwrapsPagingEnvelope(t *testing.T) {
	srv, got := fakeService(t, 200, `{
	  "records":[{"id":3,"eventType":"grabbed","date":"2026-01-01T00:00:00Z","sourceTitle":"Show.S01E01"}]
	}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	records, err := ListHistory(context.Background(), c, 10)
	if err != nil {
		t.Fatalf("ListHistory returned error: %v", err)
	}
	if got.path != "/api/v3/history" {
		t.Errorf("path = %q, want /api/v3/history", got.path)
	}
	if len(records) != 1 || records[0].EventType != "grabbed" {
		t.Errorf("records = %+v, want one grabbed event", records)
	}
}

func TestDeleteQueueItemTargetsTheItem(t *testing.T) {
	srv, got := fakeService(t, 200, ``)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	if err := DeleteQueueItem(context.Background(), c, 42, true, false); err != nil {
		t.Fatalf("DeleteQueueItem returned error: %v", err)
	}
	if got.path != "/api/v3/queue/42" {
		t.Errorf("path = %q, want /api/v3/queue/42", got.path)
	}
	if !contains(got.query, "removeFromClient=true") {
		t.Errorf("query = %q, want removeFromClient=true", got.query)
	}
	if !contains(got.query, "blocklist=false") {
		t.Errorf("query = %q, want blocklist=false", got.query)
	}
}

func TestRunCommandPostsCommandName(t *testing.T) {
	srv, got := fakeService(t, 201, `{"id":9,"name":"RefreshSeries","status":"queued"}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	res, err := RunCommand(context.Background(), c, "RefreshSeries", nil)
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}
	if got.path != "/api/v3/command" {
		t.Errorf("path = %q, want /api/v3/command", got.path)
	}
	if res.Name != "RefreshSeries" || res.Status != "queued" {
		t.Errorf("result = %+v, want RefreshSeries/queued", res)
	}
}

func TestSonarrCalendarPassesDateRange(t *testing.T) {
	srv, got := fakeService(t, 200, `[
	  {"id":1,"seriesId":2,"title":"Pilot","seasonNumber":1,"episodeNumber":1,"airDateUtc":"2026-01-01T00:00:00Z","hasFile":false}
	]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	eps, err := SonarrCalendar(context.Background(), c, "2026-01-01", "2026-01-08")
	if err != nil {
		t.Fatalf("SonarrCalendar returned error: %v", err)
	}
	if got.path != "/api/v3/calendar" {
		t.Errorf("path = %q, want /api/v3/calendar", got.path)
	}
	for _, want := range []string{"start=2026-01-01", "end=2026-01-08"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
	if len(eps) != 1 || eps[0].Title != "Pilot" {
		t.Errorf("episodes = %+v, want one titled Pilot", eps)
	}
}

func TestSonarrListEpisodesFiltersBySeries(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"id":5,"seriesId":3,"title":"Ep","seasonNumber":2,"episodeNumber":4,"hasFile":true}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	eps, err := SonarrListEpisodes(context.Background(), c, 3)
	if err != nil {
		t.Fatalf("SonarrListEpisodes returned error: %v", err)
	}
	if got.path != "/api/v3/episode" {
		t.Errorf("path = %q, want /api/v3/episode", got.path)
	}
	if !contains(got.query, "seriesId=3") {
		t.Errorf("query = %q, want seriesId=3", got.query)
	}
	if len(eps) != 1 || eps[0].SeasonNumber != 2 {
		t.Errorf("episodes = %+v, want season 2", eps)
	}
}

func TestRadarrCalendarReturnsMovies(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"id":1,"title":"Dune","year":2021,"hasFile":false}]`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	movies, err := RadarrCalendar(context.Background(), c, "2026-01-01", "2026-02-01")
	if err != nil {
		t.Fatalf("RadarrCalendar returned error: %v", err)
	}
	if got.path != "/api/v3/calendar" {
		t.Errorf("path = %q, want /api/v3/calendar", got.path)
	}
	if len(movies) != 1 || movies[0].Title != "Dune" {
		t.Errorf("movies = %+v, want Dune", movies)
	}
}

func TestProwlarrIndexerStatsUsesV1Path(t *testing.T) {
	srv, got := fakeService(t, 200, `{"indexers":[{"indexerId":1,"indexerName":"nzb","numberOfQueries":10,"numberOfGrabs":2}]}`)
	c := NewClient(srv.URL, ProwlarrSpec, Credentials{APIKey: "k"})

	stats, err := ProwlarrIndexerStats(context.Background(), c)
	if err != nil {
		t.Fatalf("ProwlarrIndexerStats returned error: %v", err)
	}
	if got.path != "/api/v1/indexerstats" {
		t.Errorf("path = %q, want /api/v1/indexerstats", got.path)
	}
	if len(stats) != 1 || stats[0].IndexerName != "nzb" {
		t.Errorf("stats = %+v, want one nzb entry", stats)
	}
}

// contains is a tiny helper so assertions read clearly.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

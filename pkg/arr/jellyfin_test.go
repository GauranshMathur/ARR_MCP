package arr

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const jellyfinKey = "jf-test-key-1234"

func jellyfinClient(url string) *Client {
	return NewClient(url, JellyfinSpec, Credentials{APIKey: jellyfinKey})
}

// wantJellyfinAuth fails unless the request carried the MediaBrowser token.
func wantJellyfinAuth(t *testing.T, got *capture) {
	t.Helper()
	if v := got.header.Get("Authorization"); v != `MediaBrowser Token="`+jellyfinKey+`"` {
		t.Errorf("Authorization = %q, want the MediaBrowser token", v)
	}
}

// jellyfinRoutes serves a fixed body per "METHOD /path" and records each
// request line with its raw query, for calls that make more than one request.
func jellyfinRoutes(t *testing.T, routes map[string]string) (*Client, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.RequestURI())
		_, _ = io.Copy(io.Discard, r.Body)
		body, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return jellyfinClient(srv.URL), &seen
}

func TestJellyfinPingHitsAuthenticatedSystemInfo(t *testing.T) {
	srv, got := fakeService(t, 200, `{}`)
	if err := jellyfinClient(srv.URL).Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if got.path != "/System/Info" {
		t.Errorf("status path = %q, want /System/Info (not the unauthenticated /System/Info/Public)", got.path)
	}
	wantJellyfinAuth(t, got)
}

func TestJellyfinSystemInfoIsTrimmed(t *testing.T) {
	srv, got := fakeService(t, 200, `{"ServerName":"Arkhaya","Version":"12.0.0","ProductName":"Jellyfin Server",
	  "OperatingSystem":"","SystemArchitecture":"X64","Id":"faaa","HasPendingRestart":true,
	  "HasUpdateAvailable":false,"IsShuttingDown":false,"StartupWizardCompleted":true,
	  "LocalAddress":"http://10.244.1.20:8096","ProgramDataPath":"/var/lib/jellyfin","LogPath":"/var/log/jellyfin",
	  "CompletedInstallations":[{"Name":"Custom Tabs"}]}`)

	info, err := JellyfinSystemInfo(context.Background(), jellyfinClient(srv.URL))
	if err != nil {
		t.Fatalf("JellyfinSystemInfo: %v", err)
	}
	if got.method != "GET" || got.path != "/System/Info" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellyfinAuth(t, got)
	want := JellyfinInfo{ServerName: "Arkhaya", Version: "12.0.0", ProductName: "Jellyfin Server",
		SystemArchitecture: "X64", ServerID: "faaa", HasPendingRestart: true, LocalAddress: "http://10.244.1.20:8096"}
	if info != want {
		t.Errorf("info = %+v, want %+v", info, want)
	}
}

func TestJellyfinListLibrariesDropsLibraryOptions(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"Name":"Movies","Locations":["/NAS/Movies","/NAS/Anime/Movies"],
	  "CollectionType":"movies","ItemId":"f137a2dd21bbc1b99aa5c0f6bf02a805","RefreshStatus":"Active","RefreshProgress":42.5,
	  "LibraryOptions":{"PathInfos":[{"Path":"/NAS/Movies"}],"MetadataSavers":[]}}]`)

	libs, err := JellyfinListLibraries(context.Background(), jellyfinClient(srv.URL))
	if err != nil {
		t.Fatalf("JellyfinListLibraries: %v", err)
	}
	if got.method != "GET" || got.path != "/Library/VirtualFolders" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellyfinAuth(t, got)
	if len(libs) != 1 {
		t.Fatalf("libraries = %+v", libs)
	}
	l := libs[0]
	if l.Name != "Movies" || l.ItemID != "f137a2dd21bbc1b99aa5c0f6bf02a805" || l.CollectionType != "movies" ||
		len(l.Locations) != 2 || l.RefreshStatus != "Active" || l.RefreshProgress == nil || *l.RefreshProgress != 42.5 {
		t.Errorf("library = %+v", l)
	}
}

func TestJellyfinSearchItemsSendsFiltersAndTrims(t *testing.T) {
	srv, got := fakeService(t, 200, `{"TotalRecordCount":183,"StartIndex":20,"Items":[
	  {"Name":"17 Again","Id":"748950f8695760523d0361ea48e0953b","Type":"Movie","ProductionYear":2009,"RunTimeTicks":61154880000,
	   "Path":"/NAS/Movies/17 Again.mkv","LocationType":"FileSystem","ImageBlurHashes":{"Primary":{"a":"b"}},
	   "BackdropImageTags":["x"]},
	  {"Name":"A Farewell Special","Id":"f1d0","Type":"Episode","SeriesName":"The Neighborhood",
	   "ParentIndexNumber":0,"IndexNumber":1,"LocationType":"Virtual"}]}`)

	page, err := JellyfinSearchItems(context.Background(), jellyfinClient(srv.URL), JellyfinItemQuery{
		SearchTerm: "17 again", IncludeItemTypes: "Movie,Episode", ParentID: "f137a2dd21bbc1b99aa5c0f6bf02a805",
		Recursive: true, Limit: 2, StartIndex: 20,
	})
	if err != nil {
		t.Fatalf("JellyfinSearchItems: %v", err)
	}
	if got.method != "GET" || got.path != "/Items" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellyfinAuth(t, got)
	for _, part := range []string{"searchTerm=17+again", "includeItemTypes=Movie%2CEpisode", "parentId=f137a2dd21bbc1b99aa5c0f6bf02a805",
		"recursive=true", "limit=2", "startIndex=20", "fields=Path"} {
		if !strings.Contains(got.query, part) {
			t.Errorf("query %q missing %q", got.query, part)
		}
	}
	if page.TotalCount != 183 || page.StartIndex != 20 || len(page.Items) != 2 {
		t.Fatalf("page = %+v", page)
	}
	m := page.Items[0]
	if m.ID != "748950f8695760523d0361ea48e0953b" || m.Name != "17 Again" || m.Type != "Movie" || m.Year != 2009 ||
		m.Path != "/NAS/Movies/17 Again.mkv" || m.RuntimeMinutes != 102 {
		t.Errorf("movie = %+v", m)
	}
	e := page.Items[1]
	if e.SeriesName != "The Neighborhood" || e.SeasonNumber == nil || *e.SeasonNumber != 0 ||
		e.EpisodeNumber == nil || *e.EpisodeNumber != 1 || e.LocationType != "Virtual" {
		t.Errorf("episode = %+v", e)
	}
}

func TestJellyfinSearchItemsOmitsUnsetFilters(t *testing.T) {
	srv, got := fakeService(t, 200, `{"TotalRecordCount":0,"StartIndex":0,"Items":[]}`)
	if _, err := JellyfinSearchItems(context.Background(), jellyfinClient(srv.URL), JellyfinItemQuery{}); err != nil {
		t.Fatalf("JellyfinSearchItems: %v", err)
	}
	for _, key := range []string{"searchTerm", "includeItemTypes", "parentId", "startIndex"} {
		if strings.Contains(got.query, key+"=") {
			t.Errorf("query %q should not carry %s", got.query, key)
		}
	}
	if !strings.Contains(got.query, "recursive=false") {
		t.Errorf("query %q should say recursive=false", got.query)
	}
}

func TestJellyfinSearchItemsRejectsABadParentID(t *testing.T) {
	srv, got := fakeService(t, 200, `{}`)
	_, err := JellyfinSearchItems(context.Background(), jellyfinClient(srv.URL), JellyfinItemQuery{ParentID: "../x"})
	if err == nil || len(got.paths) != 0 {
		t.Errorf("err = %v, upstream calls = %v; want a rejection before any request", err, got.paths)
	}
}

const jellyfinItemBody = `{"Name":"17 Again","Id":"748950f8695760523d0361ea48e0953b","Type":"Movie","ProductionYear":2009,
  "RunTimeTicks":61154880000,"Path":"/NAS/Movies/17 Again.mkv","LocationType":"FileSystem",
  "Overview":"A do-over.","Genres":["Comedy","Fantasy"],"CommunityRating":6.318,"OfficialRating":"PG-13",
  "ProviderIds":{"Imdb":"tt0974661","Tmdb":"16996"},"DateCreated":"2024-10-17T00:39:30.0000000Z",
  "Container":"mkv","Width":1920,"Height":1080,
  "UserData":{"PlaybackPositionTicks":6000000000,"PlayCount":1,"IsFavorite":true,"Played":false,
              "LastPlayedDate":"2025-11-12T14:55:53.5723829Z","Key":"16996"},
  "MediaSources":[{"Path":"x"}],"People":[{"Name":"p"}],"Studios":[{"Name":"s"}]}`

func TestJellyfinGetItemWithUserID(t *testing.T) {
	srv, got := fakeService(t, 200, jellyfinItemBody)

	item, err := JellyfinGetItem(context.Background(), jellyfinClient(srv.URL), "748950f8695760523d0361ea48e0953b", "0259b8b9ba6540c7b927ec98efa5c2da")
	if err != nil {
		t.Fatalf("JellyfinGetItem: %v", err)
	}
	if got.method != "GET" || got.path != "/Items/748950f8695760523d0361ea48e0953b" || got.query != "userId=0259b8b9ba6540c7b927ec98efa5c2da" {
		t.Errorf("request = %s %s ?%s", got.method, got.path, got.query)
	}
	wantJellyfinAuth(t, got)
	if item.Name != "17 Again" || item.RuntimeMinutes != 102 || item.Overview != "A do-over." ||
		len(item.Genres) != 2 || item.CommunityRating != 6.318 || item.OfficialRating != "PG-13" ||
		item.ProviderIDs["Tmdb"] != "16996" || item.Container != "mkv" || item.Width != 1920 {
		t.Errorf("item = %+v", item)
	}
	if item.UserID != "0259b8b9ba6540c7b927ec98efa5c2da" || item.UserData == nil || item.UserData.PlaybackPositionSeconds != 600 ||
		item.UserData.PlayCount != 1 || !item.UserData.IsFavorite || item.UserData.Played {
		t.Errorf("userData = %+v (userId %q)", item.UserData, item.UserID)
	}
}

// Jellyfin 12.0 answers GET /Items/{id} with 400 unless a userId is given, so
// the first enabled user stands in when the caller names none.
func TestJellyfinGetItemDefaultsToFirstEnabledUser(t *testing.T) {
	c, seen := jellyfinRoutes(t, map[string]string{
		"GET /Users": `[{"Id":"off","Name":"a","Policy":{"IsDisabled":true}},
		                {"Id":"on","Name":"b","Policy":{"IsDisabled":false}}]`,
		"GET /Items/748950f8695760523d0361ea48e0953b": jellyfinItemBody,
	})

	item, err := JellyfinGetItem(context.Background(), c, "748950f8695760523d0361ea48e0953b", "")
	if err != nil {
		t.Fatalf("JellyfinGetItem: %v", err)
	}
	if len(*seen) != 2 || (*seen)[0] != "GET /Users" || (*seen)[1] != "GET /Items/748950f8695760523d0361ea48e0953b?userId=on" {
		t.Errorf("requests = %v", *seen)
	}
	if item.UserID != "on" {
		t.Errorf("userId = %q, want on", item.UserID)
	}
}

func TestJellyfinGetItemWithNoUsersExplainsWhy(t *testing.T) {
	c, _ := jellyfinRoutes(t, map[string]string{"GET /Users": `[]`})
	_, err := JellyfinGetItem(context.Background(), c, "748950f8695760523d0361ea48e0953b", "")
	if err == nil || !strings.Contains(err.Error(), "no enabled user") {
		t.Errorf("err = %v, want a no-enabled-user error", err)
	}
}

func TestJellyfinGetItemRejectsABadID(t *testing.T) {
	srv, got := fakeService(t, 200, `{}`)
	_, err := JellyfinGetItem(context.Background(), jellyfinClient(srv.URL), "../Library/Refresh", "0259b8b9ba6540c7b927ec98efa5c2da")
	if err == nil || len(got.paths) != 0 {
		t.Errorf("err = %v, upstream calls = %v; want a rejection before any request", err, got.paths)
	}
}

func TestJellyfinGetItemUnknownIDIsAStatusError(t *testing.T) {
	srv, _ := fakeService(t, 404, `{"title":"Not Found","status":404}`)
	_, err := JellyfinGetItem(context.Background(), jellyfinClient(srv.URL), "ffffffffffffffffffffffffffffffff", "0259b8b9ba6540c7b927ec98efa5c2da")
	var se *StatusError
	if !errors.As(err, &se) || se.Status != 404 {
		t.Errorf("err = %v, want a 404 StatusError", err)
	}
}

func TestJellyfinListSessionsShowsNowPlaying(t *testing.T) {
	srv, got := fakeService(t, 200, `[
	  {"Id":"s1","UserName":"Arkhaya","Client":"Jellyfin Web","DeviceName":"Safari","DeviceId":"VERYLONGDEVICEID",
	   "ApplicationVersion":"12.0.0","LastActivityDate":"2026-10-04T12:38:56Z","RemoteEndPoint":"1.2.3.4",
	   "PlayState":{"PositionTicks":6000000000,"IsPaused":true,"CanSeek":true},
	   "NowPlayingItem":{"Id":"748950f8695760523d0361ea48e0953b","Name":"17 Again","Type":"Movie","RunTimeTicks":61154880000,
	                     "ImageBlurHashes":{"a":{"b":"c"}}},
	   "NowPlayingQueue":[{"Id":"748950f8695760523d0361ea48e0953b"}],"Capabilities":{"x":1}},
	  {"Id":"s2","UserName":"shasha","Client":"Infuse","DeviceName":"iPad",
	   "PlayState":{"CanSeek":false,"IsPaused":false}}]`)

	sessions, err := JellyfinListSessions(context.Background(), jellyfinClient(srv.URL))
	if err != nil {
		t.Fatalf("JellyfinListSessions: %v", err)
	}
	if got.method != "GET" || got.path != "/Sessions" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellyfinAuth(t, got)
	if len(sessions) != 2 {
		t.Fatalf("sessions = %+v", sessions)
	}
	a := sessions[0]
	if a.UserName != "Arkhaya" || a.Client != "Jellyfin Web" || a.DeviceName != "Safari" ||
		a.ApplicationVersion != "12.0.0" || a.LastActivityDate != "2026-10-04T12:38:56Z" {
		t.Errorf("session = %+v", a)
	}
	if a.NowPlaying == nil || a.NowPlaying.ID != "748950f8695760523d0361ea48e0953b" || a.NowPlaying.Name != "17 Again" ||
		a.NowPlaying.RuntimeMinutes != 102 {
		t.Errorf("nowPlaying = %+v", a.NowPlaying)
	}
	if a.PlayState == nil || !a.PlayState.IsPaused || a.PlayState.PositionSeconds != 600 {
		t.Errorf("playState = %+v", a.PlayState)
	}
	if b := sessions[1]; b.NowPlaying != nil || b.PlayState != nil {
		t.Errorf("idle session should carry neither nowPlaying nor playState: %+v", b)
	}
}

func TestJellyfinListUsersDropsPolicyAndConfiguration(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"Id":"0259b8b9ba6540c7b927ec98efa5c2da","Name":"Arkhaya","HasPassword":true,
	  "LastLoginDate":"2026-10-03T17:17:18Z","LastActivityDate":"2026-10-04T12:38:56Z",
	  "Configuration":{"AudioLanguagePreference":"eng","PlayDefaultAudioTrack":true},
	  "Policy":{"IsAdministrator":true,"IsDisabled":false,"BlockedTags":["x"],"AuthenticationProviderId":"p"}},
	  {"Id":"u2","Name":"kid","Policy":{"IsAdministrator":false,"IsDisabled":true}}]`)

	users, err := JellyfinListUsers(context.Background(), jellyfinClient(srv.URL))
	if err != nil {
		t.Fatalf("JellyfinListUsers: %v", err)
	}
	if got.method != "GET" || got.path != "/Users" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellyfinAuth(t, got)
	want := []JellyfinUser{
		{ID: "0259b8b9ba6540c7b927ec98efa5c2da", Name: "Arkhaya", IsAdministrator: true, HasPassword: true,
			LastLoginDate: "2026-10-03T17:17:18Z", LastActivityDate: "2026-10-04T12:38:56Z"},
		{ID: "u2", Name: "kid", IsDisabled: true},
	}
	if len(users) != 2 || users[0] != want[0] || users[1] != want[1] {
		t.Errorf("users = %+v, want %+v", users, want)
	}
}

func TestJellyfinListTasksTrimsTriggersAndNamesLastResult(t *testing.T) {
	srv, got := fakeService(t, 200, `[
	  {"Name":"Scan Media Library","State":"Running","CurrentProgressPercentage":12.5,"Id":"7053f526af0895e6d7e11bd42f0ec871",
	   "Key":"RefreshLibrary","Category":"Library","Description":"Scans for new files.",
	   "Triggers":[{"Type":"IntervalTrigger","IntervalTicks":1}],
	   "LastExecutionResult":{"StartTimeUtc":"a","EndTimeUtc":"2026-10-03T14:00:47Z","Status":"Completed","Key":"RefreshLibrary"}},
	  {"Name":"Clean Cache","State":"Idle","Id":"t2","Key":"DeleteCache","Category":"Maintenance"}]`)

	tasks, err := JellyfinListTasks(context.Background(), jellyfinClient(srv.URL))
	if err != nil {
		t.Fatalf("JellyfinListTasks: %v", err)
	}
	if got.method != "GET" || got.path != "/ScheduledTasks" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellyfinAuth(t, got)
	if len(tasks) != 2 {
		t.Fatalf("tasks = %+v", tasks)
	}
	a := tasks[0]
	if a.ID != "7053f526af0895e6d7e11bd42f0ec871" || a.Name != "Scan Media Library" || a.Key != "RefreshLibrary" || a.Category != "Library" ||
		a.State != "Running" || a.ProgressPercent == nil || *a.ProgressPercent != 12.5 ||
		a.LastStatus != "Completed" || a.LastEndTime != "2026-10-03T14:00:47Z" {
		t.Errorf("task = %+v", a)
	}
	if b := tasks[1]; b.ProgressPercent != nil || b.LastStatus != "" {
		t.Errorf("idle never-run task = %+v", b)
	}
}

func TestJellyfinActivityLogPagesAndTrims(t *testing.T) {
	srv, got := fakeService(t, 200, `{"TotalRecordCount":32700,"StartIndex":10,"Items":[
	  {"Id":32700,"Name":"Arkhaya is online from Safari","ShortOverview":"IP address: 1.2.3.4",
	   "Overview":"long","Type":"SessionStarted","Date":"2026-10-04T12:38:56Z","UserId":"0259b8b9ba6540c7b927ec98efa5c2da",
	   "ItemId":"0","Severity":"Information","UserPrimaryImageTag":"t"}]}`)

	page, err := JellyfinActivityLog(context.Background(), jellyfinClient(srv.URL), 5, 10)
	if err != nil {
		t.Fatalf("JellyfinActivityLog: %v", err)
	}
	if got.method != "GET" || got.path != "/System/ActivityLog/Entries" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellyfinAuth(t, got)
	if !strings.Contains(got.query, "limit=5") || !strings.Contains(got.query, "startIndex=10") {
		t.Errorf("query = %q", got.query)
	}
	if page.TotalCount != 32700 || page.StartIndex != 10 || len(page.Entries) != 1 {
		t.Fatalf("page = %+v", page)
	}
	e := page.Entries[0]
	if e.ID != 32700 || e.Name != "Arkhaya is online from Safari" || e.ShortOverview != "IP address: 1.2.3.4" ||
		e.Type != "SessionStarted" || e.Severity != "Information" || e.Date != "2026-10-04T12:38:56Z" || e.UserID != "0259b8b9ba6540c7b927ec98efa5c2da" {
		t.Errorf("entry = %+v", e)
	}
}

func TestJellyfinScanLibraryPostsRefresh(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	if err := JellyfinScanLibrary(context.Background(), jellyfinClient(srv.URL)); err != nil {
		t.Fatalf("JellyfinScanLibrary: %v", err)
	}
	if got.method != "POST" || got.path != "/Library/Refresh" || got.body != "" {
		t.Errorf("request = %s %s body %q", got.method, got.path, got.body)
	}
	wantJellyfinAuth(t, got)
}

func TestJellyfinRefreshItemSendsModeAndNoReplaceFlags(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	if err := JellyfinRefreshItem(context.Background(), jellyfinClient(srv.URL), "748950f8695760523d0361ea48e0953b", "FullRefresh"); err != nil {
		t.Fatalf("JellyfinRefreshItem: %v", err)
	}
	if got.method != "POST" || got.path != "/Items/748950f8695760523d0361ea48e0953b/Refresh" || got.body != "" {
		t.Errorf("request = %s %s body %q", got.method, got.path, got.body)
	}
	wantJellyfinAuth(t, got)
	if !strings.Contains(got.query, "metadataRefreshMode=FullRefresh") ||
		!strings.Contains(got.query, "imageRefreshMode=FullRefresh") {
		t.Errorf("query = %q", got.query)
	}
	if strings.Contains(got.query, "replaceAll") {
		t.Errorf("query %q must never ask to replace existing metadata or images", got.query)
	}
}

// The API's own default is None, which does nothing useful; an empty mode must
// mean the scan the dashboard's "Scan for new and updated files" performs.
func TestJellyfinRefreshItemDefaultsToDefaultMode(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	if err := JellyfinRefreshItem(context.Background(), jellyfinClient(srv.URL), "748950f8695760523d0361ea48e0953b", ""); err != nil {
		t.Fatalf("JellyfinRefreshItem: %v", err)
	}
	if !strings.Contains(got.query, "metadataRefreshMode=Default") {
		t.Errorf("query = %q, want metadataRefreshMode=Default", got.query)
	}
}

func TestJellyfinRefreshItemRejectsBadInput(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := jellyfinClient(srv.URL)
	if err := JellyfinRefreshItem(context.Background(), c, "748950f8695760523d0361ea48e0953b", "None"); err == nil {
		t.Error("mode None should be rejected: it does nothing")
	}
	if err := JellyfinRefreshItem(context.Background(), c, "748950f8695760523d0361ea48e0953b/../x", "Default"); err == nil {
		t.Error("a path-shaped id should be rejected")
	}
	if len(got.paths) != 0 {
		t.Errorf("upstream calls = %v, want none", got.paths)
	}
}

func TestJellyfinRunTaskPostsToRunning(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	if err := JellyfinRunTask(context.Background(), jellyfinClient(srv.URL), "ec2f221fd8e7706b3d3afd2c4591b4d7"); err != nil {
		t.Fatalf("JellyfinRunTask: %v", err)
	}
	if got.method != "POST" || got.path != "/ScheduledTasks/Running/ec2f221fd8e7706b3d3afd2c4591b4d7" || got.body != "" {
		t.Errorf("request = %s %s body %q", got.method, got.path, got.body)
	}
	wantJellyfinAuth(t, got)
	if err := JellyfinRunTask(context.Background(), jellyfinClient(srv.URL), "a/b"); err == nil {
		t.Error("a path-shaped task id should be rejected")
	}
}

func TestJellyfinUpstreamErrorsCarryTheStatus(t *testing.T) {
	srv, _ := fakeService(t, 404, `{"title":"Not Found"}`)
	c := jellyfinClient(srv.URL)
	ctx := context.Background()
	checks := map[string]func() error{
		"system info": func() error { _, err := JellyfinSystemInfo(ctx, c); return err },
		"libraries":   func() error { _, err := JellyfinListLibraries(ctx, c); return err },
		"search":      func() error { _, err := JellyfinSearchItems(ctx, c, JellyfinItemQuery{}); return err },
		"sessions":    func() error { _, err := JellyfinListSessions(ctx, c); return err },
		"users":       func() error { _, err := JellyfinListUsers(ctx, c); return err },
		"tasks":       func() error { _, err := JellyfinListTasks(ctx, c); return err },
		"activity":    func() error { _, err := JellyfinActivityLog(ctx, c, 0, 0); return err },
		"scan":        func() error { return JellyfinScanLibrary(ctx, c) },
		"refresh":     func() error { return JellyfinRefreshItem(ctx, c, "748950f8695760523d0361ea48e0953b", "") },
		"run task":    func() error { return JellyfinRunTask(ctx, c, "7053f526af0895e6d7e11bd42f0ec871") },
	}
	for name, call := range checks {
		var se *StatusError
		if err := call(); !errors.As(err, &se) || se.Status != 404 {
			t.Errorf("%s: err = %v, want a 404 StatusError", name, err)
		}
	}
}

// Every id that is spliced into a path or query must be a Jellyfin id; anything
// else, a traversal above all, must be refused before a request leaves.
func TestJellyfinRejectsEveryNonIDBeforeAnyRequest(t *testing.T) {
	srv, got := fakeService(t, 200, `{}`)
	c := jellyfinClient(srv.URL)
	ctx := context.Background()
	for _, bad := range []string{"../../System/Shutdown", "7489", "748950f8695760523d0361ea48e0953b/Refresh",
		"748950f8695760523d0361ea48e0953g", "748950f8-6957-6052-3d03-61ea48e0953b0", "", "..", "%2e%2e"} {
		calls := map[string]error{
			"get item": func() error { _, err := JellyfinGetItem(ctx, c, bad, "0259b8b9ba6540c7b927ec98efa5c2da"); return err }(),
			"get as user": func() error {
				_, err := JellyfinGetItem(ctx, c, "748950f8695760523d0361ea48e0953b", bad+"x")
				return err
			}(),
			"refresh":  JellyfinRefreshItem(ctx, c, bad, "Default"),
			"run task": JellyfinRunTask(ctx, c, bad),
			"parent": func() error {
				_, err := JellyfinSearchItems(ctx, c, JellyfinItemQuery{ParentID: bad + "x"})
				return err
			}(),
		}
		for name, err := range calls {
			if err == nil {
				t.Errorf("%s accepted %q", name, bad)
			}
		}
	}
	if len(got.paths) != 0 {
		t.Errorf("requests reached the server: %v", got.paths)
	}
}

func TestJellyfinAcceptsBothIDForms(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := jellyfinClient(srv.URL)
	for _, id := range []string{"748950F8695760523D0361EA48E0953B", "748950f8-6957-6052-3d03-61ea48e0953b"} {
		if err := JellyfinRunTask(context.Background(), c, id); err != nil {
			t.Errorf("id %q rejected: %v", id, err)
		}
	}
	if len(got.paths) != 2 {
		t.Errorf("requests = %v", got.paths)
	}
}

func TestJellyfinSearchItemsDefaultsAndCapsTheLimit(t *testing.T) {
	for _, tc := range []struct {
		in   int
		want string
	}{{0, "limit=25"}, {-3, "limit=25"}, {40, "limit=40"}, {100000, "limit=100"}} {
		srv, got := fakeService(t, 200, `{"Items":[]}`)
		if _, err := JellyfinSearchItems(context.Background(), jellyfinClient(srv.URL), JellyfinItemQuery{Limit: tc.in}); err != nil {
			t.Fatalf("limit %d: %v", tc.in, err)
		}
		if !strings.Contains(got.query, tc.want+"&") && !strings.HasSuffix(got.query, tc.want) {
			t.Errorf("limit %d: query %q, want %s", tc.in, got.query, tc.want)
		}
	}
}

func TestJellyfinActivityLogDefaultsAndCapsTheLimit(t *testing.T) {
	for _, tc := range []struct {
		in   int
		want string
	}{{0, "limit=25"}, {100000, "limit=100"}} {
		srv, got := fakeService(t, 200, `{"Items":[]}`)
		if _, err := JellyfinActivityLog(context.Background(), jellyfinClient(srv.URL), tc.in, 0); err != nil {
			t.Fatalf("limit %d: %v", tc.in, err)
		}
		if got.query != tc.want {
			t.Errorf("limit %d: query %q, want %s", tc.in, got.query, tc.want)
		}
	}
}

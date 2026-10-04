package arr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

const jellystatKey = "js-test-key-1234"

const (
	jsLib  = "f137a2dd21bbc1b99aa5c0f6bf02a805"
	jsItem = "0a1b2c3d4e5f60718293a4b5c6d7e8f9"
	jsUser = "11112222333344445555666677778888"
)

func jellystatClient(url string) *Client {
	return NewClient(url, JellystatSpec, Credentials{APIKey: jellystatKey})
}

// wantJellystatAuth fails unless the raw key travelled in X-Api-Token. The map
// is indexed by the canonical key net/http puts on the wire; Header.Get would
// pass whichever spelling the code chose.
func wantJellystatAuth(t *testing.T, got *capture) {
	t.Helper()
	if v := got.header["X-Api-Token"]; len(v) != 1 || v[0] != jellystatKey {
		t.Errorf("X-Api-Token = %q, want the raw key", v)
	}
	if v := got.header.Get("Authorization"); v != "" {
		t.Errorf("Authorization = %q, want unset", v)
	}
}

func wantBody(t *testing.T, got *capture, want map[string]any) {
	t.Helper()
	var have map[string]any
	if err := json.Unmarshal([]byte(got.body), &have); err != nil {
		t.Fatalf("body %q is not JSON: %v", got.body, err)
	}
	if !reflect.DeepEqual(have, want) {
		t.Errorf("body = %v, want %v", have, want)
	}
}

func wantQuery(t *testing.T, got *capture, want url.Values) {
	t.Helper()
	have, err := url.ParseQuery(got.query)
	if err != nil {
		t.Fatalf("query %q: %v", got.query, err)
	}
	if !reflect.DeepEqual(have, want) {
		t.Errorf("query = %v, want %v", have, want)
	}
}

func TestJellystatPingUsesAuthenticatedLibraryOverview(t *testing.T) {
	srv, got := fakeService(t, 200, `[]`)
	if err := jellystatClient(srv.URL).Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if got.method != "GET" || got.path != "/stats/getLibraryOverview" {
		t.Errorf("status request = %s %s", got.method, got.path)
	}
	wantJellystatAuth(t, got)
}

func TestJellystatPingFailsOnRejectedKey(t *testing.T) {
	srv, _ := fakeService(t, 403, `{"message":"Invalid API key"}`)
	if err := jellystatClient(srv.URL).Ping(context.Background()); err == nil {
		t.Fatal("Ping with a rejected key returned nil")
	}
}

func TestJellystatLibraryOverviewIsTrimmed(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"Id":"`+jsLib+`","Name":"Movies","CollectionType":"movies",
	  "Library_Count":"120","Season_Count":"0","Episode_Count":"0"}]`)
	libs, err := JellystatLibraryOverview(context.Background(), jellystatClient(srv.URL))
	if err != nil {
		t.Fatalf("JellystatLibraryOverview: %v", err)
	}
	if got.method != "GET" || got.path != "/stats/getLibraryOverview" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellystatAuth(t, got)
	want := []JellystatLibraryCount{{ID: jsLib, Name: "Movies", CollectionType: "movies", Items: 120}}
	if !reflect.DeepEqual(libs, want) {
		t.Errorf("libs = %+v, want %+v", libs, want)
	}
}

func TestJellystatListLibrariesProjectsStats(t *testing.T) {
	srv, got := fakeService(t, 200, `[
	 {"Id":"`+jsLib+`","Name":"Shows","ServerId":"s","IsFolder":true,"Type":"CollectionFolder","CollectionType":"tvshows",
	  "ImageTagsPrimary":"x","ItemId":"`+jsItem+`","ItemName":"Severance","ItemType":"Series","PrimaryImageHash":"h",
	  "SeasonNumber":2,"EpisodeNumber":3,"EpisodeName":"Who Is Alive?","Plays":"42","total_playback_duration":"91000",
	  "total_play_time":"5","Library_Count":10,"Season_Count":20,"Episode_Count":"200","archived":false,
	  "ActivityDateInserted":"2026-09-30T20:00:00.000Z","LastActivity":{"days":4}},
	 {"Id":"b","Name":"Music","CollectionType":"music","Plays":null,"total_playback_duration":null,"Library_Count":3,
	  "Season_Count":0,"Episode_Count":0,"archived":true,"ActivityDateInserted":null}]`)
	libs, err := JellystatListLibraries(context.Background(), jellystatClient(srv.URL))
	if err != nil {
		t.Fatalf("JellystatListLibraries: %v", err)
	}
	if got.method != "GET" || got.path != "/stats/getLibraryCardStats" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellystatAuth(t, got)
	want := []JellystatLibraryStats{
		{ID: jsLib, Name: "Shows", CollectionType: "tvshows", Plays: 42, WatchSeconds: 91000, Items: 10, Seasons: 20,
			Episodes: 200, LastPlayedAt: "2026-09-30T20:00:00.000Z", LastPlayedItem: "Severance - Who Is Alive?"},
		{ID: "b", Name: "Music", CollectionType: "music", Items: 3, Archived: true},
	}
	if !reflect.DeepEqual(libs, want) {
		t.Errorf("libs = %+v\nwant   %+v", libs, want)
	}
}

const historyRow = `{"Id":"9","IsPaused":false,"UserId":"` + jsUser + `","UserName":"alice","Client":"Jellyfin Web",
 "DeviceName":"Firefox","DeviceId":"d","ApplicationVersion":"10","NowPlayingItemId":"` + jsItem + `",
 "NowPlayingItemName":"Who Is Alive?","SeasonId":"s","SeriesName":"Severance","EpisodeId":"e","PlaybackDuration":"2400",
 "ActivityDateInserted":"2026-09-30T20:00:00.000Z","PlayMethod":"DirectPlay","MediaStreams":[{"Codec":"h264"}],
 "TranscodingInfo":{"Bitrate":1},"PlayState":{"x":1},"RemoteEndPoint":"10.0.0.9","SeasonNumber":2,"EpisodeNumber":3,
 "TotalPlays":"4","results":[{"huge":"blob"}]}`

func historyBody(row string) string {
	return `{"current_page":"2","pages":7,"size":"25","sort":"ActivityDateInserted","desc":"true","results":[` + row + `]}`
}

func TestJellystatPlaybackHistoryAllUsers(t *testing.T) {
	srv, got := fakeService(t, 200, historyBody(historyRow))
	page, err := JellystatPlaybackHistory(context.Background(), jellystatClient(srv.URL), JellystatHistoryQuery{Page: 2})
	if err != nil {
		t.Fatalf("JellystatPlaybackHistory: %v", err)
	}
	if got.method != "GET" || got.path != "/api/getHistory" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellystatAuth(t, got)
	wantQuery(t, got, url.Values{"size": {"25"}, "page": {"2"}, "desc": {"true"}})
	if got.body != "" {
		t.Errorf("GET sent body %q", got.body)
	}
	three, two := 3, 2
	want := JellystatHistoryPage{
		JellystatPaging: JellystatPaging{Page: 2, Pages: 7, Size: 25, Count: 1},
		Plays: []JellystatPlay{{UserID: jsUser, UserName: "alice", ItemID: jsItem, Title: "Who Is Alive?",
			SeriesName: "Severance", SeasonNumber: &two, EpisodeNumber: &three, PlayedAt: "2026-09-30T20:00:00.000Z",
			DurationSeconds: 2400, Client: "Jellyfin Web", DeviceName: "Firefox", PlayMethod: "DirectPlay", TotalPlays: 4}},
	}
	if !reflect.DeepEqual(page, want) {
		t.Errorf("page = %+v\nwant   %+v", page, want)
	}
	if b, _ := json.Marshal(page); strings.Contains(string(b), "blob") || strings.Contains(string(b), "10.0.0.9") {
		t.Errorf("projection leaked upstream fields: %s", b)
	}
}

func TestJellystatPlaybackHistoryScopedPosts(t *testing.T) {
	cases := []struct {
		name     string
		q        JellystatHistoryQuery
		wantPath string
		wantBody map[string]any
	}{
		{"user", JellystatHistoryQuery{UserID: jsUser}, "/api/getUserHistory", map[string]any{"userid": jsUser}},
		{"item", JellystatHistoryQuery{ItemID: jsItem}, "/api/getItemHistory", map[string]any{"itemid": jsItem}},
		{"library", JellystatHistoryQuery{LibraryID: jsLib}, "/api/getLibraryHistory", map[string]any{"libraryid": jsLib}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, got := fakeService(t, 200, historyBody(historyRow))
			tc.q.Search, tc.q.Sort = "sever", "PlaybackDuration"
			f := false
			tc.q.Desc = &f
			if _, err := JellystatPlaybackHistory(context.Background(), jellystatClient(srv.URL), tc.q); err != nil {
				t.Fatalf("JellystatPlaybackHistory: %v", err)
			}
			if got.method != "POST" || got.path != tc.wantPath {
				t.Errorf("request = %s %s, want POST %s", got.method, got.path, tc.wantPath)
			}
			wantJellystatAuth(t, got)
			wantBody(t, got, tc.wantBody)
			wantQuery(t, got, url.Values{"size": {"25"}, "page": {"1"}, "desc": {"false"},
				"search": {"sever"}, "sort": {"PlaybackDuration"}})
		})
	}
}

func TestJellystatPlaybackHistoryClampsSize(t *testing.T) {
	for in, want := range map[int]string{0: "25", -3: "25", 40: "40", 100: "100", 5000: "100"} {
		srv, got := fakeService(t, 200, historyBody(historyRow))
		if _, err := JellystatPlaybackHistory(context.Background(), jellystatClient(srv.URL), JellystatHistoryQuery{Limit: in}); err != nil {
			t.Fatalf("limit %d: %v", in, err)
		}
		if v, _ := url.ParseQuery(got.query); v.Get("size") != want {
			t.Errorf("limit %d sent size=%s, want %s", in, v.Get("size"), want)
		}
	}
}

func TestJellystatPlaybackHistoryRejectsBadIDsBeforeAnyRequest(t *testing.T) {
	for _, tc := range []struct {
		name string
		q    JellystatHistoryQuery
	}{
		{"user traversal", JellystatHistoryQuery{UserID: "../../api/keys"}},
		{"item traversal", JellystatHistoryQuery{ItemID: "../../../sync/beginSync"}},
		{"library traversal", JellystatHistoryQuery{LibraryID: "..%2f..%2fbackup"}},
		{"two scopes", JellystatHistoryQuery{UserID: jsUser, ItemID: jsItem}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, got := fakeService(t, 200, historyBody(historyRow))
			if _, err := JellystatPlaybackHistory(context.Background(), jellystatClient(srv.URL), tc.q); err == nil {
				t.Fatal("expected an error")
			}
			if len(got.paths) != 0 {
				t.Errorf("request reached the server: %v", got.paths)
			}
		})
	}
}

// Jellystat silently falls back to its default order for a sort it does not
// know, so an unknown value must be refused here rather than sent on.
func TestJellystatRejectsUnknownSortBeforeAnyRequest(t *testing.T) {
	for _, sortBy := range []string{"a.UserName; DROP TABLE jf_users", "Views", "RemoteEndPoint"} {
		srv, got := fakeService(t, 200, historyBody(historyRow))
		_, err := JellystatPlaybackHistory(context.Background(), jellystatClient(srv.URL), JellystatHistoryQuery{Sort: sortBy})
		if err == nil || !strings.Contains(err.Error(), "ActivityDateInserted") {
			t.Errorf("history sort %q: err = %v, want one naming the valid values", sortBy, err)
		}
		if len(got.paths) != 0 {
			t.Errorf("history sort %q: request reached the server: %v", sortBy, got.paths)
		}
	}
	for _, sortBy := range []string{"DateCreated desc", "UserName"} {
		srv, got := fakeService(t, 200, `{"results":[]}`)
		_, err := JellystatLibraryItems(context.Background(), jellystatClient(srv.URL), JellystatLibraryItemsQuery{LibraryID: jsLib, Sort: sortBy})
		if err == nil || !strings.Contains(err.Error(), "WatchTime") {
			t.Errorf("library items sort %q: err = %v, want one naming the valid values", sortBy, err)
		}
		if len(got.paths) != 0 {
			t.Errorf("library items sort %q: request reached the server: %v", sortBy, got.paths)
		}
	}
}

func TestJellystatMostViewedAndPopular(t *testing.T) {
	cases := []struct {
		name, path, rows string
		call             func(context.Context, *Client, string, int) ([]JellystatTopItem, error)
		want             JellystatTopItem
	}{
		{"viewed", "/stats/getMostViewedByType",
			`[{"Plays":"12","total_playback_duration":"7200","Name":"Dune","Id":"` + jsItem + `","PrimaryImageHash":"h","archived":false}]`,
			JellystatMostViewed, JellystatTopItem{ID: jsItem, Name: "Dune", Plays: 12, WatchSeconds: 7200}},
		{"popular", "/stats/getMostPopularByType",
			`[{"unique_viewers":"3","latest_activity_date":"2026-09-30T20:00:00.000Z","Name":"Dune","Id":"` + jsItem + `","archived":true}]`,
			JellystatMostPopular, JellystatTopItem{ID: jsItem, Name: "Dune", UniqueViewers: 3, LastPlayedAt: "2026-09-30T20:00:00.000Z", Archived: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, got := fakeService(t, 200, tc.rows)
			items, err := tc.call(context.Background(), jellystatClient(srv.URL), "Movie", 0)
			if err != nil {
				t.Fatalf("call: %v", err)
			}
			if got.method != "POST" || got.path != tc.path {
				t.Errorf("request = %s %s", got.method, got.path)
			}
			wantJellystatAuth(t, got)
			wantBody(t, got, map[string]any{"type": "Movie", "days": float64(30)})
			if !reflect.DeepEqual(items, []JellystatTopItem{tc.want}) {
				t.Errorf("items = %+v, want %+v", items, tc.want)
			}
		})
	}
}

func TestJellystatMostViewedSendsDaysAndRejectsBadType(t *testing.T) {
	srv, got := fakeService(t, 200, `[]`)
	if _, err := JellystatMostViewed(context.Background(), jellystatClient(srv.URL), "Series", 7); err != nil {
		t.Fatalf("JellystatMostViewed: %v", err)
	}
	wantBody(t, got, map[string]any{"type": "Series", "days": float64(7)})

	for _, bad := range []string{"", "movie", "Episode", "Movie'; drop table x;--"} {
		srv, got := fakeService(t, 200, `[]`)
		if _, err := JellystatMostViewed(context.Background(), jellystatClient(srv.URL), bad, 7); err == nil {
			t.Errorf("type %q accepted", bad)
		}
		if _, err := JellystatMostPopular(context.Background(), jellystatClient(srv.URL), bad, 7); err == nil {
			t.Errorf("popular type %q accepted", bad)
		}
		if len(got.paths) != 0 {
			t.Errorf("type %q reached the server", bad)
		}
	}
}

func TestJellystatMostActiveUsers(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"Plays":"9","UserId":"`+jsUser+`","Name":"alice"}]`)
	users, err := JellystatMostActiveUsers(context.Background(), jellystatClient(srv.URL), 14)
	if err != nil {
		t.Fatalf("JellystatMostActiveUsers: %v", err)
	}
	if got.method != "POST" || got.path != "/stats/getMostActiveUsers" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellystatAuth(t, got)
	wantBody(t, got, map[string]any{"days": float64(14)})
	if want := []JellystatActiveUser{{UserID: jsUser, Name: "alice", Plays: 9}}; !reflect.DeepEqual(users, want) {
		t.Errorf("users = %+v, want %+v", users, want)
	}
}

func userActivityBody(n int) string {
	rows := make([]string, n)
	for i := range rows {
		rows[i] = fmt.Sprintf(`{"UserId":"%032x","PrimaryImageTag":"t","UserName":"user%d","LastWatched":"Severance - Hello",
		 "NowPlayingItemId":"e","LastActivityDate":"2026-09-30T20:00:00.000Z","LastClient":"Web - Firefox",
		 "TotalPlays":"%d","TotalWatchTime":"3600","LastSeen":{"days":1}}`, i, i, i)
	}
	return "[" + strings.Join(rows, ",") + "]"
}

func TestJellystatUserActivityPagesInTheClient(t *testing.T) {
	srv, got := fakeService(t, 200, userActivityBody(130))
	c := jellystatClient(srv.URL)

	page, err := JellystatUserActivity(context.Background(), c, 0, 0)
	if err != nil {
		t.Fatalf("JellystatUserActivity: %v", err)
	}
	if got.method != "GET" || got.path != "/stats/getAllUserActivity" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellystatAuth(t, got)
	if page.Total != 130 || page.Count != 25 || len(page.Users) != 25 || page.Offset != 0 {
		t.Errorf("default page = total %d count %d offset %d, want 130/25/0", page.Total, page.Count, page.Offset)
	}
	u := page.Users[3]
	if u.UserName != "user3" || u.TotalPlays != 3 || u.TotalWatchSeconds != 3600 || u.LastClient != "Web - Firefox" ||
		u.LastWatched != "Severance - Hello" || u.LastActiveAt != "2026-09-30T20:00:00.000Z" {
		t.Errorf("user = %+v", u)
	}

	page, _ = JellystatUserActivity(context.Background(), c, 5000, 100)
	if page.Count != 30 || page.Offset != 100 || page.Users[0].UserName != "user100" {
		t.Errorf("tail page = count %d offset %d first %q", page.Count, page.Offset, page.Users[0].UserName)
	}
	page, _ = JellystatUserActivity(context.Background(), c, 200, 0)
	if page.Count != 100 {
		t.Errorf("cap: count = %d, want 100", page.Count)
	}
	page, _ = JellystatUserActivity(context.Background(), c, 10, 999)
	if page.Count != 0 || page.Total != 130 || page.Users == nil {
		t.Errorf("past the end = %+v", page)
	}
}

func TestJellystatItemStats(t *testing.T) {
	srv, got := fakeService(t, 200, `{"Plays":"5","total_playback_duration":"1800"}`)
	st, err := JellystatItemStats(context.Background(), jellystatClient(srv.URL), jsItem, 0)
	if err != nil {
		t.Fatalf("JellystatItemStats: %v", err)
	}
	if got.method != "POST" || got.path != "/stats/getGlobalItemStats" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellystatAuth(t, got)
	wantBody(t, got, map[string]any{"itemid": jsItem, "hours": float64(24)})
	if want := (JellystatItemStat{ItemID: jsItem, Hours: 24, Plays: 5, WatchSeconds: 1800}); st != want {
		t.Errorf("stat = %+v, want %+v", st, want)
	}

	srv, _ = fakeService(t, 200, `{"Plays":"0","total_playback_duration":null}`)
	st, err = JellystatItemStats(context.Background(), jellystatClient(srv.URL), jsItem, 720)
	if err != nil || st.Plays != 0 || st.WatchSeconds != 0 || st.Hours != 720 {
		t.Errorf("unplayed item = %+v, %v", st, err)
	}
}

func TestJellystatItemStatsRejectsBadIDBeforeRequest(t *testing.T) {
	srv, got := fakeService(t, 200, `{}`)
	if _, err := JellystatItemStats(context.Background(), jellystatClient(srv.URL), "../../api/keys", 24); err == nil {
		t.Fatal("expected an error")
	}
	if len(got.paths) != 0 {
		t.Errorf("request reached the server: %v", got.paths)
	}
}

func TestJellystatLibraryItems(t *testing.T) {
	srv, got := fakeService(t, 200, `{"current_page":"1","pages":3,"size":"10","sort":"Views","desc":"false","results":[
	 {"Id":"`+jsItem+`","Name":"Dune","ServerId":"s","PremiereDate":"x","DateCreated":"2025-01-02T00:00:00.000Z",
	  "CommunityRating":8,"RunTimeTicks":1,"ProductionYear":2021,"IsFolder":false,"Type":"Movie","Status":null,
	  "ImageTagsPrimary":"a","PrimaryImageHash":"h","ParentId":"`+jsLib+`","archived":false,"Size":5000000000,
	  "times_played":0,"total_play_time":0}]}`)
	f := false
	page, err := JellystatLibraryItems(context.Background(), jellystatClient(srv.URL), JellystatLibraryItemsQuery{
		LibraryID: jsLib, Limit: 10, Page: 1, Sort: "Views", Desc: &f, Search: "dun"})
	if err != nil {
		t.Fatalf("JellystatLibraryItems: %v", err)
	}
	if got.method != "POST" || got.path != "/stats/getLibraryItemsWithStats" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellystatAuth(t, got)
	wantBody(t, got, map[string]any{"libraryid": jsLib})
	wantQuery(t, got, url.Values{"size": {"10"}, "page": {"1"}, "sort": {"Views"}, "desc": {"false"}, "search": {"dun"}})
	want := JellystatItemPage{
		JellystatPaging: JellystatPaging{Page: 1, Pages: 3, Size: 10, Count: 1},
		Items: []JellystatLibraryItem{{ID: jsItem, Name: "Dune", Type: "Movie", Year: 2021,
			AddedAt: "2025-01-02T00:00:00.000Z", SizeBytes: 5000000000}},
	}
	if !reflect.DeepEqual(page, want) {
		t.Errorf("page = %+v\nwant   %+v", page, want)
	}
}

func TestJellystatLibraryItemsDefaultsAndGuards(t *testing.T) {
	srv, got := fakeService(t, 200, `{"current_page":1,"pages":0,"size":25,"results":[]}`)
	c := jellystatClient(srv.URL)
	page, err := JellystatLibraryItems(context.Background(), c, JellystatLibraryItemsQuery{LibraryID: jsLib, Limit: 9999})
	if err != nil {
		t.Fatalf("JellystatLibraryItems: %v", err)
	}
	wantQuery(t, got, url.Values{"size": {"100"}, "page": {"1"}, "desc": {"true"}})
	if page.Items == nil {
		t.Error("Items is nil, want an empty list")
	}

	srv, got = fakeService(t, 200, `{}`)
	for _, id := range []string{"", "../../sync", "abc"} {
		if _, err := JellystatLibraryItems(context.Background(), jellystatClient(srv.URL), JellystatLibraryItemsQuery{LibraryID: id}); err == nil {
			t.Errorf("library id %q accepted", id)
		}
	}
	if len(got.paths) != 0 {
		t.Errorf("request reached the server: %v", got.paths)
	}
}

func TestJellystatViewsOverTime(t *testing.T) {
	srv, got := fakeService(t, 200, `{"libraries":[{"Id":"a","Name":"Movies"},{"Id":"b","Name":"Shows"}],"stats":[
	 {"Key":"Sep 29, 2026","Movies":{"count":"2","duration":"90"},"Shows":{"count":0,"duration":0}},
	 {"Key":"Sep 30, 2026","Movies":{"count":1,"duration":"45"}}]}`)
	v, err := JellystatViewsOverTime(context.Background(), jellystatClient(srv.URL), 0)
	if err != nil {
		t.Fatalf("JellystatViewsOverTime: %v", err)
	}
	if got.method != "GET" || got.path != "/stats/getViewsOverTime" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellystatAuth(t, got)
	wantQuery(t, got, url.Values{"days": {"30"}})
	want := JellystatViews{Days: 30, Dates: []JellystatDayViews{
		{Date: "Sep 29, 2026", Libraries: []JellystatLibraryViews{{Library: "Movies", Plays: 2, Minutes: 90}, {Library: "Shows"}}},
		{Date: "Sep 30, 2026", Libraries: []JellystatLibraryViews{{Library: "Movies", Plays: 1, Minutes: 45}}},
	}}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("views = %+v\nwant   %+v", v, want)
	}

	if _, err = JellystatViewsOverTime(context.Background(), jellystatClient(srv.URL), 100000); err != nil {
		t.Fatal(err)
	}
	wantQuery(t, got, url.Values{"days": {"365"}})
}

func TestJellystatCallsSurfaceUpstreamErrors(t *testing.T) {
	ctx := context.Background()
	calls := map[string]func(*Client) error{
		"overview":  func(c *Client) error { _, err := JellystatLibraryOverview(ctx, c); return err },
		"libraries": func(c *Client) error { _, err := JellystatListLibraries(ctx, c); return err },
		"history":   func(c *Client) error { _, err := JellystatPlaybackHistory(ctx, c, JellystatHistoryQuery{}); return err },
		"viewed":    func(c *Client) error { _, err := JellystatMostViewed(ctx, c, "Movie", 7); return err },
		"popular":   func(c *Client) error { _, err := JellystatMostPopular(ctx, c, "Movie", 7); return err },
		"active":    func(c *Client) error { _, err := JellystatMostActiveUsers(ctx, c, 7); return err },
		"users":     func(c *Client) error { _, err := JellystatUserActivity(ctx, c, 0, 0); return err },
		"item":      func(c *Client) error { _, err := JellystatItemStats(ctx, c, jsItem, 1); return err },
		"items": func(c *Client) error {
			_, err := JellystatLibraryItems(ctx, c, JellystatLibraryItemsQuery{LibraryID: jsLib})
			return err
		},
		"views": func(c *Client) error { _, err := JellystatViewsOverTime(ctx, c, 7); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			srv, _ := fakeService(t, http.StatusServiceUnavailable, `"boom"`)
			var se *StatusError
			err := call(jellystatClient(srv.URL))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !errors.As(err, &se) || se.Status != 503 {
				t.Errorf("err = %v, want a 503 StatusError", err)
			}
		})
	}
}

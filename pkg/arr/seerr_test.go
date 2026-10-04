package arr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// seerrClient points a client at the fake upstream with a recognisable key.
func seerrClient(url string) *Client {
	return NewClient(url, SeerrSpec, Credentials{APIKey: "seerr-key"})
}

// seerrRoutes serves a fixed body per "METHOD /path" and records each request
// line, for calls that make more than one upstream request.
func seerrRoutes(t *testing.T, routes map[string]string) (*Client, *[]string) {
	t.Helper()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		paths = append(paths, key)
		body, ok := routes[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return seerrClient(srv.URL), &paths
}

// Seerr's /status answers without a key, so a wrong key would pass --check if
// the status path were /status. /settings/about needs the key.
func TestSeerrSpecPingsAuthenticatedPathWithKeyHeader(t *testing.T) {
	srv, got := fakeService(t, 200, `{"version":"3.4.1"}`)

	if err := seerrClient(srv.URL).Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if got.path != "/api/v1/settings/about" {
		t.Errorf("status path = %q, want /api/v1/settings/about", got.path)
	}
	if v := got.header.Get("X-Api-Key"); v != "seerr-key" {
		t.Errorf("X-Api-Key = %q, want seerr-key", v)
	}
}

func TestSeerrStatusCombinesVersionAndTotals(t *testing.T) {
	c, paths := seerrRoutes(t, map[string]string{
		"GET /api/v1/status":         `{"version":"3.4.1","commitTag":"abc","updateAvailable":true,"commitsBehind":0,"restartRequired":false}`,
		"GET /api/v1/settings/about": `{"version":"3.4.1","totalMediaItems":378,"totalRequests":290,"tz":"Asia/Singapore","appDataPath":"/app/config"}`,
	})

	st, err := SeerrGetStatus(context.Background(), c)
	if err != nil {
		t.Fatalf("SeerrGetStatus: %v", err)
	}
	if st.Version != "3.4.1" || !st.UpdateAvailable || st.TotalRequests != 290 || st.TotalMediaItems != 378 {
		t.Errorf("status = %+v", st)
	}
	if strings.Contains(mustJSON(t, st), "/app/config") {
		t.Errorf("appDataPath leaked into the projection: %+v", st)
	}
	if len(*paths) != 2 {
		t.Errorf("requests = %v, want /status and /settings/about", *paths)
	}
}

func TestSeerrRequestCounts(t *testing.T) {
	srv, got := fakeService(t, 200, `{"total":290,"movie":218,"tv":72,"pending":0,"approved":68,"declined":0,"processing":11,"available":57,"completed":222}`)

	n, err := SeerrRequestCounts(context.Background(), seerrClient(srv.URL))
	if err != nil {
		t.Fatalf("SeerrRequestCounts: %v", err)
	}
	if got.path != "/api/v1/request/count" || got.method != "GET" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if n.Total != 290 || n.Movie != 218 || n.TV != 72 || n.Approved != 68 || n.Completed != 222 {
		t.Errorf("counts = %+v", n)
	}
}

const seerrRequestBody = `{"id":470,"status":5,"createdAt":"2026-10-03T14:52:20.000Z","updatedAt":"2026-10-03T19:00:19.000Z",
 "type":"movie","is4k":false,"serverId":null,"profileId":null,"rootFolder":null,"seasons":[],
 "media":{"id":496,"mediaType":"movie","tmdbId":550,"tvdbId":null,"imdbId":"tt0137523","status":5,
   "jellyfinMediaId":"6b66","mediaUrl":"http://jf/web","serviceUrl":"http://radarr/movie/550"},
 "modifiedBy":{"id":1,"email":"admin@example.com","displayName":"Admin"},
 "requestedBy":{"id":13,"email":"soumil@example.com","permissions":1145045024,"jellyfinUserId":"d734","displayName":"soumil"}}`

// A request carries only TMDB ids, never a title, and its embedded users carry
// email and permission bits that no question about a request needs.
func TestSeerrListRequestsSendsFiltersAndTrims(t *testing.T) {
	srv, got := fakeService(t, 200, `{"pageInfo":{"pages":145,"pageSize":2,"results":290,"page":1},"results":[`+
		seerrRequestBody+`,{"id":469,"status":4,"type":"tv","is4k":true,"media":{"tmdbId":1398,"tvdbId":75299,"status":4},
		"seasons":[{"id":456,"seasonNumber":1,"status":5},{"id":457,"seasonNumber":2,"status":3}],
		"requestedBy":{"id":1,"displayName":"Arkhaya"}}],"serviceErrors":{"radarr":[],"sonarr":[]}}`)

	page, err := SeerrListRequests(context.Background(), seerrClient(srv.URL), SeerrRequestQuery{
		Filter: "approved", Sort: "modified", Take: 2, Skip: 4, RequestedBy: 13,
	})
	if err != nil {
		t.Fatalf("SeerrListRequests: %v", err)
	}
	if got.path != "/api/v1/request" || got.method != "GET" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	q, _ := url.ParseQuery(got.query)
	for k, want := range map[string]string{"filter": "approved", "sort": "modified", "take": "2", "skip": "4", "requestedBy": "13"} {
		if q.Get(k) != want {
			t.Errorf("query %s = %q, want %q (raw %q)", k, q.Get(k), want, got.query)
		}
	}
	if got.header.Get("X-Api-Key") != "seerr-key" {
		t.Errorf("X-Api-Key = %q", got.header.Get("X-Api-Key"))
	}
	if page.Total != 290 || page.Pages != 145 || page.Page != 1 || page.Count != 2 || len(page.Requests) != 2 {
		t.Fatalf("page = %+v", page)
	}
	r := page.Requests[0]
	if r.ID != 470 || r.Status != "COMPLETED" || r.Type != "movie" || r.TMDBID != 550 || r.MediaStatus != "AVAILABLE" {
		t.Errorf("request = %+v", r)
	}
	if r.RequestedBy == nil || r.RequestedBy.ID != 13 || r.RequestedBy.Name != "soumil" {
		t.Errorf("requestedBy = %+v", r.RequestedBy)
	}
	tv := page.Requests[1]
	if tv.Status != "FAILED" || !tv.Is4K || tv.TVDBID != 75299 || tv.MediaStatus != "PARTIALLY_AVAILABLE" {
		t.Errorf("tv request = %+v", tv)
	}
	if len(tv.Seasons) != 2 || tv.Seasons[1].Number != 2 || tv.Seasons[1].Status != "DECLINED" {
		t.Errorf("seasons = %+v", tv.Seasons)
	}
	out := mustJSON(t, page)
	for _, leaked := range []string{"example.com", "1145045024", "jellyfinUserId", "serviceUrl", "mediaUrl"} {
		if strings.Contains(out, leaked) {
			t.Errorf("projection leaked %q: %s", leaked, out)
		}
	}
}

func TestSeerrListRequestsDefaultsAndValidation(t *testing.T) {
	srv, got := fakeService(t, 200, `{"pageInfo":{"pages":0,"pageSize":20,"results":0,"page":1},"results":[]}`)
	c := seerrClient(srv.URL)

	page, err := SeerrListRequests(context.Background(), c, SeerrRequestQuery{})
	if err != nil {
		t.Fatalf("SeerrListRequests: %v", err)
	}
	if page.Requests == nil || page.Count != 0 {
		t.Errorf("page = %+v, want an empty non-nil list", page)
	}
	q, _ := url.ParseQuery(got.query)
	if q.Get("take") != "20" || q.Has("filter") || q.Has("requestedBy") || q.Has("skip") {
		t.Errorf("default query = %q, want only take=20", got.query)
	}

	got.path = ""
	for _, bad := range []SeerrRequestQuery{{Filter: "bogus"}, {Sort: "name"}} {
		if _, err := SeerrListRequests(context.Background(), c, bad); err == nil || !strings.Contains(err.Error(), "must be one of") {
			t.Errorf("query %+v: error = %v, want a must be one of error", bad, err)
		}
	}
	if got.path != "" {
		t.Errorf("an invalid query reached the server: %s", got.path)
	}
}

func TestSeerrGetRequest(t *testing.T) {
	srv, got := fakeService(t, 200, seerrRequestBody)

	r, err := SeerrGetRequest(context.Background(), seerrClient(srv.URL), 470)
	if err != nil {
		t.Fatalf("SeerrGetRequest: %v", err)
	}
	if got.path != "/api/v1/request/470" || got.method != "GET" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if r.ID != 470 || r.TMDBID != 550 {
		t.Errorf("request = %+v", r)
	}
}

func TestSeerrSearchTrimsAndNamesAvailability(t *testing.T) {
	srv, got := fakeService(t, 200, `{"page":1,"totalPages":5,"totalResults":81,"results":[
	 {"id":550,"mediaType":"movie","title":"Fight Club","releaseDate":"1999-10-15","overview":"long text","voteAverage":8.4,
	  "posterPath":"/p.jpg","mediaInfo":{"id":496,"status":5,"serviceUrl":"http://radarr/x"}},
	 {"id":1398,"mediaType":"tv","name":"The Sopranos","firstAirDate":"1999-01-10"},
	 {"id":287,"mediaType":"person","name":"Brad Pitt"}]}`)

	page, err := SeerrSearch(context.Background(), seerrClient(srv.URL), "fight club", 2)
	if err != nil {
		t.Fatalf("SeerrSearch: %v", err)
	}
	if got.path != "/api/v1/search" {
		t.Errorf("path = %q", got.path)
	}
	q, _ := url.ParseQuery(got.query)
	if q.Get("query") != "fight club" || q.Get("page") != "2" {
		t.Errorf("query = %q", got.query)
	}
	if page.Page != 1 || page.TotalPages != 5 || page.TotalResults != 81 || page.Count != 3 {
		t.Fatalf("page = %+v", page)
	}
	m, tv, p := page.Results[0], page.Results[1], page.Results[2]
	if m.TMDBID != 550 || m.Title != "Fight Club" || m.ReleaseDate != "1999-10-15" || m.Availability != "AVAILABLE" {
		t.Errorf("movie = %+v", m)
	}
	if tv.Title != "The Sopranos" || tv.ReleaseDate != "1999-01-10" || tv.Availability != "" {
		t.Errorf("tv = %+v", tv)
	}
	if p.MediaType != "person" || p.Title != "Brad Pitt" {
		t.Errorf("person = %+v", p)
	}
	if out := mustJSON(t, page); strings.Contains(out, "long text") || strings.Contains(out, "radarr") {
		t.Errorf("projection kept overview or service url: %s", out)
	}
}

func TestSeerrSearchRequiresAQuery(t *testing.T) {
	srv, got := fakeService(t, 200, `{}`)
	if _, err := SeerrSearch(context.Background(), seerrClient(srv.URL), "  ", 1); err == nil {
		t.Fatal("blank query accepted")
	}
	if got.path != "" {
		t.Errorf("blank query reached the server")
	}
}

func TestSeerrDiscoverMapsKindToPath(t *testing.T) {
	for kind, path := range map[string]string{
		"trending": "/api/v1/discover/trending",
		"movies":   "/api/v1/discover/movies",
		"tv":       "/api/v1/discover/tv",
	} {
		t.Run(kind, func(t *testing.T) {
			srv, got := fakeService(t, 200, `{"page":3,"totalPages":500,"totalResults":10000,"results":[{"id":1,"mediaType":"tv","name":"X"}]}`)

			page, err := SeerrDiscover(context.Background(), seerrClient(srv.URL), kind, 3)
			if err != nil {
				t.Fatalf("SeerrDiscover: %v", err)
			}
			if got.path != path {
				t.Errorf("path = %q, want %q", got.path, path)
			}
			if got.query != "page=3" {
				t.Errorf("query = %q, want page=3", got.query)
			}
			if page.Count != 1 || page.Results[0].Title != "X" {
				t.Errorf("page = %+v", page)
			}
		})
	}
	if _, err := SeerrDiscover(context.Background(), seerrClient("http://unused"), "upcoming", 1); err == nil {
		t.Error("unknown kind accepted")
	}
}

func TestSeerrGetMovieAndTV(t *testing.T) {
	t.Run("movie", func(t *testing.T) {
		srv, got := fakeService(t, 200, `{"id":550,"title":"Fight Club","releaseDate":"1999-10-15","status":"Released","runtime":139,
		 "overview":"A ticking-time-bomb insomniac","genres":[{"id":18,"name":"Drama"},{"id":53,"name":"Thriller"}],
		 "voteAverage":8.437,"imdbId":"tt0137523","credits":{"cast":[1,2,3]},"relatedVideos":[{"key":"x"}],
		 "mediaInfo":{"id":496,"status":5,"requests":[{"id":470,"status":5},{"id":12,"status":1}],"serviceUrl":"http://radarr/x"}}`)

		m, err := SeerrGetMedia(context.Background(), seerrClient(srv.URL), "movie", 550)
		if err != nil {
			t.Fatalf("SeerrGetMedia: %v", err)
		}
		if got.path != "/api/v1/movie/550" {
			t.Errorf("path = %q", got.path)
		}
		if m.TMDBID != 550 || m.MediaType != "movie" || m.Title != "Fight Club" || m.RuntimeMinutes != 139 ||
			m.Availability != "AVAILABLE" || len(m.Genres) != 2 || m.Genres[1] != "Thriller" || m.IMDBID != "tt0137523" {
			t.Errorf("media = %+v", m)
		}
		if len(m.Requests) != 2 || m.Requests[0].ID != 470 || m.Requests[1].Status != "PENDING" {
			t.Errorf("requests = %+v", m.Requests)
		}
		if out := mustJSON(t, m); strings.Contains(out, "relatedVideos") || strings.Contains(out, "radarr") || strings.Contains(out, "cast") {
			t.Errorf("projection kept bulk fields: %s", out)
		}
	})
	t.Run("tv", func(t *testing.T) {
		srv, got := fakeService(t, 200, `{"id":1398,"name":"The Sopranos","firstAirDate":"1999-01-10","status":"Ended",
		 "numberOfSeasons":6,"numberOfEpisodes":86,"seasons":[{"seasonNumber":0,"episodeCount":14,"name":"Specials"},{"seasonNumber":1,"episodeCount":13,"name":"Season 1"}]}`)

		m, err := SeerrGetMedia(context.Background(), seerrClient(srv.URL), "tv", 1398)
		if err != nil {
			t.Fatalf("SeerrGetMedia: %v", err)
		}
		if got.path != "/api/v1/tv/1398" {
			t.Errorf("path = %q", got.path)
		}
		if m.Title != "The Sopranos" || m.ReleaseDate != "1999-01-10" || m.NumberOfSeasons != 6 || m.NumberOfEpisodes != 86 {
			t.Errorf("media = %+v", m)
		}
		if len(m.Seasons) != 2 || m.Seasons[1].Number != 1 || m.Seasons[1].Episodes != 13 {
			t.Errorf("seasons = %+v", m.Seasons)
		}
		if m.Availability != "" {
			t.Errorf("availability = %q, want empty for never-requested media", m.Availability)
		}
	})
	t.Run("bad type", func(t *testing.T) {
		if _, err := SeerrGetMedia(context.Background(), seerrClient("http://unused"), "person", 1); err == nil {
			t.Error("unknown media type accepted")
		}
	})
}

// Seerr numbers MediaStatus 6 as BLOCKLISTED and 7 as DELETED, which the
// shipped spec's description gets wrong.
func TestSeerrMediaStatusNames(t *testing.T) {
	for v, want := range map[int]string{1: "UNKNOWN", 2: "PENDING", 3: "PROCESSING", 4: "PARTIALLY_AVAILABLE",
		5: "AVAILABLE", 6: "BLOCKLISTED", 7: "DELETED", 9: "UNKNOWN(9)"} {
		if got := seerrMediaStatus(v); got != want {
			t.Errorf("seerrMediaStatus(%d) = %q, want %q", v, got, want)
		}
	}
}

func TestSeerrListUsersKeepsCountsNotContactDetails(t *testing.T) {
	srv, got := fakeService(t, 200, `{"pageInfo":{"pages":1,"pageSize":20,"results":2,"page":1},"results":[
	 {"id":1,"email":"admin@example.com","displayName":"Arkhaya","requestCount":275,"permissions":2,"userType":3,"jellyfinUserId":"0259","avatar":"/a"},
	 {"id":4,"email":"d@example.com","displayName":"Daniel","requestCount":0,"permissions":32}]}`)

	page, err := SeerrListUsers(context.Background(), seerrClient(srv.URL), 0, 40)
	if err != nil {
		t.Fatalf("SeerrListUsers: %v", err)
	}
	if got.path != "/api/v1/user" {
		t.Errorf("path = %q", got.path)
	}
	q, _ := url.ParseQuery(got.query)
	if q.Get("take") != "20" || q.Get("skip") != "40" {
		t.Errorf("query = %q, want take=20 skip=40", got.query)
	}
	if page.Total != 2 || page.Count != 2 || page.Users[0].DisplayName != "Arkhaya" || page.Users[0].RequestCount != 275 || page.Users[1].ID != 4 {
		t.Errorf("page = %+v", page)
	}
	if out := mustJSON(t, page); strings.Contains(out, "example.com") || strings.Contains(out, "0259") {
		t.Errorf("projection leaked contact details: %s", out)
	}
}

const seerrIssueBody = `{"id":1,"issueType":3,"status":2,"problemSeason":0,"problemEpisode":0,
 "createdAt":"2025-02-09T03:39:52.000Z","updatedAt":"2025-02-09T03:40:17.000Z",
 "comments":[{"id":1,"message":"Not found in bazarr","createdAt":"2025-02-09T03:39:52.000Z","user":{"id":1,"email":"a@example.com","displayName":"Arkhaya"}}],
 "createdBy":{"id":1,"email":"a@example.com","displayName":"Arkhaya"},
 "media":{"id":22,"mediaType":"movie","tmdbId":25868,"status":5,"serviceUrl":"http://radarr/x"}}`

func TestSeerrListIssuesSendsFilterAndOmitsComments(t *testing.T) {
	srv, got := fakeService(t, 200, `{"pageInfo":{"pages":1,"pageSize":2,"results":1,"page":1},"results":[`+seerrIssueBody+`]}`)

	page, err := SeerrListIssues(context.Background(), seerrClient(srv.URL), SeerrIssueQuery{Filter: "resolved", Sort: "modified", Take: 2, RequestedBy: 1})
	if err != nil {
		t.Fatalf("SeerrListIssues: %v", err)
	}
	if got.path != "/api/v1/issue" {
		t.Errorf("path = %q", got.path)
	}
	q, _ := url.ParseQuery(got.query)
	for k, want := range map[string]string{"filter": "resolved", "sort": "modified", "take": "2", "requestedBy": "1"} {
		if q.Get(k) != want {
			t.Errorf("query %s = %q, want %q", k, q.Get(k), want)
		}
	}
	if page.Total != 1 || page.Count != 1 {
		t.Fatalf("page = %+v", page)
	}
	i := page.Issues[0]
	if i.ID != 1 || i.Type != "SUBTITLES" || i.Status != "RESOLVED" || i.TMDBID != 25868 || i.MediaType != "movie" ||
		i.CreatedBy == nil || i.CreatedBy.Name != "Arkhaya" || i.CommentCount != 1 {
		t.Errorf("issue = %+v", i)
	}
	if len(i.Comments) != 0 {
		t.Errorf("list kept comments: %+v", i.Comments)
	}
	if _, err := SeerrListIssues(context.Background(), seerrClient(srv.URL), SeerrIssueQuery{Filter: "closed"}); err == nil {
		t.Error("filter closed accepted")
	}
}

func TestSeerrGetIssueIncludesComments(t *testing.T) {
	srv, got := fakeService(t, 200, seerrIssueBody)

	i, err := SeerrGetIssue(context.Background(), seerrClient(srv.URL), 1)
	if err != nil {
		t.Fatalf("SeerrGetIssue: %v", err)
	}
	if got.path != "/api/v1/issue/1" {
		t.Errorf("path = %q", got.path)
	}
	if len(i.Comments) != 1 || i.Comments[0].Message != "Not found in bazarr" || i.Comments[0].User != "Arkhaya" {
		t.Errorf("comments = %+v", i.Comments)
	}
	if strings.Contains(mustJSON(t, i), "example.com") {
		t.Errorf("email leaked")
	}
}

func TestSeerrCreateRequestBody(t *testing.T) {
	cases := []struct {
		name string
		in   SeerrNewRequest
		want map[string]any
	}{
		{"movie", SeerrNewRequest{MediaType: "movie", MediaID: 550},
			map[string]any{"mediaType": "movie", "mediaId": float64(550)}},
		{"tv seasons for another user", SeerrNewRequest{MediaType: "tv", MediaID: 1398, Seasons: []int{1, 2}, Is4K: true, UserID: 13},
			map[string]any{"mediaType": "tv", "mediaId": float64(1398), "seasons": []any{float64(1), float64(2)}, "is4k": true, "userId": float64(13)}},
		{"tv all seasons", SeerrNewRequest{MediaType: "tv", MediaID: 1398, AllSeasons: true},
			map[string]any{"mediaType": "tv", "mediaId": float64(1398), "seasons": "all"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, got := fakeService(t, 201, seerrRequestBody)

			r, err := SeerrCreateRequest(context.Background(), seerrClient(srv.URL), tc.in)
			if err != nil {
				t.Fatalf("SeerrCreateRequest: %v", err)
			}
			if got.method != "POST" || got.path != "/api/v1/request" {
				t.Errorf("request = %s %s", got.method, got.path)
			}
			if got.header.Get("X-Api-Key") != "seerr-key" {
				t.Errorf("X-Api-Key = %q", got.header.Get("X-Api-Key"))
			}
			var body map[string]any
			if err := json.Unmarshal([]byte(got.body), &body); err != nil {
				t.Fatalf("body %q: %v", got.body, err)
			}
			if mustJSON(t, body) != mustJSON(t, tc.want) {
				t.Errorf("body = %s, want %s", mustJSON(t, body), mustJSON(t, tc.want))
			}
			if r.ID != 470 {
				t.Errorf("request = %+v", r)
			}
		})
	}
}

func TestSeerrCreateRequestRejectsWhatSeerrWouldMisread(t *testing.T) {
	srv, got := fakeService(t, 201, seerrRequestBody)
	for name, in := range map[string]SeerrNewRequest{
		"bad type":           {MediaType: "person", MediaID: 1},
		"missing id":         {MediaType: "movie"},
		"tv without seasons": {MediaType: "tv", MediaID: 1},
		"movie with seasons": {MediaType: "movie", MediaID: 1, Seasons: []int{1}},
		"seasons and all":    {MediaType: "tv", MediaID: 1, Seasons: []int{1}, AllSeasons: true},
	} {
		if _, err := SeerrCreateRequest(context.Background(), seerrClient(srv.URL), in); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if got.path != "" {
		t.Errorf("an invalid request reached the server: %s", got.path)
	}
}

func TestSeerrRequestStatusActions(t *testing.T) {
	for name, tc := range map[string]struct {
		call func(*Client) (SeerrRequest, error)
		path string
	}{
		"approve": {func(c *Client) (SeerrRequest, error) {
			return SeerrSetRequestStatus(context.Background(), c, 470, "approve")
		}, "/api/v1/request/470/approve"},
		"decline": {func(c *Client) (SeerrRequest, error) {
			return SeerrSetRequestStatus(context.Background(), c, 470, "decline")
		}, "/api/v1/request/470/decline"},
		"retry": {func(c *Client) (SeerrRequest, error) { return SeerrRetryRequest(context.Background(), c, 470) }, "/api/v1/request/470/retry"},
	} {
		t.Run(name, func(t *testing.T) {
			srv, got := fakeService(t, 200, seerrRequestBody)

			r, err := tc.call(seerrClient(srv.URL))
			if err != nil {
				t.Fatalf("call: %v", err)
			}
			if got.method != "POST" || got.path != tc.path {
				t.Errorf("request = %s %s, want POST %s", got.method, got.path, tc.path)
			}
			if r.ID != 470 {
				t.Errorf("request = %+v", r)
			}
		})
	}
	if _, err := SeerrSetRequestStatus(context.Background(), seerrClient("http://unused"), 1, "delete"); err == nil {
		t.Error("an action outside approve/decline was accepted")
	}
}

func TestSeerrCommentIssue(t *testing.T) {
	srv, got := fakeService(t, 200, seerrIssueBody)

	i, err := SeerrCommentIssue(context.Background(), seerrClient(srv.URL), 1, "checking now")
	if err != nil {
		t.Fatalf("SeerrCommentIssue: %v", err)
	}
	if got.method != "POST" || got.path != "/api/v1/issue/1/comment" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.body != `{"message":"checking now"}` {
		t.Errorf("body = %s", got.body)
	}
	if i.ID != 1 || len(i.Comments) != 1 {
		t.Errorf("issue = %+v", i)
	}
	if _, err := SeerrCommentIssue(context.Background(), seerrClient(srv.URL), 1, " "); err == nil {
		t.Error("blank comment accepted")
	}
}

func TestSeerrSetIssueStatus(t *testing.T) {
	for _, status := range []string{"open", "resolved"} {
		t.Run(status, func(t *testing.T) {
			srv, got := fakeService(t, 200, seerrIssueBody)

			if _, err := SeerrSetIssueStatus(context.Background(), seerrClient(srv.URL), 1, status); err != nil {
				t.Fatalf("SeerrSetIssueStatus: %v", err)
			}
			if got.method != "POST" || got.path != "/api/v1/issue/1/"+status {
				t.Errorf("request = %s %s", got.method, got.path)
			}
		})
	}
	if _, err := SeerrSetIssueStatus(context.Background(), seerrClient("http://unused"), 1, "closed"); err == nil {
		t.Error("status closed accepted")
	}
}

func TestSeerrDeleteRequest(t *testing.T) {
	srv, got := fakeService(t, 204, ``)

	if err := SeerrDeleteRequest(context.Background(), seerrClient(srv.URL), 470); err != nil {
		t.Fatalf("SeerrDeleteRequest: %v", err)
	}
	if got.method != "DELETE" || got.path != "/api/v1/request/470" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
}

// Every call must surface an upstream failure, naming the service and status,
// rather than returning an empty result.
func TestSeerrCallsReportUpstreamErrors(t *testing.T) {
	ctx := context.Background()
	calls := map[string]func(*Client) error{
		"status":        func(c *Client) error { _, err := SeerrGetStatus(ctx, c); return err },
		"counts":        func(c *Client) error { _, err := SeerrRequestCounts(ctx, c); return err },
		"list requests": func(c *Client) error { _, err := SeerrListRequests(ctx, c, SeerrRequestQuery{}); return err },
		"get request":   func(c *Client) error { _, err := SeerrGetRequest(ctx, c, 1); return err },
		"search":        func(c *Client) error { _, err := SeerrSearch(ctx, c, "x", 1); return err },
		"discover":      func(c *Client) error { _, err := SeerrDiscover(ctx, c, "tv", 1); return err },
		"get media":     func(c *Client) error { _, err := SeerrGetMedia(ctx, c, "movie", 1); return err },
		"list users":    func(c *Client) error { _, err := SeerrListUsers(ctx, c, 0, 0); return err },
		"list issues":   func(c *Client) error { _, err := SeerrListIssues(ctx, c, SeerrIssueQuery{}); return err },
		"get issue":     func(c *Client) error { _, err := SeerrGetIssue(ctx, c, 1); return err },
		"create request": func(c *Client) error {
			_, err := SeerrCreateRequest(ctx, c, SeerrNewRequest{MediaType: "movie", MediaID: 1})
			return err
		},
		"approve":        func(c *Client) error { _, err := SeerrSetRequestStatus(ctx, c, 1, "approve"); return err },
		"retry":          func(c *Client) error { _, err := SeerrRetryRequest(ctx, c, 1); return err },
		"comment":        func(c *Client) error { _, err := SeerrCommentIssue(ctx, c, 1, "x"); return err },
		"issue status":   func(c *Client) error { _, err := SeerrSetIssueStatus(ctx, c, 1, "open"); return err },
		"delete request": func(c *Client) error { return SeerrDeleteRequest(ctx, c, 1) },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			srv, _ := fakeService(t, 403, `{"message":"You do not have permission"}`)

			err := call(seerrClient(srv.URL))
			if err == nil || !strings.Contains(err.Error(), "seerr returned 403") {
				t.Errorf("error = %v, want seerr returned 403", err)
			}
		})
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

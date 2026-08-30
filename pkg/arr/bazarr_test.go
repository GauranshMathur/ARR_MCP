package arr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Go canonicalises header keys, so a ServiceSpec cannot control their case.
// Bazarr was verified to accept X-Api-Key, X-API-KEY and x-api-key alike, so
// the canonical form is correct and no override is needed.
func TestBazarrSendsCanonicalAPIKeyHeader(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":{}}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "bk"})

	if _, err := c.Get(context.Background(), "/system/status"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if v := got.header.Get("X-Api-Key"); v != "bk" {
		t.Errorf("X-Api-Key = %q, want %q", v, "bk")
	}
	if got.path != "/api/system/status" {
		t.Errorf("path = %q, want /api/system/status", got.path)
	}
}

// Most Bazarr endpoints wrap results in a "data" key, unlike the *arr apps.
func TestBazarrListSeriesUnwrapsDataEnvelope(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[
	  {"sonarrSeriesId":16,"title":"Abbott Elementary","monitored":true,
	   "episodeFileCount":93,"episodeMissingCount":0,"profileId":1,
	   "overview":"a very long paragraph that must not be returned","path":"/NAS/Shows/x"}
	],"total":1}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	series, _, err := BazarrListSeries(context.Background(), c, 0, 50)
	if err != nil {
		t.Fatalf("BazarrListSeries returned error: %v", err)
	}
	if got.path != "/api/series" {
		t.Errorf("path = %q, want /api/series", got.path)
	}
	if len(series) != 1 {
		t.Fatalf("series = %d, want 1", len(series))
	}
	if series[0].SonarrSeriesID != 16 || series[0].EpisodeFileCount != 93 {
		t.Errorf("series = %+v, want sonarrSeriesId 16 and 93 files", series[0])
	}
}

func TestBazarrListSeriesPassesPaging(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[]}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if _, _, err := BazarrListSeries(context.Background(), c, 20, 10); err != nil {
		t.Fatalf("BazarrListSeries returned error: %v", err)
	}
	for _, want := range []string{"start=20", "length=10"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
}

// The wanted-episodes list is the question Bazarr exists to answer.
func TestBazarrWantedEpisodesReportsMissingLanguages(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[
	  {"seriesTitle":"Archer (2009)","episode_number":"8x1","episodeTitle":"No Good Deed",
	   "missing_subtitles":[{"name":"English","code2":"en","code3":"eng","forced":false,"hi":false}],
	   "sonarrSeriesId":99,"sonarrEpisodeId":7889}
	],"total":202}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	wanted, total, err := BazarrWantedEpisodes(context.Background(), c, 0, 50)
	if err != nil {
		t.Fatalf("BazarrWantedEpisodes returned error: %v", err)
	}
	if got.path != "/api/episodes/wanted" {
		t.Errorf("path = %q, want /api/episodes/wanted", got.path)
	}
	if total != 202 {
		t.Errorf("total = %d, want 202", total)
	}
	if len(wanted) != 1 {
		t.Fatalf("wanted = %d, want 1", len(wanted))
	}
	w := wanted[0]
	if w.SonarrEpisodeID != 7889 || w.EpisodeNumber != "8x1" {
		t.Errorf("wanted = %+v, want episode 8x1 id 7889", w)
	}
	if len(w.MissingSubtitles) != 1 || w.MissingSubtitles[0].Code2 != "en" {
		t.Errorf("missing subtitles = %+v, want one English entry", w.MissingSubtitles)
	}
}

func TestBazarrWantedMoviesReportsRadarrID(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[
	  {"title":"Shang-Chi","radarrId":192,
	   "missing_subtitles":[{"name":"English","code2":"en","code3":"eng","forced":false,"hi":false}]}
	],"total":6}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	wanted, total, err := BazarrWantedMovies(context.Background(), c, 0, 50)
	if err != nil {
		t.Fatalf("BazarrWantedMovies returned error: %v", err)
	}
	if got.path != "/api/movies/wanted" {
		t.Errorf("path = %q, want /api/movies/wanted", got.path)
	}
	if total != 6 || len(wanted) != 1 || wanted[0].RadarrID != 192 {
		t.Errorf("wanted = %+v total = %d, want radarrId 192 total 6", wanted, total)
	}
}

// /badges is NOT wrapped in "data" -- unlike almost every other endpoint.
func TestBazarrBadgesIsNotDataWrapped(t *testing.T) {
	srv, got := fakeService(t, 200,
		`{"episodes":202,"movies":6,"providers":1,"status":0,"sonarr_signalr":"LIVE","radarr_signalr":"LIVE"}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	b, err := BazarrBadges(context.Background(), c)
	if err != nil {
		t.Fatalf("BazarrBadges returned error: %v", err)
	}
	if got.path != "/api/badges" {
		t.Errorf("path = %q, want /api/badges", got.path)
	}
	if b.Episodes != 202 || b.Movies != 6 {
		t.Errorf("badges = %+v, want 202 episodes and 6 movies", b)
	}
	if b.SonarrSignalR != "LIVE" {
		t.Errorf("sonarr signalr = %q, want LIVE", b.SonarrSignalR)
	}
}

// /system/languages returns a bare array, also unwrapped.
func TestBazarrLanguagesIsBareArray(t *testing.T) {
	srv, got := fakeService(t, 200,
		`[{"name":"English","code2":"en","code3":"eng","enabled":true},
		  {"name":"Abkhazian","code2":"ab","code3":"abk","enabled":false}]`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	langs, err := BazarrLanguages(context.Background(), c, true)
	if err != nil {
		t.Fatalf("BazarrLanguages returned error: %v", err)
	}
	if got.path != "/api/system/languages" {
		t.Errorf("path = %q, want /api/system/languages", got.path)
	}
	// enabledOnly must filter, since Bazarr returns every ISO language.
	if len(langs) != 1 || langs[0].Code2 != "en" {
		t.Errorf("languages = %+v, want only the enabled English entry", langs)
	}
}

func TestBazarrProvidersReportsStatus(t *testing.T) {
	srv, _ := fakeService(t, 200, `{"data":[{"name":"opensubtitlescom","status":"Good","retry":"-"}]}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	providers, err := BazarrProviders(context.Background(), c)
	if err != nil {
		t.Fatalf("BazarrProviders returned error: %v", err)
	}
	if len(providers) != 1 || providers[0].Status != "Good" {
		t.Errorf("providers = %+v, want one Good provider", providers)
	}
}

func TestBazarrSearchEpisodeSubtitlesPatchesWithQueryParams(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	err := BazarrSearchEpisodeSubtitles(context.Background(), c, 99, 7889, "en", false, false)
	if err != nil {
		t.Fatalf("BazarrSearchEpisodeSubtitles returned error: %v", err)
	}
	if got.method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", got.method)
	}
	if got.path != "/api/episodes/subtitles" {
		t.Errorf("path = %q, want /api/episodes/subtitles", got.path)
	}
	for _, want := range []string{"seriesid=99", "episodeid=7889", "language=en", "forced=false", "hi=false"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
}

func TestBazarrSearchMovieSubtitlesPatchesWithQueryParams(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrSearchMovieSubtitles(context.Background(), c, 192, "en", false, true); err != nil {
		t.Fatalf("BazarrSearchMovieSubtitles returned error: %v", err)
	}
	if got.method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", got.method)
	}
	for _, want := range []string{"radarrid=192", "language=en", "hi=true"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
}

// Bazarr's health entries are {object, issue}, not the {source, type, message}
// the *arr apps use. Decoding into the *arr shape yields blank messages.
func TestBazarrHealthDecodesObjectAndIssue(t *testing.T) {
	srv, got := fakeService(t, 200,
		`{"data":[{"object":"/NAS/Shows","issue":"Path does not exist"}]}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	issues, err := BazarrHealth(context.Background(), c)
	if err != nil {
		t.Fatalf("BazarrHealth returned error: %v", err)
	}
	if got.path != "/api/system/health" {
		t.Errorf("path = %q, want /api/system/health", got.path)
	}
	if len(issues) != 1 {
		t.Fatalf("issues = %d, want 1", len(issues))
	}
	if issues[0].Object != "/NAS/Shows" || issues[0].Issue != "Path does not exist" {
		t.Errorf("issue = %+v, want the object and issue populated", issues[0])
	}
}

// A page of results is meaningless without the library total: the model cannot
// otherwise tell "50 series exist" from "50 of 500".
func TestBazarrListSeriesReportsLibraryTotal(t *testing.T) {
	srv, _ := fakeService(t, 200, `{"data":[{"sonarrSeriesId":1,"title":"A"}],"total":500}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	series, total, err := BazarrListSeries(context.Background(), c, 0, 50)
	if err != nil {
		t.Fatalf("BazarrListSeries returned error: %v", err)
	}
	if total != 500 {
		t.Errorf("total = %d, want 500", total)
	}
	if len(series) != 1 {
		t.Errorf("series = %d, want 1", len(series))
	}
}

func TestBazarrListMoviesReportsLibraryTotal(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[{"radarrId":7,"title":"B"}],"total":42}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	movies, total, err := BazarrListMovies(context.Background(), c, 0, 50)
	if err != nil {
		t.Fatalf("BazarrListMovies returned error: %v", err)
	}
	if got.path != "/api/movies" {
		t.Errorf("path = %q, want /api/movies", got.path)
	}
	if total != 42 || len(movies) != 1 || movies[0].RadarrID != 7 {
		t.Errorf("movies = %+v total = %d, want radarrId 7 total 42", movies, total)
	}
}

// Subtitle deletion needs a file path, and this is the only tool that can
// produce one. Embedded tracks have a null path and cannot be deleted.
func TestBazarrListEpisodeSubtitlesReturnsPaths(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[
	  {"sonarrSeriesId":99,"sonarrEpisodeId":7946,"title":"Into the Cold","season":14,"episode":9,
	   "subtitles":[
	     {"name":"English","code2":"en","path":"/NAS/Shows/x.en.srt","forced":false,"hi":false},
	     {"name":"English","code2":"en","path":null,"forced":false,"hi":true,"embedded_track_id":3}],
	   "missing_subtitles":[{"name":"Spanish","code2":"es"}]}
	]}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	eps, err := BazarrListEpisodeSubtitles(context.Background(), c, 99)
	if err != nil {
		t.Fatalf("BazarrListEpisodeSubtitles returned error: %v", err)
	}
	if got.path != "/api/episodes" {
		t.Errorf("path = %q, want /api/episodes", got.path)
	}
	if !contains(got.query, "seriesid") {
		t.Errorf("query = %q, want a seriesid parameter", got.query)
	}
	if len(eps) != 1 {
		t.Fatalf("episodes = %d, want 1", len(eps))
	}
	if len(eps[0].Subtitles) != 2 {
		t.Fatalf("subtitles = %d, want 2", len(eps[0].Subtitles))
	}
	if eps[0].Subtitles[0].Path != "/NAS/Shows/x.en.srt" {
		t.Errorf("first subtitle path = %q, want the external file path", eps[0].Subtitles[0].Path)
	}
	if eps[0].Subtitles[1].Path != "" {
		t.Errorf("embedded track path = %q, want empty", eps[0].Subtitles[1].Path)
	}
}

func TestBazarrDeleteEpisodeSubtitleSendsPathAndMethod(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	err := BazarrDeleteEpisodeSubtitle(context.Background(), c, 99, 7889, "en", "/sub/a.srt", false, false)
	if err != nil {
		t.Fatalf("BazarrDeleteEpisodeSubtitle returned error: %v", err)
	}
	if got.method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", got.method)
	}
	if got.path != "/api/episodes/subtitles" {
		t.Errorf("path = %q, want /api/episodes/subtitles", got.path)
	}
	for _, want := range []string{"seriesid=99", "episodeid=7889", "language=en", "path=%2Fsub%2Fa.srt"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
}

// Deleting with an empty path would ask Bazarr to remove "" and then report
// success, so the client refuses before making the request.
func TestBazarrDeleteEpisodeSubtitleRejectsEmptyPath(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	err := BazarrDeleteEpisodeSubtitle(context.Background(), c, 99, 7889, "en", "", false, false)
	if err == nil {
		t.Fatal("expected an error when the subtitle path is empty, got nil")
	}
	if got.path != "" {
		t.Errorf("a request was sent despite the empty path: %q", got.path)
	}
}

func TestBazarrDeleteMovieSubtitleRejectsEmptyPath(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrDeleteMovieSubtitle(context.Background(), c, 192, "en", "", false, false); err == nil {
		t.Fatal("expected an error when the subtitle path is empty, got nil")
	}
	if got.path != "" {
		t.Errorf("a request was sent despite the empty path: %q", got.path)
	}
}

func TestBazarrDeleteMovieSubtitleSendsPathAndMethod(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrDeleteMovieSubtitle(context.Background(), c, 192, "en", "/sub/b.srt", false, true); err != nil {
		t.Fatalf("BazarrDeleteMovieSubtitle returned error: %v", err)
	}
	if got.method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", got.method)
	}
	for _, want := range []string{"radarrid=192", "path=%2Fsub%2Fb.srt", "hi=true"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
}

// Provider search blocks while Bazarr queries every provider, which routinely
// exceeds the default read timeout.
func TestSubtitleSearchUsesLongerTimeoutThanDefault(t *testing.T) {
	c := NewClient("http://x", BazarrSpec, Credentials{APIKey: "k"})
	slow := c.WithTimeout(subtitleSearchTimeout)

	if slow.http.Timeout <= c.http.Timeout {
		t.Errorf("search timeout %v is not longer than the default %v", slow.http.Timeout, c.http.Timeout)
	}
	if c.http.Timeout != defaultTimeout {
		t.Errorf("WithTimeout mutated the original client: %v", c.http.Timeout)
	}
}

// fakeBazarr records every request line, including the query string. The
// shared fakeService keeps only the last query, which cannot verify a call
// that fans out into one request per id.
func fakeBazarr(t *testing.T, status int, body string) (*httptest.Server, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

// /system/languages/profiles returns a bare array, like /system/languages and
// /badges, and renders the per-item booleans as the Python strings "True" and
// "False" rather than JSON booleans.
func TestBazarrLanguageProfilesIsBareArrayWithStringBooleans(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"profileId":1,"name":"eng","cutoff":null,
	  "items":[{"id":1,"language":"en","audio_exclude":"False","hi":"True","forced":"False"}],
	  "mustContain":[],"mustNotContain":[],"originalFormat":0,"tag":null}]`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	profiles, err := BazarrLanguageProfiles(context.Background(), c)
	if err != nil {
		t.Fatalf("BazarrLanguageProfiles returned error: %v", err)
	}
	if got.path != "/api/system/languages/profiles" {
		t.Errorf("path = %q, want /api/system/languages/profiles", got.path)
	}
	if len(profiles) != 1 || profiles[0].ProfileID != 1 || profiles[0].Name != "eng" {
		t.Fatalf("profiles = %+v, want one profile 1 named eng", profiles)
	}
	if len(profiles[0].Items) != 1 {
		t.Fatalf("items = %d, want 1", len(profiles[0].Items))
	}
	item := profiles[0].Items[0]
	if item.Language != "en" || !item.HI || item.Forced {
		t.Errorf("item = %+v, want language en with hi true and forced false", item)
	}
}

// Bazarr's POST /series takes parallel seriesid/profileid arrays, and the
// parameter names carry no "[]" suffix even though the GET variant's do.
// Query cannot express a repeated key, and the arrays are positional, so the
// client sends one request per id.
func TestBazarrSetSeriesProfilePostsOnePairPerID(t *testing.T) {
	srv, seen := fakeBazarr(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrSetSeriesProfile(context.Background(), c, []int{99, 16}, 2); err != nil {
		t.Fatalf("BazarrSetSeriesProfile returned error: %v", err)
	}
	if len(*seen) != 2 {
		t.Fatalf("requests = %v, want one per series id", *seen)
	}
	for i, want := range []string{
		"POST /api/series?profileid=2&seriesid=99",
		"POST /api/series?profileid=2&seriesid=16",
	} {
		if (*seen)[i] != want {
			t.Errorf("request %d = %q, want %q", i, (*seen)[i], want)
		}
	}
}

// profileid accepts the literal "none" to unassign a profile, which is the
// only way to stop Bazarr fetching subtitles for a series.
func TestBazarrSetSeriesProfileSendsNoneForZero(t *testing.T) {
	srv, seen := fakeBazarr(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrSetSeriesProfile(context.Background(), c, []int{99}, 0); err != nil {
		t.Fatalf("BazarrSetSeriesProfile returned error: %v", err)
	}
	if len(*seen) != 1 || !contains((*seen)[0], "profileid=none") {
		t.Errorf("requests = %v, want profileid=none", *seen)
	}
}

func TestBazarrSetProfileRejectsEmptyIDList(t *testing.T) {
	srv, seen := fakeBazarr(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrSetSeriesProfile(context.Background(), c, nil, 1); err == nil {
		t.Error("expected an error for an empty series id list, got nil")
	}
	if err := BazarrSetMovieProfile(context.Background(), c, nil, 1); err == nil {
		t.Error("expected an error for an empty movie id list, got nil")
	}
	if len(*seen) != 0 {
		t.Errorf("requests sent for an empty id list: %v", *seen)
	}
}

func TestBazarrSetMovieProfileUsesRadarrID(t *testing.T) {
	srv, seen := fakeBazarr(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrSetMovieProfile(context.Background(), c, []int{192}, 3); err != nil {
		t.Fatalf("BazarrSetMovieProfile returned error: %v", err)
	}
	if len(*seen) != 1 || (*seen)[0] != "POST /api/movies?profileid=3&radarrid=192" {
		t.Errorf("requests = %v, want a single POST with radarrid and profileid", *seen)
	}
}

func TestBazarrSeriesActionPatchesActionByQuery(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrSeriesAction(context.Background(), c, 99, "search-missing"); err != nil {
		t.Fatalf("BazarrSeriesAction returned error: %v", err)
	}
	if got.method != http.MethodPatch || got.path != "/api/series" {
		t.Errorf("%s %s, want PATCH /api/series", got.method, got.path)
	}
	for _, want := range []string{"seriesid=99", "action=search-missing"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
}

func TestBazarrMovieActionPatchesActionByQuery(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrMovieAction(context.Background(), c, 192, "scan-disk"); err != nil {
		t.Fatalf("BazarrMovieAction returned error: %v", err)
	}
	if got.method != http.MethodPatch || got.path != "/api/movies" {
		t.Errorf("%s %s, want PATCH /api/movies", got.method, got.path)
	}
	for _, want := range []string{"radarrid=192", "action=scan-disk"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
}

// The manual search is the only call that returns the opaque subtitle token a
// download needs. Its booleans arrive as the strings "True"/"False".
func TestBazarrManualSearchEpisodeTrimsCandidates(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[
	  {"provider":"opensubtitlescom","subtitle":"fdded9ba-0761","language":"en","score":94,
	   "orig_score":340,"score_without_hash":340,"forced":"False","hearing_impaired":"True",
	   "original_format":"False","uploader":"someone","url":"https://example/1",
	   "matches":["series","season","episode"],"dont_matches":["hash"],
	   "release_info":["Archer.S08E01.WEBDL-1080p"]}
	]}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	found, err := BazarrManualSearchEpisode(context.Background(), c, 7889)
	if err != nil {
		t.Fatalf("BazarrManualSearchEpisode returned error: %v", err)
	}
	if got.path != "/api/providers/episodes" {
		t.Errorf("path = %q, want /api/providers/episodes", got.path)
	}
	if !contains(got.query, "episodeid=7889") {
		t.Errorf("query = %q, want episodeid=7889", got.query)
	}
	if len(found) != 1 {
		t.Fatalf("candidates = %d, want 1", len(found))
	}
	cand := found[0]
	if cand.Subtitle != "fdded9ba-0761" || cand.Provider != "opensubtitlescom" {
		t.Errorf("candidate = %+v, want the provider and subtitle token preserved", cand)
	}
	// "True"/"False" strings must become real booleans, or every candidate
	// looks hearing-impaired to a model reading a non-empty string.
	if !cand.HearingImpaired || cand.Forced || cand.OriginalFormat {
		t.Errorf("candidate flags = %+v, want hi true, forced false, original format false", cand)
	}
	if cand.Score != 94 || len(cand.ReleaseInfo) != 1 {
		t.Errorf("candidate = %+v, want score 94 and one release", cand)
	}
}

func TestBazarrManualSearchMovieUsesRadarrID(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[]}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if _, err := BazarrManualSearchMovie(context.Background(), c, 192); err != nil {
		t.Fatalf("BazarrManualSearchMovie returned error: %v", err)
	}
	if got.path != "/api/providers/movies" || !contains(got.query, "radarrid=192") {
		t.Errorf("%s?%s, want /api/providers/movies with radarrid=192", got.path, got.query)
	}
}

// A manual search queries every provider and routinely outlives the default
// read timeout, so it must build its own long-timeout client. A base client
// with an impossibly short timeout still succeeds when it does.
func TestBazarrManualSearchOverridesTheClientTimeout(t *testing.T) {
	srv, _ := fakeService(t, 200, `{"data":[]}`)
	impatient := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"}).WithTimeout(time.Nanosecond)

	if _, err := impatient.Get(context.Background(), "/badges"); err == nil {
		t.Fatal("a one-nanosecond timeout completed a request; the test proves nothing")
	}
	if _, err := BazarrManualSearchEpisode(context.Background(), impatient, 7889); err != nil {
		t.Errorf("manual search used the caller's short timeout: %v", err)
	}
	if _, err := BazarrManualSearchMovie(context.Background(), impatient, 192); err != nil {
		t.Errorf("manual search used the caller's short timeout: %v", err)
	}
}

// Bazarr calls .capitalize() on these three, so the lowercase Go rendering is
// accepted -- unlike PATCH /subtitles, which compares to "True" exactly.
func TestBazarrDownloadEpisodeSubtitleSendsProviderAndToken(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	err := BazarrDownloadEpisodeSubtitle(context.Background(), c, 99, 7889,
		"opensubtitlescom", "fdded9ba-0761", true, false, false)
	if err != nil {
		t.Fatalf("BazarrDownloadEpisodeSubtitle returned error: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/providers/episodes" {
		t.Errorf("%s %s, want POST /api/providers/episodes", got.method, got.path)
	}
	if got.body != "" {
		t.Errorf("body = %q, want an empty body; Bazarr reads these from the query", got.body)
	}
	for _, want := range []string{
		"seriesid=99", "episodeid=7889", "provider=opensubtitlescom",
		"subtitle=fdded9ba-0761", "hi=true", "forced=false", "original_format=false",
	} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
}

func TestBazarrDownloadMovieSubtitleSendsProviderAndToken(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	err := BazarrDownloadMovieSubtitle(context.Background(), c, 192, "subsource", "0fa8f8da", false, true, true)
	if err != nil {
		t.Fatalf("BazarrDownloadMovieSubtitle returned error: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/providers/movies" {
		t.Errorf("%s %s, want POST /api/providers/movies", got.method, got.path)
	}
	for _, want := range []string{"radarrid=192", "provider=subsource", "subtitle=0fa8f8da",
		"hi=false", "forced=true", "original_format=true"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
}

func TestBazarrDownloadRejectsMissingToken(t *testing.T) {
	srv, seen := fakeBazarr(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrDownloadEpisodeSubtitle(context.Background(), c, 99, 7889, "", "", false, false, false); err == nil {
		t.Error("expected an error without a provider and subtitle token, got nil")
	}
	if err := BazarrDownloadMovieSubtitle(context.Background(), c, 192, "subsource", "", false, false, false); err == nil {
		t.Error("expected an error without a subtitle token, got nil")
	}
	if len(*seen) != 0 {
		t.Errorf("requests sent without a token: %v", *seen)
	}
}

func TestBazarrEpisodeHistoryReportsTotalAndFiltersByEpisode(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[
	  {"seriesTitle":"Video Game High School","episode_number":"1x7","episodeTitle":"Sign Up to Sign Out",
	   "sonarrSeriesId":65,"sonarrEpisodeId":5635,"action":1,"provider":"gestdown",
	   "subs_id":"sp_019c82e3_ep_7","score":"99.44%","upgradable":false,"blacklisted":false,
	   "description":"English subtitles downloaded from gestdown with a score of 99.44%.",
	   "language":{"name":"English","code2":"en","code3":"eng","forced":false,"hi":false},
	   "subtitles_path":"/NAS/Shows/x.en.srt","timestamp":"last month","parsed_timestamp":"07/10/26 05:33:31",
	   "matches":["episode","series"],"dont_matches":["hash"],"tags":["1-gauransh"],"monitored":true}
	],"total":67}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	records, total, err := BazarrEpisodeHistory(context.Background(), c, 0, 20, 5635)
	if err != nil {
		t.Fatalf("BazarrEpisodeHistory returned error: %v", err)
	}
	if got.path != "/api/episodes/history" {
		t.Errorf("path = %q, want /api/episodes/history", got.path)
	}
	for _, want := range []string{"start=0", "length=20", "episodeid=5635"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
	if total != 67 || len(records) != 1 {
		t.Fatalf("records = %d total = %d, want 1 record of 67", len(records), total)
	}
	r := records[0]
	if r.SubsID != "sp_019c82e3_ep_7" || r.Provider != "gestdown" {
		t.Errorf("record = %+v, want the provider and subs_id needed to blacklist it", r)
	}
	if r.SubtitlesPath != "/NAS/Shows/x.en.srt" || r.Language.Code2 != "en" {
		t.Errorf("record = %+v, want the subtitle path and language", r)
	}
}

// episodeid is a filter, not a requirement: zero must leave it off entirely
// rather than asking Bazarr for episode 0.
func TestBazarrHistoryOmitsZeroIDFilter(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[],"total":0}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if _, _, err := BazarrEpisodeHistory(context.Background(), c, 0, 20, 0); err != nil {
		t.Fatalf("BazarrEpisodeHistory returned error: %v", err)
	}
	if contains(got.query, "episodeid") {
		t.Errorf("query = %q, want no episodeid filter", got.query)
	}
	if _, _, err := BazarrMovieHistory(context.Background(), c, 0, 20, 0); err != nil {
		t.Fatalf("BazarrMovieHistory returned error: %v", err)
	}
	if got.path != "/api/movies/history" {
		t.Errorf("path = %q, want /api/movies/history", got.path)
	}
	if contains(got.query, "radarrid") {
		t.Errorf("query = %q, want no radarrid filter", got.query)
	}
}

func TestBazarrMovieHistoryFiltersByRadarrID(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[
	  {"title":"Bhavesh Joshi Superhero","radarrId":193,"action":1,"provider":"supersubtitles",
	   "subs_id":"1691655025","score":"98.89%","subtitles_path":"/NAS/Movies/x.en.srt",
	   "language":{"name":"English","code2":"en","code3":"eng","forced":false,"hi":false},
	   "timestamp":"last month","parsed_timestamp":"07/12/26 07:47:11","blacklisted":false}
	],"total":1}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	records, total, err := BazarrMovieHistory(context.Background(), c, 0, 20, 193)
	if err != nil {
		t.Fatalf("BazarrMovieHistory returned error: %v", err)
	}
	if !contains(got.query, "radarrid=193") {
		t.Errorf("query = %q, want radarrid=193", got.query)
	}
	if total != 1 || len(records) != 1 || records[0].RadarrID != 193 {
		t.Errorf("records = %+v total = %d, want one record for radarrId 193", records, total)
	}
}

// GET /movies/blacklist raises a 500 upstream whenever length is greater than
// zero -- it calls .limit() on an already-executed result -- so the client
// never sends paging and slices the answer itself. /episodes/blacklist is
// paged the same way for one code path and one behaviour.
func TestBazarrBlacklistPagesLocallyWithoutLengthParameter(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[
	  {"seriesTitle":"A","episode_number":"1x1","episodeTitle":"One","sonarrSeriesId":1,
	   "provider":"p1","subs_id":"s1","timestamp":"today","parsed_timestamp":"1",
	   "language":{"name":"English","code2":"en"}},
	  {"seriesTitle":"B","episode_number":"1x2","episodeTitle":"Two","sonarrSeriesId":2,
	   "provider":"p2","subs_id":"s2","timestamp":"today","parsed_timestamp":"2",
	   "language":{"name":"English","code2":"en"}},
	  {"seriesTitle":"C","episode_number":"1x3","episodeTitle":"Three","sonarrSeriesId":3,
	   "provider":"p3","subs_id":"s3","timestamp":"today","parsed_timestamp":"3",
	   "language":{"name":"English","code2":"en"}}
	]}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	items, err := BazarrBlacklist(context.Background(), c, "episodes", 1, 1)
	if err != nil {
		t.Fatalf("BazarrBlacklist returned error: %v", err)
	}
	if got.path != "/api/episodes/blacklist" {
		t.Errorf("path = %q, want /api/episodes/blacklist", got.path)
	}
	if got.query != "" {
		t.Errorf("query = %q, want none; length>0 makes /movies/blacklist answer 500", got.query)
	}
	if len(items) != 1 || items[0].SubsID != "s2" {
		t.Fatalf("items = %+v, want only the second entry", items)
	}
}

func TestBazarrBlacklistReadsMovieEntries(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[
	  {"title":"Shang-Chi","radarrId":192,"provider":"p","subs_id":"s","timestamp":"today",
	   "parsed_timestamp":"1","language":{"name":"English","code2":"en"}}]}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	items, err := BazarrBlacklist(context.Background(), c, "movies", 0, 0)
	if err != nil {
		t.Fatalf("BazarrBlacklist returned error: %v", err)
	}
	if got.path != "/api/movies/blacklist" {
		t.Errorf("path = %q, want /api/movies/blacklist", got.path)
	}
	if len(items) != 1 || items[0].RadarrID != 192 || items[0].Title != "Shang-Chi" {
		t.Errorf("items = %+v, want the movie entry", items)
	}
}

// A model that guesses "episode" must be told the two valid values rather than
// getting a 404 from a path it cannot see.
func TestBazarrBlacklistRejectsUnknownKind(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[]}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	_, err := BazarrBlacklist(context.Background(), c, "episode", 0, 0)
	if err == nil {
		t.Fatal("expected an error for an unknown kind, got nil")
	}
	for _, want := range []string{"episodes", "movies"} {
		if !contains(err.Error(), want) {
			t.Errorf("error %q does not name the valid kind %q", err, want)
		}
	}
	if got.path != "" {
		t.Errorf("a request was sent for an unknown kind: %q", got.path)
	}
}

func TestBazarrBlacklistSubtitlePostsEpisodeFields(t *testing.T) {
	srv, got := fakeService(t, 200, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	err := BazarrBlacklistEpisodeSubtitle(context.Background(), c, 65, 5635,
		"gestdown", "sp_019c82e3_ep_7", "en", "/NAS/Shows/x.en.srt")
	if err != nil {
		t.Fatalf("BazarrBlacklistEpisodeSubtitle returned error: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/episodes/blacklist" {
		t.Errorf("%s %s, want POST /api/episodes/blacklist", got.method, got.path)
	}
	for _, want := range []string{"seriesid=65", "episodeid=5635", "provider=gestdown",
		"subs_id=sp_019c82e3_ep_7", "language=en", "subtitles_path=%2FNAS%2FShows%2Fx.en.srt"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
}

func TestBazarrBlacklistSubtitlePostsMovieFields(t *testing.T) {
	srv, got := fakeService(t, 200, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	err := BazarrBlacklistMovieSubtitle(context.Background(), c, 193,
		"supersubtitles", "1691655025", "en", "/NAS/Movies/x.en.srt")
	if err != nil {
		t.Fatalf("BazarrBlacklistMovieSubtitle returned error: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/movies/blacklist" {
		t.Errorf("%s %s, want POST /api/movies/blacklist", got.method, got.path)
	}
	for _, want := range []string{"radarrid=193", "provider=supersubtitles", "subs_id=1691655025"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
}

// Bazarr compares this one to the lowercase "true" literally, so Go's default
// rendering is required here and a capitalised "True" would silently be read
// as a single-item delete with no provider.
func TestBazarrDeleteBlacklistAllSendsLowercaseTrue(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrDeleteBlacklistItem(context.Background(), c, "episodes", "", "", true); err != nil {
		t.Fatalf("BazarrDeleteBlacklistItem returned error: %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/api/episodes/blacklist" {
		t.Errorf("%s %s, want DELETE /api/episodes/blacklist", got.method, got.path)
	}
	if got.query != "all=true" {
		t.Errorf("query = %q, want exactly all=true", got.query)
	}
}

func TestBazarrDeleteBlacklistItemSendsProviderAndSubsID(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	err := BazarrDeleteBlacklistItem(context.Background(), c, "movies", "supersubtitles", "1691655025", false)
	if err != nil {
		t.Fatalf("BazarrDeleteBlacklistItem returned error: %v", err)
	}
	if got.path != "/api/movies/blacklist" {
		t.Errorf("path = %q, want /api/movies/blacklist", got.path)
	}
	for _, want := range []string{"provider=supersubtitles", "subs_id=1691655025"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
	if contains(got.query, "all=") {
		t.Errorf("query = %q, want no all parameter for a single delete", got.query)
	}
}

// Deleting one entry needs both halves of its identity; without them Bazarr
// would delete the rows matching NULL, which is nothing, and report success.
func TestBazarrDeleteBlacklistItemRequiresIdentity(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrDeleteBlacklistItem(context.Background(), c, "episodes", "", "", false); err == nil {
		t.Error("expected an error without provider, subs_id or all, got nil")
	}
	if got.path != "" {
		t.Errorf("a request was sent without an identity: %q", got.path)
	}
}

func TestBazarrResetProvidersPostsResetAction(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrResetProviders(context.Background(), c); err != nil {
		t.Fatalf("BazarrResetProviders returned error: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/providers" {
		t.Errorf("%s %s, want POST /api/providers", got.method, got.path)
	}
	if got.query != "action=reset" {
		t.Errorf("query = %q, want action=reset", got.query)
	}
}

func TestBazarrTasksDecodesJobIdentifiers(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":[
	  {"interval":"every 15 minutes","job_id":"update_series","job_running":false,
	   "name":"Sync with Sonarr","next_run_in":"in a minute","next_run_time":"in a minute"}]}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	tasks, err := BazarrTasks(context.Background(), c)
	if err != nil {
		t.Fatalf("BazarrTasks returned error: %v", err)
	}
	if got.path != "/api/system/tasks" {
		t.Errorf("path = %q, want /api/system/tasks", got.path)
	}
	if len(tasks) != 1 || tasks[0].JobID != "update_series" || tasks[0].Name != "Sync with Sonarr" {
		t.Errorf("tasks = %+v, want the update_series job", tasks)
	}
}

func TestBazarrRunTaskPostsTaskID(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrRunTask(context.Background(), c, "update_series"); err != nil {
		t.Fatalf("BazarrRunTask returned error: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/api/system/tasks" {
		t.Errorf("%s %s, want POST /api/system/tasks", got.method, got.path)
	}
	if got.query != "taskid=update_series" {
		t.Errorf("query = %q, want taskid=update_series", got.query)
	}
}

func TestBazarrRunTaskRequiresTaskID(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if err := BazarrRunTask(context.Background(), c, ""); err == nil {
		t.Error("expected an error for an empty task id, got nil")
	}
	if got.path != "" {
		t.Errorf("a request was sent without a task id: %q", got.path)
	}
}

// PATCH /subtitles is the one endpoint that compares its booleans to the
// Python literal "True" instead of calling .capitalize() first. Sending Go's
// lowercase "true" there means false, silently.
func TestBazarrModifySubtitleSendsCapitalisedBooleans(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	err := BazarrModifySubtitle(context.Background(), c, SubtitleMod{
		Action: "remove_HI", Language: "en", Path: "/sub/a.srt", MediaType: "episode",
		MediaID: 5635, HI: true, Forced: false,
	})
	if err != nil {
		t.Fatalf("BazarrModifySubtitle returned error: %v", err)
	}
	if got.method != http.MethodPatch || got.path != "/api/subtitles" {
		t.Errorf("%s %s, want PATCH /api/subtitles", got.method, got.path)
	}
	for _, want := range []string{"action=remove_HI", "language=en", "path=%2Fsub%2Fa.srt",
		"type=episode", "id=5635", "hi=True", "forced=False"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
	for _, unwanted := range []string{"hi=true", "forced=false", "reference=", "gss="} {
		if contains(got.query, unwanted) {
			t.Errorf("query = %q, must not contain %q", got.query, unwanted)
		}
	}
}

// The sync-only options are optional upstream and must stay absent when unset,
// so a plain sync does not pin the offset to an empty string.
func TestBazarrModifySubtitleSendsSyncOptionsOnlyWhenSet(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	err := BazarrModifySubtitle(context.Background(), c, SubtitleMod{
		Action: "sync", Language: "en", Path: "/sub/a.srt", MediaType: "movie", MediaID: 193,
		Reference: "a:0", MaxOffsetSeconds: 60, NoFixFramerate: true, GSS: true,
	})
	if err != nil {
		t.Fatalf("BazarrModifySubtitle returned error: %v", err)
	}
	for _, want := range []string{"reference=a%3A0", "max_offset_seconds=60",
		"no_fix_framerate=True", "gss=True", "type=movie", "id=193"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
}

func TestBazarrModifySubtitleRejectsUnknownMediaType(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	err := BazarrModifySubtitle(context.Background(), c, SubtitleMod{
		Action: "sync", Language: "en", Path: "/sub/a.srt", MediaType: "series", MediaID: 1,
	})
	if err == nil {
		t.Fatal("expected an error for an unknown media type, got nil")
	}
	for _, want := range []string{"episode", "movie"} {
		if !contains(err.Error(), want) {
			t.Errorf("error %q does not name the valid type %q", err, want)
		}
	}
	if got.path != "" {
		t.Errorf("a request was sent for an unknown media type: %q", got.path)
	}
}

// GET /subtitles answers a data-wrapped object, not a list, and is the only
// place the embedded track numbers a sync reference needs are visible.
func TestBazarrSubtitleTracksUnwrapsDataObject(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":{
	  "audio_tracks":[{"stream":"a:0","name":"","language":"English"}],
	  "embedded_subtitles_tracks":[{"stream":"s:0","name":"","language":"English",
	    "forced":false,"hearing_impaired":true}],
	  "external_subtitles_tracks":[{"name":"English","path":"/sub/a.srt","language":"English",
	    "forced":false,"hearing_impaired":false}]}}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	tracks, err := BazarrSubtitleTracks(context.Background(), c, "/sub/a.srt", 5635, 0)
	if err != nil {
		t.Fatalf("BazarrSubtitleTracks returned error: %v", err)
	}
	if got.path != "/api/subtitles" {
		t.Errorf("path = %q, want /api/subtitles", got.path)
	}
	for _, want := range []string{"subtitlesPath=%2Fsub%2Fa.srt", "sonarrEpisodeId=5635"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
	if contains(got.query, "radarrMovieId") {
		t.Errorf("query = %q, want no radarrMovieId for an episode", got.query)
	}
	if len(tracks.AudioTracks) != 1 || tracks.AudioTracks[0].Stream != "a:0" {
		t.Errorf("audio tracks = %+v, want one a:0 track", tracks.AudioTracks)
	}
	if len(tracks.EmbeddedSubtitles) != 1 || !tracks.EmbeddedSubtitles[0].HearingImpaired {
		t.Errorf("embedded tracks = %+v, want one hearing-impaired track", tracks.EmbeddedSubtitles)
	}
	if len(tracks.ExternalSubtitles) != 1 || tracks.ExternalSubtitles[0].Path != "/sub/a.srt" {
		t.Errorf("external tracks = %+v, want the external file", tracks.ExternalSubtitles)
	}
}

func TestBazarrSubtitleTracksRequiresAPath(t *testing.T) {
	srv, got := fakeService(t, 200, `{"data":{}}`)
	c := NewClient(srv.URL, BazarrSpec, Credentials{APIKey: "k"})

	if _, err := BazarrSubtitleTracks(context.Background(), c, "", 5635, 0); err == nil {
		t.Error("expected an error without a subtitle path, got nil")
	}
	if got.path != "" {
		t.Errorf("a request was sent without a subtitle path: %q", got.path)
	}
}

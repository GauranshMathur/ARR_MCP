package arr

import (
	"context"
	"net/http"
	"testing"
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

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

func TestSonarrEditSeriesPutsToTheEditorEndpoint(t *testing.T) {
	srv, got := fakeService(t, 202, `[{"id":5,"title":"New Girl","monitored":false}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	monitored := false
	series, err := SonarrEditSeries(context.Background(), c, SeriesEditRequest{
		SeriesIDs: []int{5, 6},
		Monitored: &monitored,
	})
	if err != nil {
		t.Fatalf("SonarrEditSeries returned error: %v", err)
	}
	if got.method != "PUT" || got.path != "/api/v3/series/editor" {
		t.Errorf("request = %s %s, want PUT /api/v3/series/editor", got.method, got.path)
	}
	if !contains(got.body, `"seriesIds":[5,6]`) || !contains(got.body, `"monitored":false`) {
		t.Errorf("body = %q, want the ids and the monitored flag", got.body)
	}
	if len(series) != 1 || series[0].Monitored {
		t.Errorf("series = %+v, want one unmonitored record", series)
	}
}

// Every editable field is optional upstream: sending a zero value would reset
// a setting the caller never mentioned.
func TestSonarrEditSeriesOmitsUnsetFields(t *testing.T) {
	srv, got := fakeService(t, 202, `[]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	if _, err := SonarrEditSeries(context.Background(), c, SeriesEditRequest{SeriesIDs: []int{1}}); err != nil {
		t.Fatalf("SonarrEditSeries returned error: %v", err)
	}
	for _, unwanted := range []string{"monitored", "qualityProfileId", "rootFolderPath", "seriesType"} {
		if contains(got.body, `"`+unwanted+`"`) {
			t.Errorf("body = %q, must not carry unset field %q", got.body, unwanted)
		}
	}
}

// Applying tags without saying how would let the service pick for us; "add" is
// the only safe default because it never removes an existing tag.
func TestSonarrEditSeriesDefaultsApplyTagsToAdd(t *testing.T) {
	srv, got := fakeService(t, 202, `[]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	if _, err := SonarrEditSeries(context.Background(), c, SeriesEditRequest{
		SeriesIDs: []int{1}, Tags: []int{3},
	}); err != nil {
		t.Fatalf("SonarrEditSeries returned error: %v", err)
	}
	if !contains(got.body, `"applyTags":"add"`) {
		t.Errorf("body = %q, want applyTags defaulted to add", got.body)
	}
}

func TestRadarrEditMoviesPutsToTheEditorEndpoint(t *testing.T) {
	srv, got := fakeService(t, 202, `[{"id":9,"title":"Dune","monitored":true}]`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	monitored := true
	profile := 7
	movies, err := RadarrEditMovies(context.Background(), c, MovieEditRequest{
		MovieIDs: []int{9}, Monitored: &monitored, QualityProfileID: &profile,
	})
	if err != nil {
		t.Fatalf("RadarrEditMovies returned error: %v", err)
	}
	if got.method != "PUT" || got.path != "/api/v3/movie/editor" {
		t.Errorf("request = %s %s, want PUT /api/v3/movie/editor", got.method, got.path)
	}
	if !contains(got.body, `"movieIds":[9]`) || !contains(got.body, `"qualityProfileId":7`) {
		t.Errorf("body = %q, want the ids and the quality profile", got.body)
	}
	if len(movies) != 1 || movies[0].Title != "Dune" {
		t.Errorf("movies = %+v, want Dune", movies)
	}
}

func TestSonarrMonitorEpisodesPutsTheEpisodeIDs(t *testing.T) {
	srv, got := fakeService(t, 202, `[]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	if err := SonarrMonitorEpisodes(context.Background(), c, []int{11, 12}, true); err != nil {
		t.Fatalf("SonarrMonitorEpisodes returned error: %v", err)
	}
	if got.method != "PUT" || got.path != "/api/v3/episode/monitor" {
		t.Errorf("request = %s %s, want PUT /api/v3/episode/monitor", got.method, got.path)
	}
	if !contains(got.body, `"episodeIds":[11,12]`) || !contains(got.body, `"monitored":true`) {
		t.Errorf("body = %q, want the episode ids and the flag", got.body)
	}
}

// Season monitoring has no dedicated endpoint: the season lives inside the
// series resource, so the whole record is read back, edited and written.
func TestSonarrSetSeasonMonitoredReadsModifiesAndWritesTheSeries(t *testing.T) {
	var putBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			body, _ := readAll(r)
			putBody = body
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":5,"title":"New Girl","monitored":true,"tvdbId":248682,
		  "seasons":[{"seasonNumber":1,"monitored":true},{"seasonNumber":2,"monitored":true}],
		  "images":[{"coverType":"poster"}]}`))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	series, err := SonarrSetSeasonMonitored(context.Background(), c, 5, 2, false)
	if err != nil {
		t.Fatalf("SonarrSetSeasonMonitored returned error: %v", err)
	}
	if putBody == "" {
		t.Fatal("no PUT was sent")
	}
	// Decoded rather than string-matched: the record round trips through a map,
	// and Go writes map keys in sorted order rather than the order received.
	var sent struct {
		Seasons []struct {
			SeasonNumber int  `json:"seasonNumber"`
			Monitored    bool `json:"monitored"`
		} `json:"seasons"`
	}
	if err := json.Unmarshal([]byte(putBody), &sent); err != nil {
		t.Fatalf("PUT body is not valid JSON: %v", err)
	}
	if len(sent.Seasons) != 2 {
		t.Fatalf("PUT body carried %d seasons, want 2", len(sent.Seasons))
	}
	if sent.Seasons[1].SeasonNumber != 2 || sent.Seasons[1].Monitored {
		t.Errorf("season 2 = %+v, want unmonitored", sent.Seasons[1])
	}
	if sent.Seasons[0].SeasonNumber != 1 || !sent.Seasons[0].Monitored {
		t.Errorf("season 1 = %+v, must be left alone", sent.Seasons[0])
	}
	// Round-tripping the whole record is the point: dropping fields the tool
	// does not model would silently reset them on the instance.
	if !strings.Contains(putBody, `"images"`) {
		t.Errorf("PUT body = %q, must preserve fields the projection does not model", putBody)
	}
	if series.Title != "New Girl" {
		t.Errorf("series = %+v, want the updated record projected back", series)
	}
}

// A model guessing a season number needs to be told which ones exist.
func TestSonarrSetSeasonMonitoredNamesTheAvailableSeasons(t *testing.T) {
	srv, _ := fakeService(t, 200, `{"id":5,"title":"New Girl","seasons":[{"seasonNumber":1},{"seasonNumber":2}]}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	_, err := SonarrSetSeasonMonitored(context.Background(), c, 5, 9, false)
	if err == nil {
		t.Fatal("expected an error for a season the series does not have")
	}
	if !strings.Contains(err.Error(), "1, 2") {
		t.Errorf("error %q does not list the seasons the caller could have meant", err)
	}
}

// readAll reads a request body for assertions.
func readAll(r *http.Request) (string, error) {
	body, err := io.ReadAll(r.Body)
	return string(body), err
}

package arr

import (
	"context"
	"encoding/json"
	"testing"
)

// One series' episode files come to 252 KB on the live Sonarr, almost all of it
// mediaInfo and custom format detail. The listing has to survive a library.
func TestSonarrListEpisodeFilesTrimsMediaInfo(t *testing.T) {
	srv, got := fakeService(t, 200, `[{
	  "id":6032,"seriesId":5,"seasonNumber":7,
	  "relativePath":"Season 07/New Girl - S07E08.mkv",
	  "path":"/NAS/Shows/New Girl (2011)/Season 07/New Girl - S07E08.mkv",
	  "size":2299894963,"dateAdded":"2025-08-13T17:20:12Z",
	  "sceneName":"New.Girl.S07E08.1080p","releaseGroup":"NTb",
	  "languages":[{"id":1,"name":"English"}],
	  "quality":{"quality":{"id":3,"name":"WEBDL-1080p"},"revision":{"version":1}},
	  "customFormats":[{"id":21,"name":"AMZN"}],"customFormatScore":1700,
	  "qualityCutoffNotMet":true,
	  "mediaInfo":{"audioCodec":"EAC3","videoCodec":"h264","runTime":"22:00","subtitles":"eng"}
	}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	files, err := SonarrListEpisodeFiles(context.Background(), c, 5)
	if err != nil {
		t.Fatalf("SonarrListEpisodeFiles returned error: %v", err)
	}
	if got.path != "/api/v3/episodefile" {
		t.Errorf("path = %q, want /api/v3/episodefile", got.path)
	}
	if !contains(got.query, "seriesId=5") {
		t.Errorf("query = %q, want seriesId=5", got.query)
	}
	if len(files) != 1 {
		t.Fatalf("files = %d, want 1", len(files))
	}
	f := files[0]
	if f.ID != 6032 || f.Size != 2299894963 || f.Quality != "WEBDL-1080p" {
		t.Errorf("file = %+v, want id 6032 at WEBDL-1080p", f)
	}
	if f.SeasonNumber == nil || *f.SeasonNumber != 7 {
		t.Errorf("seasonNumber = %v, want 7", f.SeasonNumber)
	}
	if len(f.Languages) != 1 || f.Languages[0] != "English" {
		t.Errorf("languages = %v, want [English]", f.Languages)
	}
	if encoded, _ := json.Marshal(files); contains(string(encoded), "mediaInfo") ||
		contains(string(encoded), "EAC3") {
		t.Errorf("mediaInfo leaked into the result: %s", encoded)
	}
}

// Season 0 is specials. An int with omitempty would erase it from the output,
// so the field has to be a pointer.
func TestSonarrListEpisodeFilesKeepsSeasonZero(t *testing.T) {
	srv, _ := fakeService(t, 200, `[{"id":1,"seriesId":5,"seasonNumber":0,"relativePath":"Specials/x.mkv"}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	files, err := SonarrListEpisodeFiles(context.Background(), c, 5)
	if err != nil {
		t.Fatalf("SonarrListEpisodeFiles returned error: %v", err)
	}
	encoded, _ := json.Marshal(files)
	if !contains(string(encoded), `"seasonNumber":0`) {
		t.Errorf("season 0 was dropped from the output: %s", encoded)
	}
}

func TestRadarrListMovieFilesFiltersByMovie(t *testing.T) {
	srv, got := fakeService(t, 200, `[{
	  "id":7,"movieId":278,"relativePath":"Dune (2021).mkv","size":12,
	  "quality":{"quality":{"name":"Bluray-2160p"}},"releaseGroup":"FraMeSToR"
	}]`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	files, err := RadarrListMovieFiles(context.Background(), c, 278)
	if err != nil {
		t.Fatalf("RadarrListMovieFiles returned error: %v", err)
	}
	if got.path != "/api/v3/moviefile" {
		t.Errorf("path = %q, want /api/v3/moviefile", got.path)
	}
	if !contains(got.query, "movieId=278") {
		t.Errorf("query = %q, want movieId=278", got.query)
	}
	if len(files) != 1 || files[0].MovieID != 278 || files[0].Quality != "Bluray-2160p" {
		t.Errorf("files = %+v, want one Bluray-2160p file for movie 278", files)
	}
	// A movie file has no season; the field must not appear at all.
	if encoded, _ := json.Marshal(files); contains(string(encoded), "seasonNumber") {
		t.Errorf("movie file carries a season number: %s", encoded)
	}
}

func TestSonarrDeleteEpisodeFilesRemovesEachID(t *testing.T) {
	srv, got := fakeService(t, 200, ``)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	deleted, err := SonarrDeleteEpisodeFiles(context.Background(), c, []int{4, 9})
	if err != nil {
		t.Fatalf("SonarrDeleteEpisodeFiles returned error: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2", deleted)
	}
	want := []string{"DELETE /api/v3/episodefile/4", "DELETE /api/v3/episodefile/9"}
	if len(got.paths) != 2 || got.paths[0] != want[0] || got.paths[1] != want[1] {
		t.Errorf("requests = %v, want %v", got.paths, want)
	}
}

// Deleting files is not undoable, so a caller that passed no ids is far more
// likely to have made a mistake than to have meant "delete nothing".
func TestDeleteFilesRejectsAnEmptyIDList(t *testing.T) {
	srv, got := fakeService(t, 200, ``)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	if _, err := SonarrDeleteEpisodeFiles(context.Background(), c, nil); err == nil {
		t.Fatal("expected an error when no ids are given")
	}
	if len(got.paths) != 0 {
		t.Errorf("upstream was contacted %v for an empty id list", got.paths)
	}
}

// A partial failure has to say how far it got: the files already deleted are
// not coming back, and the caller must not retry the whole list blindly.
func TestDeleteFilesReportsHowManySucceededBeforeFailing(t *testing.T) {
	srv, _ := fakeService(t, 404, `{"message":"not found"}`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	deleted, err := RadarrDeleteMovieFiles(context.Background(), c, []int{1, 2})
	if err == nil {
		t.Fatal("expected an error when the service rejects a delete")
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
	if !contains(err.Error(), "1") {
		t.Errorf("error %q does not name the id that failed", err)
	}
}

func TestSonarrRenamePreviewPassesSeriesAndSeason(t *testing.T) {
	srv, got := fakeService(t, 200, `[{
	  "seriesId":5,"seasonNumber":7,"episodeNumbers":[8],"episodeFileId":6032,
	  "existingPath":"Season 07/old.mkv","newPath":"Season 07/new.mkv"
	}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	season := 7
	previews, err := SonarrRenamePreview(context.Background(), c, 5, &season)
	if err != nil {
		t.Fatalf("SonarrRenamePreview returned error: %v", err)
	}
	if got.path != "/api/v3/rename" {
		t.Errorf("path = %q, want /api/v3/rename", got.path)
	}
	for _, want := range []string{"seriesId=5", "seasonNumber=7"} {
		if !contains(got.query, want) {
			t.Errorf("query = %q, want %q", got.query, want)
		}
	}
	if len(previews) != 1 || previews[0].NewPath != "Season 07/new.mkv" {
		t.Errorf("previews = %+v, want the new path", previews)
	}
}

func TestSonarrRenamePreviewOmitsSeasonWhenNotGiven(t *testing.T) {
	srv, got := fakeService(t, 200, `[]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	if _, err := SonarrRenamePreview(context.Background(), c, 5, nil); err != nil {
		t.Fatalf("SonarrRenamePreview returned error: %v", err)
	}
	if contains(got.query, "seasonNumber") {
		t.Errorf("query = %q, must not constrain the season when none was given", got.query)
	}
}

func TestRadarrRenamePreviewPassesTheMovie(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"movieId":1,"movieFileId":7,"existingPath":"a.mkv","newPath":"b.mkv"}]`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	previews, err := RadarrRenamePreview(context.Background(), c, 1)
	if err != nil {
		t.Fatalf("RadarrRenamePreview returned error: %v", err)
	}
	if got.path != "/api/v3/rename" || !contains(got.query, "movieId=1") {
		t.Errorf("request = %s?%s, want /api/v3/rename?movieId=1", got.path, got.query)
	}
	if len(previews) != 1 || previews[0].MovieFileID != 7 {
		t.Errorf("previews = %+v, want movie file 7", previews)
	}
}

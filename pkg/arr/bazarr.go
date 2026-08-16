package arr

import (
	"context"
	"strconv"
)

// Bazarr wraps most responses in a "data" key, but not all of them: /badges
// returns a bare object and /system/languages a bare array. The helpers below
// keep that inconsistency in one place rather than in every call site.
type bazarrEnvelope[T any] struct {
	Data  T   `json:"data"`
	Total int `json:"total"`
}

// SubtitleLanguage is a language Bazarr can fetch subtitles in.
type SubtitleLanguage struct {
	Name    string `json:"name"`
	Code2   string `json:"code2" jsonschema:"two-letter code, e.g. en"`
	Code3   string `json:"code3,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
	Forced  bool   `json:"forced,omitempty"`
	HI      bool   `json:"hi,omitempty" jsonschema:"hearing impaired"`
}

// BazarrSeries is the trimmed subtitle view of a series known to Bazarr.
// The upstream payload also carries overview, artwork paths and alternate
// titles, which would dominate the response without answering any question.
type BazarrSeries struct {
	SonarrSeriesID      int    `json:"sonarrSeriesId"`
	Title               string `json:"title"`
	Monitored           bool   `json:"monitored"`
	ProfileID           int    `json:"profileId,omitempty" jsonschema:"language profile id, 0 means none assigned"`
	EpisodeFileCount    int    `json:"episodeFileCount"`
	EpisodeMissingCount int    `json:"episodeMissingCount" jsonschema:"episodes missing subtitles"`
}

// BazarrMovie is the trimmed subtitle view of a movie known to Bazarr.
type BazarrMovie struct {
	RadarrID  int    `json:"radarrId"`
	Title     string `json:"title"`
	Monitored bool   `json:"monitored"`
	ProfileID int    `json:"profileId,omitempty"`
}

// WantedEpisode is an episode missing one or more subtitle languages.
type WantedEpisode struct {
	SeriesTitle      string             `json:"seriesTitle"`
	EpisodeTitle     string             `json:"episodeTitle"`
	EpisodeNumber    string             `json:"episode_number" jsonschema:"season and episode, e.g. 8x1"`
	SonarrSeriesID   int                `json:"sonarrSeriesId"`
	SonarrEpisodeID  int                `json:"sonarrEpisodeId"`
	MissingSubtitles []SubtitleLanguage `json:"missing_subtitles"`
}

// WantedMovie is a movie missing one or more subtitle languages.
type WantedMovie struct {
	Title            string             `json:"title"`
	RadarrID         int                `json:"radarrId"`
	MissingSubtitles []SubtitleLanguage `json:"missing_subtitles"`
}

// SubtitleProvider is a configured subtitle source and its current state.
type SubtitleProvider struct {
	Name   string `json:"name"`
	Status string `json:"status,omitempty" jsonschema:"Good, or an error description"`
	Retry  string `json:"retry,omitempty"`
}

// BazarrBadgeCounts summarises outstanding subtitle work. Cheapest first call
// for "is anything missing?" since it avoids listing every item.
type BazarrBadgeCounts struct {
	Episodes      int    `json:"episodes" jsonschema:"episodes missing subtitles"`
	Movies        int    `json:"movies" jsonschema:"movies missing subtitles"`
	Providers     int    `json:"providers" jsonschema:"providers currently throttled or erroring"`
	Status        int    `json:"status" jsonschema:"outstanding health issues"`
	SonarrSignalR string `json:"sonarr_signalr,omitempty" jsonschema:"LIVE when connected to Sonarr"`
	RadarrSignalR string `json:"radarr_signalr,omitempty" jsonschema:"LIVE when connected to Radarr"`
}

// paging renders Bazarr's start/length parameters.
func paging(start, length int) Query {
	if length <= 0 {
		length = 50
	}
	return Query{"start": strconv.Itoa(start), "length": strconv.Itoa(length)}
}

// bazarrList fetches a data-wrapped list and returns it with its total.
func bazarrList[T any](ctx context.Context, c *Client, path string, q ...Query) ([]T, int, error) {
	env, err := GetJSON[bazarrEnvelope[[]T]](ctx, c, path, q...)
	if err != nil {
		return nil, 0, err
	}
	return env.Data, env.Total, nil
}

// BazarrListSeries returns the series Bazarr tracks subtitles for.
func BazarrListSeries(ctx context.Context, c *Client, start, length int) ([]BazarrSeries, error) {
	series, _, err := bazarrList[BazarrSeries](ctx, c, "/series", paging(start, length))
	return series, err
}

// BazarrListMovies returns the movies Bazarr tracks subtitles for.
func BazarrListMovies(ctx context.Context, c *Client, start, length int) ([]BazarrMovie, error) {
	movies, _, err := bazarrList[BazarrMovie](ctx, c, "/movies", paging(start, length))
	return movies, err
}

// BazarrWantedEpisodes returns episodes missing subtitles, with the total count.
func BazarrWantedEpisodes(ctx context.Context, c *Client, start, length int) ([]WantedEpisode, int, error) {
	return bazarrList[WantedEpisode](ctx, c, "/episodes/wanted", paging(start, length))
}

// BazarrWantedMovies returns movies missing subtitles, with the total count.
func BazarrWantedMovies(ctx context.Context, c *Client, start, length int) ([]WantedMovie, int, error) {
	return bazarrList[WantedMovie](ctx, c, "/movies/wanted", paging(start, length))
}

// BazarrProviders returns configured subtitle providers and their status.
func BazarrProviders(ctx context.Context, c *Client) ([]SubtitleProvider, error) {
	providers, _, err := bazarrList[SubtitleProvider](ctx, c, "/providers")
	return providers, err
}

// BazarrHealth returns Bazarr's outstanding health issues.
func BazarrHealth(ctx context.Context, c *Client) ([]HealthIssue, error) {
	issues, _, err := bazarrList[HealthIssue](ctx, c, "/system/health")
	return issues, err
}

// BazarrStatus returns version and environment information.
func BazarrStatus(ctx context.Context, c *Client) (map[string]any, error) {
	env, err := GetJSON[bazarrEnvelope[map[string]any]](ctx, c, "/system/status")
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// BazarrBadges returns outstanding subtitle counts. This endpoint is not
// data-wrapped, unlike most of the Bazarr API.
func BazarrBadges(ctx context.Context, c *Client) (BazarrBadgeCounts, error) {
	return GetJSON[BazarrBadgeCounts](ctx, c, "/badges")
}

// BazarrLanguages returns Bazarr's languages. This endpoint returns a bare
// array rather than a data-wrapped object. Bazarr lists every ISO language, so
// enabledOnly filters to the ones actually configured.
func BazarrLanguages(ctx context.Context, c *Client, enabledOnly bool) ([]SubtitleLanguage, error) {
	all, err := GetJSON[[]SubtitleLanguage](ctx, c, "/system/languages")
	if err != nil {
		return nil, err
	}
	if !enabledOnly {
		return all, nil
	}
	out := make([]SubtitleLanguage, 0, 8)
	for _, l := range all {
		if l.Enabled {
			out = append(out, l)
		}
	}
	return out, nil
}

// BazarrSearchEpisodeSubtitles asks Bazarr to find and download a subtitle for
// one episode in the given language.
func BazarrSearchEpisodeSubtitles(ctx context.Context, c *Client, seriesID, episodeID int, language string, forced, hi bool) error {
	_, err := c.Patch(ctx, "/episodes/subtitles", nil, Query{
		"seriesid":  itoa(seriesID),
		"episodeid": itoa(episodeID),
		"language":  language,
		"forced":    btoa(forced),
		"hi":        btoa(hi),
	})
	return err
}

// BazarrSearchMovieSubtitles asks Bazarr to find and download a subtitle for
// one movie in the given language.
func BazarrSearchMovieSubtitles(ctx context.Context, c *Client, radarrID int, language string, forced, hi bool) error {
	_, err := c.Patch(ctx, "/movies/subtitles", nil, Query{
		"radarrid": itoa(radarrID),
		"language": language,
		"forced":   btoa(forced),
		"hi":       btoa(hi),
	})
	return err
}

// BazarrDeleteEpisodeSubtitle removes a downloaded subtitle file for an episode.
func BazarrDeleteEpisodeSubtitle(ctx context.Context, c *Client, seriesID, episodeID int, language, path string, forced, hi bool) error {
	_, err := c.Delete(ctx, "/episodes/subtitles", Query{
		"seriesid":  itoa(seriesID),
		"episodeid": itoa(episodeID),
		"language":  language,
		"path":      path,
		"forced":    btoa(forced),
		"hi":        btoa(hi),
	})
	return err
}

// BazarrDeleteMovieSubtitle removes a downloaded subtitle file for a movie.
func BazarrDeleteMovieSubtitle(ctx context.Context, c *Client, radarrID int, language, path string, forced, hi bool) error {
	_, err := c.Delete(ctx, "/movies/subtitles", Query{
		"radarrid": itoa(radarrID),
		"language": language,
		"path":     path,
		"forced":   btoa(forced),
		"hi":       btoa(hi),
	})
	return err
}

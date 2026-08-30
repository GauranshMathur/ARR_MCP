package arr

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// subtitleSearchTimeout bounds a provider search, which blocks while Bazarr
// queries every configured provider, downloads and post-processes. That
// routinely exceeds the default read timeout, and aborting early would report
// a failure while Bazarr keeps working -- inviting a duplicate retry.
const subtitleSearchTimeout = 5 * time.Minute

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
	Enabled bool   `json:"enabled"`
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
	ProfileID           int    `json:"profileId" jsonschema:"language profile id; 0 means none assigned, which is why a series gets no subtitles"`
	EpisodeFileCount    int    `json:"episodeFileCount"`
	EpisodeMissingCount int    `json:"episodeMissingCount" jsonschema:"episodes missing subtitles"`
}

// BazarrMovie is the trimmed subtitle view of a movie known to Bazarr.
type BazarrMovie struct {
	RadarrID  int    `json:"radarrId"`
	Title     string `json:"title"`
	Monitored bool   `json:"monitored"`
	ProfileID int    `json:"profileId" jsonschema:"language profile id; 0 means none assigned"`
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

// BazarrHealthIssue is a Bazarr health problem. Bazarr reports {object, issue}
// rather than the {source, type, message} the *arr apps use.
type BazarrHealthIssue struct {
	Object string `json:"object" jsonschema:"the path or item the problem concerns"`
	Issue  string `json:"issue" jsonschema:"what is wrong with it"`
}

// EpisodeSubtitles lists the subtitles present and missing for one episode.
type EpisodeSubtitles struct {
	SonarrSeriesID   int                `json:"sonarrSeriesId"`
	SonarrEpisodeID  int                `json:"sonarrEpisodeId"`
	Title            string             `json:"title"`
	Season           int                `json:"season"`
	Episode          int                `json:"episode"`
	Subtitles        []SubtitleFile     `json:"subtitles"`
	MissingSubtitles []SubtitleLanguage `json:"missing_subtitles"`
}

// SubtitleFile is a subtitle already attached to an episode. Path is empty for
// tracks embedded in the media file, which cannot be deleted individually.
type SubtitleFile struct {
	Name   string `json:"name"`
	Code2  string `json:"code2"`
	Path   string `json:"path,omitempty" jsonschema:"file path, required to delete this subtitle; empty means the track is embedded"`
	Forced bool   `json:"forced"`
	HI     bool   `json:"hi"`
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

// BazarrListSeries returns a page of tracked series with the library total.
func BazarrListSeries(ctx context.Context, c *Client, start, length int) ([]BazarrSeries, int, error) {
	return bazarrList[BazarrSeries](ctx, c, "/series", paging(start, length))
}

// BazarrListMovies returns a page of tracked movies with the library total.
func BazarrListMovies(ctx context.Context, c *Client, start, length int) ([]BazarrMovie, int, error) {
	return bazarrList[BazarrMovie](ctx, c, "/movies", paging(start, length))
}

// BazarrListEpisodeSubtitles returns the subtitles present and missing for each
// episode of a series. This is the only source of the file paths the deletion
// tools require.
func BazarrListEpisodeSubtitles(ctx context.Context, c *Client, seriesID int) ([]EpisodeSubtitles, error) {
	eps, _, err := bazarrList[EpisodeSubtitles](ctx, c, "/episodes", Query{"seriesid[]": itoa(seriesID)})
	return eps, err
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
func BazarrHealth(ctx context.Context, c *Client) ([]BazarrHealthIssue, error) {
	issues, _, err := bazarrList[BazarrHealthIssue](ctx, c, "/system/health")
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
	_, err := c.WithTimeout(subtitleSearchTimeout).Patch(ctx, "/episodes/subtitles", Query{
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
	_, err := c.WithTimeout(subtitleSearchTimeout).Patch(ctx, "/movies/subtitles", Query{
		"radarrid": itoa(radarrID),
		"language": language,
		"forced":   btoa(forced),
		"hi":       btoa(hi),
	})
	return err
}

// BazarrDeleteEpisodeSubtitle removes a downloaded subtitle file for an episode.
func BazarrDeleteEpisodeSubtitle(ctx context.Context, c *Client, seriesID, episodeID int, language, path string, forced, hi bool) error {
	if path == "" {
		return fmt.Errorf("subtitle path is required; take it from bazarr_list_episode_subtitles " +
			"(an empty path means the track is embedded and cannot be deleted)")
	}
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
	if path == "" {
		return fmt.Errorf("subtitle path is required; an empty path means the track is embedded " +
			"in the media file and cannot be deleted")
	}
	_, err := c.Delete(ctx, "/movies/subtitles", Query{
		"radarrid": itoa(radarrID),
		"language": language,
		"path":     path,
		"forced":   btoa(forced),
		"hi":       btoa(hi),
	})
	return err
}

// pyBool renders a boolean the way PATCH /subtitles reads it. That endpoint
// compares the raw query value to the Python literal "True", while every other
// Bazarr endpoint calls .capitalize() on it first and so accepts Go's
// lowercase rendering. Sending "true" there means false, silently.
func pyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// bazarrKind validates the episodes/movies selector shared by the blacklist
// endpoints and returns it as a path segment. The error names both valid
// values so a caller that guessed the singular can correct itself.
func bazarrKind(kind string) (string, error) {
	switch kind {
	case "episodes", "movies":
		return kind, nil
	default:
		return "", fmt.Errorf("unknown kind %q; valid kinds: episodes, movies", kind)
	}
}

// LanguageProfileItem is one language inside a Bazarr languages profile.
type LanguageProfileItem struct {
	Language string `json:"language" jsonschema:"two-letter code, e.g. en"`
	HI       bool   `json:"hi,omitempty" jsonschema:"prefer hearing impaired subtitles"`
	Forced   bool   `json:"forced,omitempty"`
}

// LanguageProfile is a Bazarr languages profile: the set of subtitle languages
// wanted for whatever series or movies it is assigned to.
type LanguageProfile struct {
	ProfileID int                   `json:"profileId" jsonschema:"assign this with bazarr_set_series_profile or bazarr_set_movie_profile"`
	Name      string                `json:"name"`
	Items     []LanguageProfileItem `json:"items"`
}

// rawLanguageProfile is the upstream shape. Its per-item booleans arrive as
// the Python strings "True" and "False", unlike the rest of the API.
type rawLanguageProfile struct {
	ProfileID int    `json:"profileId"`
	Name      string `json:"name"`
	Items     []struct {
		Language string `json:"language"`
		HI       string `json:"hi"`
		Forced   string `json:"forced"`
	} `json:"items"`
}

// SubtitleCandidate is one provider result from a manual subtitle search.
// Subtitle is opaque and only meaningful to the provider that produced it.
type SubtitleCandidate struct {
	Provider        string   `json:"provider" jsonschema:"pass back to the download tool unchanged"`
	Subtitle        string   `json:"subtitle" jsonschema:"opaque token identifying this result; pass back to the download tool unchanged"`
	Language        string   `json:"language"`
	Score           int      `json:"score" jsonschema:"match quality as a percentage"`
	HearingImpaired bool     `json:"hearingImpaired"`
	Forced          bool     `json:"forced"`
	OriginalFormat  bool     `json:"originalFormat"`
	Matches         []string `json:"matches,omitempty" jsonschema:"release attributes this subtitle matches"`
	DontMatches     []string `json:"dontMatches,omitempty" jsonschema:"release attributes it does not match"`
	ReleaseInfo     []string `json:"releaseInfo,omitempty" jsonschema:"releases the subtitle was timed for"`
	Uploader        string   `json:"uploader,omitempty"`
	URL             string   `json:"url,omitempty"`
}

// rawSubtitleCandidate is the upstream shape, whose three flags are the Python
// strings "True" and "False" rather than JSON booleans.
type rawSubtitleCandidate struct {
	Provider        string   `json:"provider"`
	Subtitle        string   `json:"subtitle"`
	Language        string   `json:"language"`
	Score           int      `json:"score"`
	HearingImpaired string   `json:"hearing_impaired"`
	Forced          string   `json:"forced"`
	OriginalFormat  string   `json:"original_format"`
	Matches         []string `json:"matches"`
	DontMatches     []string `json:"dont_matches"`
	ReleaseInfo     []string `json:"release_info"`
	Uploader        string   `json:"uploader"`
	URL             string   `json:"url"`
}

// SubtitleHistoryRecord is one subtitle download, upgrade or removal. The
// series fields are set for episode history and the movie fields for movie
// history; Provider, SubsID and SubtitlesPath are what the blacklist tools
// need to reject a bad subtitle.
type SubtitleHistoryRecord struct {
	SeriesTitle     string           `json:"seriesTitle,omitempty"`
	EpisodeTitle    string           `json:"episodeTitle,omitempty"`
	EpisodeNumber   string           `json:"episode_number,omitempty" jsonschema:"season and episode, e.g. 1x7"`
	SonarrSeriesID  int              `json:"sonarrSeriesId,omitempty"`
	SonarrEpisodeID int              `json:"sonarrEpisodeId,omitempty"`
	Title           string           `json:"title,omitempty" jsonschema:"movie title"`
	RadarrID        int              `json:"radarrId,omitempty"`
	Action          int              `json:"action" jsonschema:"1 downloaded, 2 manually downloaded, 3 upgraded, 0 deleted"`
	Description     string           `json:"description,omitempty"`
	Provider        string           `json:"provider,omitempty"`
	SubsID          string           `json:"subs_id,omitempty" jsonschema:"provider-side subtitle id, required to blacklist it"`
	Language        SubtitleLanguage `json:"language"`
	Score           string           `json:"score,omitempty"`
	SubtitlesPath   string           `json:"subtitles_path,omitempty" jsonschema:"where the subtitle was written"`
	Timestamp       string           `json:"timestamp,omitempty" jsonschema:"relative, e.g. last month"`
	ParsedTimestamp string           `json:"parsed_timestamp,omitempty"`
	Upgradable      bool             `json:"upgradable,omitempty"`
	Blacklisted     bool             `json:"blacklisted,omitempty"`
}

// SubtitleBlacklistItem is a subtitle Bazarr has been told never to fetch
// again. Provider and SubsID together identify it for removal.
type SubtitleBlacklistItem struct {
	SeriesTitle     string           `json:"seriesTitle,omitempty"`
	EpisodeTitle    string           `json:"episodeTitle,omitempty"`
	EpisodeNumber   string           `json:"episode_number,omitempty"`
	SonarrSeriesID  int              `json:"sonarrSeriesId,omitempty"`
	Title           string           `json:"title,omitempty" jsonschema:"movie title"`
	RadarrID        int              `json:"radarrId,omitempty"`
	Provider        string           `json:"provider"`
	SubsID          string           `json:"subs_id" jsonschema:"pass with provider to remove this entry"`
	Language        SubtitleLanguage `json:"language"`
	Timestamp       string           `json:"timestamp,omitempty"`
	ParsedTimestamp string           `json:"parsed_timestamp,omitempty"`
}

// BazarrTask is one of Bazarr's scheduled jobs. JobID is what runs it.
type BazarrTask struct {
	JobID      string `json:"job_id" jsonschema:"pass to bazarr_run_task to run it now"`
	Name       string `json:"name"`
	Interval   string `json:"interval"`
	JobRunning bool   `json:"job_running"`
	NextRunIn  string `json:"next_run_in,omitempty"`
}

// AudioTrack is one audio stream in a media file. Stream is the ffmpeg-style
// index a subtitle sync can use as its reference.
type AudioTrack struct {
	Stream   string `json:"stream" jsonschema:"track reference, e.g. a:0"`
	Name     string `json:"name,omitempty"`
	Language string `json:"language,omitempty"`
}

// EmbeddedSubtitleTrack is a subtitle stream inside the media file itself.
type EmbeddedSubtitleTrack struct {
	Stream          string `json:"stream" jsonschema:"track reference, e.g. s:0"`
	Name            string `json:"name,omitempty"`
	Language        string `json:"language,omitempty"`
	Forced          bool   `json:"forced,omitempty"`
	HearingImpaired bool   `json:"hearing_impaired,omitempty"`
}

// ExternalSubtitleTrack is a subtitle file alongside the media file.
type ExternalSubtitleTrack struct {
	Name            string `json:"name,omitempty"`
	Path            string `json:"path,omitempty"`
	Language        string `json:"language,omitempty"`
	Forced          bool   `json:"forced,omitempty"`
	HearingImpaired bool   `json:"hearing_impaired,omitempty"`
}

// SubtitleTracks lists what a media file contains alongside one subtitle: the
// audio and embedded tracks a sync can reference, plus the other external
// subtitle files.
type SubtitleTracks struct {
	AudioTracks       []AudioTrack            `json:"audio_tracks"`
	EmbeddedSubtitles []EmbeddedSubtitleTrack `json:"embedded_subtitles_tracks"`
	ExternalSubtitles []ExternalSubtitleTrack `json:"external_subtitles_tracks"`
}

// SubtitleMod describes one edit to an existing subtitle file.
type SubtitleMod struct {
	// Action is "sync", "translate" or a subzero mod name.
	Action string
	// Language is the subtitle's two-letter code, or the target language when
	// translating.
	Language string
	// Path is the subtitle file to edit.
	Path string
	// MediaType is "episode" or "movie".
	MediaType string
	// MediaID is the sonarrEpisodeId or radarrId the subtitle belongs to.
	MediaID int
	// Forced marks the subtitle as forced.
	Forced bool
	// HI marks the subtitle as hearing impaired.
	HI bool
	// OriginalFormat keeps the subtitle's original format instead of srt.
	OriginalFormat bool
	// Reference is the sync reference: an audio track like "a:0" or a subtitle
	// file path. Empty means the video file itself.
	Reference string
	// MaxOffsetSeconds bounds how far a sync may shift the subtitle.
	MaxOffsetSeconds int
	// NoFixFramerate stops a sync from correcting the framerate.
	NoFixFramerate bool
	// GSS selects Golden-Section Search for a sync.
	GSS bool
}

// BazarrLanguageProfiles returns the configured languages profiles. Like
// /system/languages and /badges, this endpoint is not data-wrapped.
func BazarrLanguageProfiles(ctx context.Context, c *Client) ([]LanguageProfile, error) {
	raw, err := GetJSON[[]rawLanguageProfile](ctx, c, "/system/languages/profiles")
	if err != nil {
		return nil, err
	}
	out := make([]LanguageProfile, 0, len(raw))
	for _, p := range raw {
		profile := LanguageProfile{ProfileID: p.ProfileID, Name: p.Name}
		for _, item := range p.Items {
			profile.Items = append(profile.Items, LanguageProfileItem{
				Language: item.Language,
				HI:       strings.EqualFold(item.HI, "true"),
				Forced:   strings.EqualFold(item.Forced, "true"),
			})
		}
		out = append(out, profile)
	}
	return out, nil
}

// setProfile assigns one languages profile to each id. Bazarr takes parallel
// seriesid/profileid arrays -- note the names carry no "[]" suffix here, even
// though the GET variants' do -- and Query cannot express a repeated key, so
// each id gets its own request. The arrays being positional makes that
// equivalent to one call carrying the whole list.
func setProfile(ctx context.Context, c *Client, path, idParam string, ids []int, profileID int) error {
	if len(ids) == 0 {
		return fmt.Errorf("at least one id is required")
	}
	profile := itoa(profileID)
	if profileID == 0 {
		// Bazarr spells "no profile" as the literal "none"; profile 0 does not
		// exist, so sending it would fail rather than unassign.
		profile = "none"
	}
	for _, id := range ids {
		q := Query{idParam: itoa(id), "profileid": profile}
		if _, err := c.do(ctx, http.MethodPost, path, nil, q); err != nil {
			return fmt.Errorf("setting profile on %s %d: %w", idParam, id, err)
		}
	}
	return nil
}

// BazarrSetSeriesProfile assigns a languages profile to each series. A
// profileID of 0 unassigns it, which stops Bazarr fetching subtitles at all.
func BazarrSetSeriesProfile(ctx context.Context, c *Client, seriesIDs []int, profileID int) error {
	return setProfile(ctx, c, "/series", "seriesid", seriesIDs, profileID)
}

// BazarrSetMovieProfile assigns a languages profile to each movie. A profileID
// of 0 unassigns it.
func BazarrSetMovieProfile(ctx context.Context, c *Client, radarrIDs []int, profileID int) error {
	return setProfile(ctx, c, "/movies", "radarrid", radarrIDs, profileID)
}

// BazarrSeriesAction runs scan-disk, search-missing or search-wanted against
// one series.
func BazarrSeriesAction(ctx context.Context, c *Client, seriesID int, action string) error {
	_, err := c.WithTimeout(subtitleSearchTimeout).Patch(ctx, "/series", Query{
		"seriesid": itoa(seriesID),
		"action":   action,
	})
	return err
}

// BazarrMovieAction runs scan-disk, search-missing or search-wanted against
// one movie.
func BazarrMovieAction(ctx context.Context, c *Client, radarrID int, action string) error {
	_, err := c.WithTimeout(subtitleSearchTimeout).Patch(ctx, "/movies", Query{
		"radarrid": itoa(radarrID),
		"action":   action,
	})
	return err
}

// manualSearch runs a provider search and trims the results. It queries every
// configured provider synchronously, so it gets the long search timeout rather
// than the default read timeout.
func manualSearch(ctx context.Context, c *Client, path string, q Query) ([]SubtitleCandidate, error) {
	raw, _, err := bazarrList[rawSubtitleCandidate](ctx, c.WithTimeout(subtitleSearchTimeout), path, q)
	if err != nil {
		return nil, err
	}
	out := make([]SubtitleCandidate, 0, len(raw))
	for _, r := range raw {
		out = append(out, SubtitleCandidate{
			Provider:        r.Provider,
			Subtitle:        r.Subtitle,
			Language:        r.Language,
			Score:           r.Score,
			HearingImpaired: strings.EqualFold(r.HearingImpaired, "true"),
			Forced:          strings.EqualFold(r.Forced, "true"),
			OriginalFormat:  strings.EqualFold(r.OriginalFormat, "true"),
			Matches:         r.Matches,
			DontMatches:     r.DontMatches,
			ReleaseInfo:     r.ReleaseInfo,
			Uploader:        r.Uploader,
			URL:             r.URL,
		})
	}
	return out, nil
}

// BazarrManualSearchEpisode lists the subtitles providers currently offer for
// one episode, without downloading any of them.
func BazarrManualSearchEpisode(ctx context.Context, c *Client, episodeID int) ([]SubtitleCandidate, error) {
	return manualSearch(ctx, c, "/providers/episodes", Query{"episodeid": itoa(episodeID)})
}

// BazarrManualSearchMovie lists the subtitles providers currently offer for
// one movie, without downloading any of them.
func BazarrManualSearchMovie(ctx context.Context, c *Client, radarrID int) ([]SubtitleCandidate, error) {
	return manualSearch(ctx, c, "/providers/movies", Query{"radarrid": itoa(radarrID)})
}

// BazarrDownloadEpisodeSubtitle downloads one specific search result. Both
// provider and subtitle must come from BazarrManualSearchEpisode: the token is
// opaque and only that provider can resolve it.
func BazarrDownloadEpisodeSubtitle(ctx context.Context, c *Client, seriesID, episodeID int,
	provider, subtitle string, hi, forced, originalFormat bool) error {
	if provider == "" || subtitle == "" {
		return fmt.Errorf("provider and subtitle are required; take both from bazarr_manual_search_episode")
	}
	_, err := c.WithTimeout(subtitleSearchTimeout).do(ctx, http.MethodPost, "/providers/episodes", nil, Query{
		"seriesid":        itoa(seriesID),
		"episodeid":       itoa(episodeID),
		"provider":        provider,
		"subtitle":        subtitle,
		"hi":              btoa(hi),
		"forced":          btoa(forced),
		"original_format": btoa(originalFormat),
	})
	return err
}

// BazarrDownloadMovieSubtitle downloads one specific search result for a
// movie. Both provider and subtitle must come from BazarrManualSearchMovie.
func BazarrDownloadMovieSubtitle(ctx context.Context, c *Client, radarrID int,
	provider, subtitle string, hi, forced, originalFormat bool) error {
	if provider == "" || subtitle == "" {
		return fmt.Errorf("provider and subtitle are required; take both from bazarr_manual_search_movie")
	}
	_, err := c.WithTimeout(subtitleSearchTimeout).do(ctx, http.MethodPost, "/providers/movies", nil, Query{
		"radarrid":        itoa(radarrID),
		"provider":        provider,
		"subtitle":        subtitle,
		"hi":              btoa(hi),
		"forced":          btoa(forced),
		"original_format": btoa(originalFormat),
	})
	return err
}

// BazarrEpisodeHistory returns subtitle history for episodes, newest first,
// with the total across all pages. A non-zero episodeID narrows it to one
// episode.
func BazarrEpisodeHistory(ctx context.Context, c *Client, start, length, episodeID int) ([]SubtitleHistoryRecord, int, error) {
	q := paging(start, length)
	if episodeID > 0 {
		q["episodeid"] = itoa(episodeID)
	}
	return bazarrList[SubtitleHistoryRecord](ctx, c, "/episodes/history", q)
}

// BazarrMovieHistory returns subtitle history for movies, newest first, with
// the total across all pages. A non-zero radarrID narrows it to one movie.
func BazarrMovieHistory(ctx context.Context, c *Client, start, length, radarrID int) ([]SubtitleHistoryRecord, int, error) {
	q := paging(start, length)
	if radarrID > 0 {
		q["radarrid"] = itoa(radarrID)
	}
	return bazarrList[SubtitleHistoryRecord](ctx, c, "/movies/history", q)
}

// BazarrBlacklist returns blacklisted subtitles for "episodes" or "movies".
//
// Paging is applied here rather than upstream: GET /movies/blacklist answers
// 500 for any length above zero, because it calls .limit() on a result set it
// has already executed. Omitting the parameter is the only way to read it at
// all, and the episode endpoint is treated the same way so both behave alike.
func BazarrBlacklist(ctx context.Context, c *Client, kind string, start, length int) ([]SubtitleBlacklistItem, error) {
	path, err := bazarrKind(kind)
	if err != nil {
		return nil, err
	}
	items, _, err := bazarrList[SubtitleBlacklistItem](ctx, c, "/"+path+"/blacklist")
	if err != nil {
		return nil, err
	}
	if start < 0 {
		start = 0
	}
	if start >= len(items) {
		return nil, nil
	}
	items = items[start:]
	if length > 0 && length < len(items) {
		items = items[:length]
	}
	return items, nil
}

// BazarrBlacklistEpisodeSubtitle blacklists one episode subtitle. Bazarr also
// deletes the subtitle file and starts a replacement search.
func BazarrBlacklistEpisodeSubtitle(ctx context.Context, c *Client, seriesID, episodeID int,
	provider, subsID, language, subtitlesPath string) error {
	if provider == "" || subsID == "" || subtitlesPath == "" {
		return fmt.Errorf("provider, subsId and subtitlesPath are required; take them from bazarr_episode_history")
	}
	_, err := c.WithTimeout(subtitleSearchTimeout).do(ctx, http.MethodPost, "/episodes/blacklist", nil, Query{
		"seriesid":       itoa(seriesID),
		"episodeid":      itoa(episodeID),
		"provider":       provider,
		"subs_id":        subsID,
		"language":       language,
		"subtitles_path": subtitlesPath,
	})
	return err
}

// BazarrBlacklistMovieSubtitle blacklists one movie subtitle. Bazarr also
// deletes the subtitle file and starts a replacement search.
func BazarrBlacklistMovieSubtitle(ctx context.Context, c *Client, radarrID int,
	provider, subsID, language, subtitlesPath string) error {
	if provider == "" || subsID == "" || subtitlesPath == "" {
		return fmt.Errorf("provider, subsId and subtitlesPath are required; take them from bazarr_movie_history")
	}
	_, err := c.WithTimeout(subtitleSearchTimeout).do(ctx, http.MethodPost, "/movies/blacklist", nil, Query{
		"radarrid":       itoa(radarrID),
		"provider":       provider,
		"subs_id":        subsID,
		"language":       language,
		"subtitles_path": subtitlesPath,
	})
	return err
}

// BazarrDeleteBlacklistItem removes one blacklist entry, or empties the list
// when all is true. Bazarr compares the all parameter to the lowercase literal
// "true", so it is the one flag that must not be capitalised.
func BazarrDeleteBlacklistItem(ctx context.Context, c *Client, kind, provider, subsID string, all bool) error {
	path, err := bazarrKind(kind)
	if err != nil {
		return err
	}
	q := Query{}
	switch {
	case all:
		q["all"] = btoa(true)
	case provider != "" && subsID != "":
		q["provider"] = provider
		q["subs_id"] = subsID
	default:
		return fmt.Errorf("provider and subsId are required unless all is true; take them from bazarr_list_blacklist")
	}
	_, err = c.Delete(ctx, "/"+path+"/blacklist", q)
	return err
}

// BazarrResetProviders clears the throttling state of every provider, so
// providers disabled by an error are retried immediately.
func BazarrResetProviders(ctx context.Context, c *Client) error {
	_, err := c.do(ctx, http.MethodPost, "/providers", nil, Query{"action": "reset"})
	return err
}

// BazarrTasks returns Bazarr's scheduled jobs and when each next runs.
func BazarrTasks(ctx context.Context, c *Client) ([]BazarrTask, error) {
	tasks, _, err := bazarrList[BazarrTask](ctx, c, "/system/tasks")
	return tasks, err
}

// BazarrRunTask runs one scheduled job now. The id is a job_id from
// BazarrTasks; Bazarr ignores an unknown one rather than reporting it.
func BazarrRunTask(ctx context.Context, c *Client, taskID string) error {
	if taskID == "" {
		return fmt.Errorf("task id is required; take job_id from bazarr_list_tasks")
	}
	_, err := c.do(ctx, http.MethodPost, "/system/tasks", nil, Query{"taskid": taskID})
	return err
}

// BazarrModifySubtitle applies a mod, a sync or a translation to an existing
// subtitle file, in place. Syncing and translating are slow enough to need the
// long timeout.
func BazarrModifySubtitle(ctx context.Context, c *Client, mod SubtitleMod) error {
	if mod.MediaType != "episode" && mod.MediaType != "movie" {
		return fmt.Errorf("unknown media type %q; valid types: episode, movie", mod.MediaType)
	}
	if mod.Action == "" || mod.Path == "" || mod.Language == "" {
		return fmt.Errorf("action, language and path are required; take path from bazarr_list_episode_subtitles")
	}
	q := Query{
		"action":          mod.Action,
		"language":        mod.Language,
		"path":            mod.Path,
		"type":            mod.MediaType,
		"id":              itoa(mod.MediaID),
		"hi":              pyBool(mod.HI),
		"forced":          pyBool(mod.Forced),
		"original_format": pyBool(mod.OriginalFormat),
	}
	// The sync options are optional upstream: sending them empty would pin the
	// offset to "" and lose Bazarr's own configured defaults.
	if mod.Reference != "" {
		q["reference"] = mod.Reference
	}
	if mod.MaxOffsetSeconds > 0 {
		q["max_offset_seconds"] = itoa(mod.MaxOffsetSeconds)
	}
	if mod.NoFixFramerate {
		q["no_fix_framerate"] = pyBool(true)
	}
	if mod.GSS {
		q["gss"] = pyBool(true)
	}
	_, err := c.WithTimeout(subtitleSearchTimeout).Patch(ctx, "/subtitles", q)
	return err
}

// BazarrSubtitleTracks lists the audio, embedded and external tracks beside
// one subtitle file. It answers a data-wrapped object rather than a list, and
// is the only place the track references a sync can use are visible.
func BazarrSubtitleTracks(ctx context.Context, c *Client, subtitlesPath string, episodeID, radarrID int) (SubtitleTracks, error) {
	if subtitlesPath == "" {
		return SubtitleTracks{}, fmt.Errorf("subtitle path is required; take it from bazarr_list_episode_subtitles")
	}
	q := Query{"subtitlesPath": subtitlesPath}
	if episodeID > 0 {
		q["sonarrEpisodeId"] = itoa(episodeID)
	}
	if radarrID > 0 {
		q["radarrMovieId"] = itoa(radarrID)
	}
	env, err := GetJSON[bazarrEnvelope[SubtitleTracks]](ctx, c, "/subtitles", q)
	if err != nil {
		return SubtitleTracks{}, err
	}
	return env.Data, nil
}

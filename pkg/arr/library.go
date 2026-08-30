package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// searchTimeout bounds the calls that reach out to indexers. An interactive
// release search queries every configured indexer synchronously and routinely
// takes far longer than an ordinary read; the grab that follows hands the
// release to a download client, which is just as slow.
const searchTimeout = 2 * time.Minute

// defaultReleaseLimit caps an interactive search. A real query returns 200+
// releases, which would spend most of the model's context on one call.
const defaultReleaseLimit = 30

// rejectionList decodes the reasons a service refused a release or a file.
//
// The field is a list of plain strings on /release but a list of
// {reason, type} objects on /manualimport. Accepting only one shape would fail
// the entire call against the other endpoint.
type rejectionList []string

// UnmarshalJSON accepts either a list of strings or a list of reason objects.
func (r *rejectionList) UnmarshalJSON(data []byte) error {
	var plain []string
	if err := json.Unmarshal(data, &plain); err == nil {
		*r = plain
		return nil
	}
	var structured []struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(data, &structured); err != nil {
		return fmt.Errorf("rejections are neither strings nor objects: %s", data)
	}
	*r = nil
	for _, entry := range structured {
		if entry.Reason != "" {
			*r = append(*r, entry.Reason)
		}
	}
	return nil
}

// --- interactive release search ---

// ReleaseCandidate is one release an indexer offered, trimmed to what a caller
// needs to choose between them and then grab one. GUID and IndexerID together
// identify the release to the grab endpoint.
type ReleaseCandidate struct {
	GUID              string   `json:"guid" jsonschema:"pass to the grab_release tool together with indexerId"`
	IndexerID         int      `json:"indexerId"`
	Indexer           string   `json:"indexer,omitempty"`
	Title             string   `json:"title"`
	Quality           string   `json:"quality,omitempty"`
	Size              int64    `json:"size,omitempty" jsonschema:"release size in bytes"`
	Seeders           *int     `json:"seeders,omitempty" jsonschema:"torrents only; absent for usenet"`
	Leechers          *int     `json:"leechers,omitempty"`
	Age               int      `json:"age,omitempty" jsonschema:"days since the release was published"`
	Protocol          string   `json:"protocol,omitempty" jsonschema:"usenet or torrent"`
	PublishDate       string   `json:"publishDate,omitempty"`
	Approved          bool     `json:"approved" jsonschema:"false when the service would reject this release"`
	Rejections        []string `json:"rejections,omitempty" jsonschema:"why the service would refuse it; grabbing anyway overrides these"`
	CustomFormatScore int      `json:"customFormatScore,omitempty"`
	ReleaseGroup      string   `json:"releaseGroup,omitempty"`
	SeasonNumber      *int     `json:"seasonNumber,omitempty" jsonschema:"Sonarr only"`
	FullSeason        bool     `json:"fullSeason,omitempty" jsonschema:"Sonarr only; true for a whole-season pack"`
}

// rawRelease mirrors the upstream release resource. The payload also carries
// per-release custom format definitions, mapped episode info and several URL
// variants; none of them have a member here, so they cannot ride along.
type rawRelease struct {
	GUID      string `json:"guid"`
	IndexerID int    `json:"indexerId"`
	Indexer   string `json:"indexer"`
	Title     string `json:"title"`
	Quality   struct {
		Quality struct {
			Name string `json:"name"`
		} `json:"quality"`
	} `json:"quality"`
	Size              int64         `json:"size"`
	Seeders           *int          `json:"seeders"`
	Leechers          *int          `json:"leechers"`
	Age               int           `json:"age"`
	Protocol          string        `json:"protocol"`
	PublishDate       string        `json:"publishDate"`
	Approved          bool          `json:"approved"`
	Rejections        rejectionList `json:"rejections"`
	CustomFormatScore int           `json:"customFormatScore"`
	ReleaseGroup      string        `json:"releaseGroup"`
	SeasonNumber      *int          `json:"seasonNumber"`
	FullSeason        bool          `json:"fullSeason"`
}

// toReleaseCandidate projects an upstream release onto the trimmed view.
func (r rawRelease) toReleaseCandidate() ReleaseCandidate {
	return ReleaseCandidate{
		GUID: r.GUID, IndexerID: r.IndexerID, Indexer: r.Indexer, Title: r.Title,
		Quality: r.Quality.Quality.Name, Size: r.Size,
		Seeders: r.Seeders, Leechers: r.Leechers, Age: r.Age,
		Protocol: r.Protocol, PublishDate: r.PublishDate,
		Approved: r.Approved, Rejections: r.Rejections,
		CustomFormatScore: r.CustomFormatScore, ReleaseGroup: r.ReleaseGroup,
		SeasonNumber: r.SeasonNumber, FullSeason: r.FullSeason,
	}
}

// listReleases runs one interactive search and trims the result to limit.
func listReleases(ctx context.Context, c *Client, q Query, limit int) ([]ReleaseCandidate, error) {
	if limit <= 0 {
		limit = defaultReleaseLimit
	}
	raw, err := GetJSON[[]rawRelease](ctx, c.WithTimeout(searchTimeout), "/release", q)
	if err != nil {
		return nil, err
	}
	if len(raw) > limit {
		raw = raw[:limit]
	}
	out := make([]ReleaseCandidate, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.toReleaseCandidate())
	}
	return out, nil
}

// SonarrListReleases runs an interactive indexer search for one episode, or for
// one season of a series. This is a real search against every configured
// indexer, not a cached listing.
func SonarrListReleases(ctx context.Context, c *Client, episodeID, seriesID, seasonNumber *int, limit int) ([]ReleaseCandidate, error) {
	switch {
	case episodeID != nil:
		return listReleases(ctx, c, Query{"episodeId": itoa(*episodeID)}, limit)
	case seriesID != nil && seasonNumber != nil:
		return listReleases(ctx, c, Query{
			"seriesId": itoa(*seriesID), "seasonNumber": itoa(*seasonNumber),
		}, limit)
	default:
		return nil, fmt.Errorf("no search scope given; pass episodeId, or both seriesId and seasonNumber")
	}
}

// RadarrListReleases runs an interactive indexer search for one movie.
func RadarrListReleases(ctx context.Context, c *Client, movieID, limit int) ([]ReleaseCandidate, error) {
	return listReleases(ctx, c, Query{"movieId": itoa(movieID)}, limit)
}

// GrabRelease sends one release to a download client, bypassing the rejections
// an automatic search would have honoured.
func GrabRelease(ctx context.Context, c *Client, guid string, indexerID int) error {
	if guid == "" {
		return fmt.Errorf("no release guid given; pass guid and indexerId from the list_releases tool")
	}
	_, err := c.WithTimeout(searchTimeout).Post(ctx, "/release", struct {
		GUID      string `json:"guid"`
		IndexerID int    `json:"indexerId"`
	}{GUID: guid, IndexerID: indexerID})
	return err
}

// GrabQueueItem forces a pending queue item to be grabbed now, which is how a
// release held by a delay profile is released early.
func GrabQueueItem(ctx context.Context, c *Client, id int) error {
	_, err := c.WithTimeout(searchTimeout).Post(ctx, "/queue/grab/"+itoa(id), nil)
	return err
}

// MarkHistoryFailed marks a past grab as failed, which blocklists the release
// and lets the service search for a replacement.
func MarkHistoryFailed(ctx context.Context, c *Client, id int) error {
	_, err := c.Post(ctx, "/history/failed/"+itoa(id), nil)
	return err
}

// --- manual import ---

// ManualImportQuery scopes a manual import preview. Exactly one of Folder and
// DownloadID identifies what to inspect.
type ManualImportQuery struct {
	// Folder is a path on the service's filesystem to scan.
	Folder string
	// DownloadID is the download client's hash for a finished download.
	DownloadID string
	// SeriesID hints which series the files belong to (Sonarr).
	SeriesID *int
	// MovieID hints which movie the files belong to (Radarr).
	MovieID *int
	// FilterExistingFiles drops files already in the library.
	FilterExistingFiles *bool
}

// ManualImportCandidate is one file the service could import, with the match it
// guessed. A candidate with rejections and no series or movie needs the caller
// to supply the missing identity before it can be imported.
type ManualImportCandidate struct {
	ID           int      `json:"id"`
	Path         string   `json:"path" jsonschema:"pass back to the manual_import tool unchanged"`
	RelativePath string   `json:"relativePath,omitempty"`
	Size         int64    `json:"size,omitempty" jsonschema:"file size in bytes"`
	SeriesID     int      `json:"seriesId,omitempty"`
	SeriesTitle  string   `json:"seriesTitle,omitempty"`
	MovieID      int      `json:"movieId,omitempty"`
	MovieTitle   string   `json:"movieTitle,omitempty"`
	SeasonNumber *int     `json:"seasonNumber,omitempty"`
	EpisodeIDs   []int    `json:"episodeIds,omitempty"`
	Quality      string   `json:"quality,omitempty"`
	Languages    []string `json:"languages,omitempty"`
	ReleaseGroup string   `json:"releaseGroup,omitempty"`
	DownloadID   string   `json:"downloadId,omitempty"`
	Rejections   []string `json:"rejections,omitempty" jsonschema:"why this file cannot be imported as matched"`
}

// rawManualImportCandidate mirrors the upstream resource. Each row embeds the
// whole series or movie it matched, so those are decoded down to an id and a
// title rather than kept.
type rawManualImportCandidate struct {
	ID           int    `json:"id"`
	Path         string `json:"path"`
	RelativePath string `json:"relativePath"`
	Size         int64  `json:"size"`
	Series       *struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	} `json:"series"`
	Movie *struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	} `json:"movie"`
	SeasonNumber *int `json:"seasonNumber"`
	Episodes     []struct {
		ID int `json:"id"`
	} `json:"episodes"`
	Quality struct {
		Quality struct {
			Name string `json:"name"`
		} `json:"quality"`
	} `json:"quality"`
	Languages []struct {
		Name string `json:"name"`
	} `json:"languages"`
	ReleaseGroup string        `json:"releaseGroup"`
	DownloadID   string        `json:"downloadId"`
	Rejections   rejectionList `json:"rejections"`
}

// toCandidate projects an upstream candidate onto the trimmed view.
func (r rawManualImportCandidate) toCandidate() ManualImportCandidate {
	out := ManualImportCandidate{
		ID: r.ID, Path: r.Path, RelativePath: r.RelativePath, Size: r.Size,
		SeasonNumber: r.SeasonNumber, Quality: r.Quality.Quality.Name,
		ReleaseGroup: r.ReleaseGroup, DownloadID: r.DownloadID,
		Rejections: r.Rejections,
	}
	if r.Series != nil {
		out.SeriesID, out.SeriesTitle = r.Series.ID, r.Series.Title
	}
	if r.Movie != nil {
		out.MovieID, out.MovieTitle = r.Movie.ID, r.Movie.Title
	}
	for _, e := range r.Episodes {
		out.EpisodeIDs = append(out.EpisodeIDs, e.ID)
	}
	for _, l := range r.Languages {
		out.Languages = append(out.Languages, l.Name)
	}
	return out
}

// ListManualImportCandidates previews what a manual import would find, without
// importing anything.
func ListManualImportCandidates(ctx context.Context, c *Client, in ManualImportQuery) ([]ManualImportCandidate, error) {
	if in.Folder == "" && in.DownloadID == "" {
		return nil, fmt.Errorf("no scope given; pass folder, or downloadId from the queue tool")
	}
	q := Query{}
	if in.Folder != "" {
		q["folder"] = in.Folder
	}
	if in.DownloadID != "" {
		q["downloadId"] = in.DownloadID
	}
	if in.SeriesID != nil {
		q["seriesId"] = itoa(*in.SeriesID)
	}
	if in.MovieID != nil {
		q["movieId"] = itoa(*in.MovieID)
	}
	if in.FilterExistingFiles != nil {
		q["filterExistingFiles"] = btoa(*in.FilterExistingFiles)
	}

	raw, err := GetJSON[[]rawManualImportCandidate](ctx, c.WithTimeout(searchTimeout), "/manualimport", q)
	if err != nil {
		return nil, err
	}
	out := make([]ManualImportCandidate, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.toCandidate())
	}
	return out, nil
}

// ManualImportFile is one file to import and how to interpret it. Quality and
// Languages are names as the preview reports them, not ids.
type ManualImportFile struct {
	Path         string
	SeriesID     int
	EpisodeIDs   []int
	MovieID      int
	Quality      string
	Languages    []string
	ReleaseGroup string
	DownloadID   string
}

// importModes are the values the ManualImport command accepts.
var importModes = []string{"auto", "move", "copy"}

// ManualImport imports specific files into the library.
//
// This is the ManualImport *command*, not a POST to /manualimport: that route
// only reprocesses candidates and returns updated rows, so posting there would
// report success and leave every file exactly where it was.
func ManualImport(ctx context.Context, c *Client, files []ManualImportFile, importMode string) (CommandResult, error) {
	if len(files) == 0 {
		return CommandResult{}, fmt.Errorf("no files given; pass paths from the manual_import_preview tool")
	}

	mode := strings.ToLower(strings.TrimSpace(importMode))
	if mode == "" {
		mode = "auto"
	}
	if !oneOf(importModes, mode) {
		return CommandResult{}, fmt.Errorf("unknown import mode %q; valid modes: %s",
			importMode, strings.Join(importModes, ", "))
	}

	qualities, languages, err := importLookups(ctx, c, files)
	if err != nil {
		return CommandResult{}, err
	}

	payload := make([]map[string]any, 0, len(files))
	for _, f := range files {
		if f.Path == "" {
			return CommandResult{}, fmt.Errorf("a file was given with no path; pass paths from the manual_import_preview tool")
		}
		entry := map[string]any{"path": f.Path}
		if f.SeriesID > 0 {
			entry["seriesId"] = f.SeriesID
		}
		if len(f.EpisodeIDs) > 0 {
			entry["episodeIds"] = f.EpisodeIDs
		}
		if f.MovieID > 0 {
			entry["movieId"] = f.MovieID
		}
		if f.ReleaseGroup != "" {
			entry["releaseGroup"] = f.ReleaseGroup
		}
		if f.DownloadID != "" {
			entry["downloadId"] = f.DownloadID
		}
		if f.Quality != "" {
			quality, err := qualities.resolve(f.Quality)
			if err != nil {
				return CommandResult{}, err
			}
			entry["quality"] = map[string]any{"quality": quality}
		}
		if len(f.Languages) > 0 {
			resolved, err := languages.resolveAll(f.Languages)
			if err != nil {
				return CommandResult{}, err
			}
			entry["languages"] = resolved
		}
		payload = append(payload, entry)
	}

	return RunCommand(ctx, c, "ManualImport", map[string]any{
		"files": payload, "importMode": mode,
	})
}

// nameTable maps a lowercased name onto the {id,name} object the API wants,
// and remembers the names it knows so a miss can say what was available.
type nameTable struct {
	kind  string
	byKey map[string]map[string]any
	names []string
}

// resolve returns the reference object for one name.
func (t nameTable) resolve(name string) (map[string]any, error) {
	if ref, ok := t.byKey[strings.ToLower(strings.TrimSpace(name))]; ok {
		return ref, nil
	}
	return nil, fmt.Errorf("unknown %s %q; this instance offers: %s",
		t.kind, name, strings.Join(t.names, ", "))
}

// resolveAll returns the reference objects for a list of names.
func (t nameTable) resolveAll(names []string) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		ref, err := t.resolve(name)
		if err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, nil
}

// importLookups fetches the quality and language tables, but only the ones the
// files actually reference, so a fully-specified import costs no extra reads.
func importLookups(ctx context.Context, c *Client, files []ManualImportFile) (qualities, languages nameTable, err error) {
	var wantQuality, wantLanguage bool
	for _, f := range files {
		wantQuality = wantQuality || f.Quality != ""
		wantLanguage = wantLanguage || len(f.Languages) > 0
	}

	if wantQuality {
		// The id the API wants is the quality's own id, not the id of the
		// definition wrapping it: they differ (WEBDL-480p is definition 4 and
		// quality 8), and sending the definition id silently mislabels the file.
		raw, fetchErr := GetJSON[[]struct {
			Quality struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"quality"`
		}](ctx, c, "/qualitydefinition")
		if fetchErr != nil {
			return qualities, languages, fetchErr
		}
		qualities = nameTable{kind: "quality", byKey: map[string]map[string]any{}}
		for _, r := range raw {
			if r.Quality.Name == "" {
				continue
			}
			qualities.byKey[strings.ToLower(r.Quality.Name)] = map[string]any{
				"id": r.Quality.ID, "name": r.Quality.Name,
			}
			qualities.names = append(qualities.names, r.Quality.Name)
		}
		sort.Strings(qualities.names)
	}

	if wantLanguage {
		raw, fetchErr := GetJSON[[]struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}](ctx, c, "/language")
		if fetchErr != nil {
			return qualities, languages, fetchErr
		}
		languages = nameTable{kind: "language", byKey: map[string]map[string]any{}}
		for _, r := range raw {
			if r.Name == "" {
				continue
			}
			languages.byKey[strings.ToLower(r.Name)] = map[string]any{"id": r.ID, "name": r.Name}
			languages.names = append(languages.names, r.Name)
		}
		sort.Strings(languages.names)
	}

	return qualities, languages, nil
}

// oneOf reports whether list holds value.
func oneOf(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// --- detail views ---

// SeasonSummary is one season of a series with its file counts.
type SeasonSummary struct {
	SeasonNumber      int     `json:"seasonNumber" jsonschema:"0 is specials"`
	Monitored         bool    `json:"monitored"`
	EpisodeCount      int     `json:"episodeCount"`
	EpisodeFileCount  int     `json:"episodeFileCount"`
	TotalEpisodeCount int     `json:"totalEpisodeCount,omitempty" jsonschema:"including unaired episodes"`
	SizeOnDisk        int64   `json:"sizeOnDisk,omitempty"`
	PercentOfEpisodes float64 `json:"percentOfEpisodes,omitempty"`
}

// LibraryStats summarises how complete a series is.
type LibraryStats struct {
	SeasonCount       int     `json:"seasonCount,omitempty"`
	EpisodeCount      int     `json:"episodeCount"`
	EpisodeFileCount  int     `json:"episodeFileCount"`
	TotalEpisodeCount int     `json:"totalEpisodeCount,omitempty"`
	SizeOnDisk        int64   `json:"sizeOnDisk,omitempty"`
	PercentOfEpisodes float64 `json:"percentOfEpisodes,omitempty"`
}

// SeriesDetail is one series with the per-season breakdown the list view omits.
// The overview, artwork, ratings and alternate titles are all dropped.
type SeriesDetail struct {
	ID               int             `json:"id"`
	Title            string          `json:"title"`
	Year             int             `json:"year,omitempty"`
	Status           string          `json:"status,omitempty"`
	Monitored        bool            `json:"monitored"`
	TVDBID           int             `json:"tvdbId,omitempty"`
	Path             string          `json:"path,omitempty"`
	RootFolderPath   string          `json:"rootFolderPath,omitempty"`
	QualityProfileID int             `json:"qualityProfileId,omitempty"`
	SeriesType       string          `json:"seriesType,omitempty" jsonschema:"standard, daily or anime"`
	SeasonFolder     bool            `json:"seasonFolder"`
	MonitorNewItems  string          `json:"monitorNewItems,omitempty"`
	Network          string          `json:"network,omitempty"`
	Runtime          int             `json:"runtime,omitempty" jsonschema:"minutes per episode"`
	Added            string          `json:"added,omitempty"`
	Tags             []int           `json:"tags,omitempty"`
	Seasons          []SeasonSummary `json:"seasons,omitempty"`
	Statistics       LibraryStats    `json:"statistics"`
}

// rawSeriesDetail mirrors the upstream series resource before trimming.
type rawSeriesDetail struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	Year             int    `json:"year"`
	Status           string `json:"status"`
	Monitored        bool   `json:"monitored"`
	TVDBID           int    `json:"tvdbId"`
	Path             string `json:"path"`
	RootFolderPath   string `json:"rootFolderPath"`
	QualityProfileID int    `json:"qualityProfileId"`
	SeriesType       string `json:"seriesType"`
	SeasonFolder     bool   `json:"seasonFolder"`
	MonitorNewItems  string `json:"monitorNewItems"`
	Network          string `json:"network"`
	Runtime          int    `json:"runtime"`
	Added            string `json:"added"`
	Tags             []int  `json:"tags"`
	Seasons          []struct {
		SeasonNumber int          `json:"seasonNumber"`
		Monitored    bool         `json:"monitored"`
		Statistics   LibraryStats `json:"statistics"`
	} `json:"seasons"`
	Statistics LibraryStats `json:"statistics"`
}

// SonarrGetSeries returns one series with its seasons and library statistics.
func SonarrGetSeries(ctx context.Context, c *Client, id int) (SeriesDetail, error) {
	raw, err := GetJSON[rawSeriesDetail](ctx, c, "/series/"+itoa(id))
	if err != nil {
		return SeriesDetail{}, err
	}
	out := SeriesDetail{
		ID: raw.ID, Title: raw.Title, Year: raw.Year, Status: raw.Status,
		Monitored: raw.Monitored, TVDBID: raw.TVDBID, Path: raw.Path,
		RootFolderPath: raw.RootFolderPath, QualityProfileID: raw.QualityProfileID,
		SeriesType: raw.SeriesType, SeasonFolder: raw.SeasonFolder,
		MonitorNewItems: raw.MonitorNewItems, Network: raw.Network,
		Runtime: raw.Runtime, Added: raw.Added, Tags: raw.Tags,
		Statistics: raw.Statistics,
	}
	for _, s := range raw.Seasons {
		out.Seasons = append(out.Seasons, SeasonSummary{
			SeasonNumber:      s.SeasonNumber,
			Monitored:         s.Monitored,
			EpisodeCount:      s.Statistics.EpisodeCount,
			EpisodeFileCount:  s.Statistics.EpisodeFileCount,
			TotalEpisodeCount: s.Statistics.TotalEpisodeCount,
			SizeOnDisk:        s.Statistics.SizeOnDisk,
			PercentOfEpisodes: s.Statistics.PercentOfEpisodes,
		})
	}
	return out, nil
}

// MovieDetail is one movie with its file summary. As with the series view, the
// overview, artwork, ratings and alternate titles are dropped.
type MovieDetail struct {
	ID                  int        `json:"id"`
	Title               string     `json:"title"`
	Year                int        `json:"year,omitempty"`
	Status              string     `json:"status,omitempty"`
	Monitored           bool       `json:"monitored"`
	HasFile             bool       `json:"hasFile"`
	TMDBID              int        `json:"tmdbId,omitempty"`
	IMDBID              string     `json:"imdbId,omitempty"`
	Path                string     `json:"path,omitempty"`
	RootFolderPath      string     `json:"rootFolderPath,omitempty"`
	QualityProfileID    int        `json:"qualityProfileId,omitempty"`
	MinimumAvailability string     `json:"minimumAvailability,omitempty"`
	Runtime             int        `json:"runtime,omitempty" jsonschema:"minutes"`
	SizeOnDisk          int64      `json:"sizeOnDisk,omitempty"`
	IsAvailable         bool       `json:"isAvailable" jsonschema:"whether the movie has reached its minimum availability"`
	Added               string     `json:"added,omitempty"`
	Tags                []int      `json:"tags,omitempty"`
	CollectionTitle     string     `json:"collectionTitle,omitempty"`
	File                *MediaFile `json:"file,omitempty" jsonschema:"the file on disk, when there is one"`
}

// rawMovieDetail mirrors the upstream movie resource before trimming.
type rawMovieDetail struct {
	ID                  int           `json:"id"`
	Title               string        `json:"title"`
	Year                int           `json:"year"`
	Status              string        `json:"status"`
	Monitored           bool          `json:"monitored"`
	HasFile             bool          `json:"hasFile"`
	TMDBID              int           `json:"tmdbId"`
	IMDBID              string        `json:"imdbId"`
	Path                string        `json:"path"`
	RootFolderPath      string        `json:"rootFolderPath"`
	QualityProfileID    int           `json:"qualityProfileId"`
	MinimumAvailability string        `json:"minimumAvailability"`
	Runtime             int           `json:"runtime"`
	SizeOnDisk          int64         `json:"sizeOnDisk"`
	IsAvailable         bool          `json:"isAvailable"`
	Added               string        `json:"added"`
	Tags                []int         `json:"tags"`
	MovieFile           *rawMediaFile `json:"movieFile"`
	Collection          *struct {
		Title string `json:"title"`
	} `json:"collection"`
}

// RadarrGetMovie returns one movie with a summary of the file on disk.
func RadarrGetMovie(ctx context.Context, c *Client, id int) (MovieDetail, error) {
	raw, err := GetJSON[rawMovieDetail](ctx, c, "/movie/"+itoa(id))
	if err != nil {
		return MovieDetail{}, err
	}
	out := MovieDetail{
		ID: raw.ID, Title: raw.Title, Year: raw.Year, Status: raw.Status,
		Monitored: raw.Monitored, HasFile: raw.HasFile, TMDBID: raw.TMDBID,
		IMDBID: raw.IMDBID, Path: raw.Path, RootFolderPath: raw.RootFolderPath,
		QualityProfileID: raw.QualityProfileID, MinimumAvailability: raw.MinimumAvailability,
		Runtime: raw.Runtime, SizeOnDisk: raw.SizeOnDisk, IsAvailable: raw.IsAvailable,
		Added: raw.Added, Tags: raw.Tags,
	}
	if raw.MovieFile != nil {
		file := raw.MovieFile.toMediaFile()
		out.File = &file
	}
	if raw.Collection != nil {
		out.CollectionTitle = raw.Collection.Title
	}
	return out, nil
}

// --- collections ---

// CollectionUpdate describes a change to one Radarr collection. Every optional
// field is a pointer so an omitted argument stays absent from the request
// instead of resetting the setting to its zero value.
type CollectionUpdate struct {
	ID                  int
	Monitored           *bool
	QualityProfileID    *int
	RootFolderPath      *string
	SearchOnAdd         *bool
	MinimumAvailability *string
}

// RadarrUpdateCollection changes the settings applied to a TMDB collection and
// the movies added from it.
//
// The record is read back and written whole, decoded into a map rather than a
// struct on purpose: a typed round trip would drop every field this package
// does not model -- the collection's images, sort title and member list among
// them -- and silently reset them on the instance.
func RadarrUpdateCollection(ctx context.Context, c *Client, in CollectionUpdate) (Collection, error) {
	current, err := GetJSON[map[string]any](ctx, c, "/collection/"+itoa(in.ID))
	if err != nil {
		return Collection{}, err
	}
	if in.Monitored != nil {
		current["monitored"] = *in.Monitored
	}
	if in.QualityProfileID != nil {
		current["qualityProfileId"] = *in.QualityProfileID
	}
	if in.RootFolderPath != nil {
		current["rootFolderPath"] = *in.RootFolderPath
	}
	if in.SearchOnAdd != nil {
		current["searchOnAdd"] = *in.SearchOnAdd
	}
	if in.MinimumAvailability != nil {
		current["minimumAvailability"] = *in.MinimumAvailability
	}

	body, err := c.Put(ctx, "/collection/"+itoa(in.ID), current)
	if err != nil {
		return Collection{}, err
	}
	var raw rawCollection
	if err := unmarshal(body, &raw); err != nil {
		return Collection{}, err
	}
	return Collection{
		ID: raw.ID, Title: raw.Title, TMDBID: raw.TMDBID, Monitored: raw.Monitored,
		MovieCount: len(raw.Movies), MissingMovies: raw.MissingMovies,
		QualityProfileID: raw.QualityProfileID, RootFolderPath: raw.RootFolderPath,
		SearchOnAdd: raw.SearchOnAdd, MinimumAvailability: raw.MinimumAvailability,
		Tags: raw.Tags,
	}, nil
}

// --- renames and file edits ---

// renameFiles triggers the RenameFiles command for one series or movie.
func renameFiles(ctx context.Context, c *Client, key string, mediaID int, fileIDs []int) (CommandResult, error) {
	if len(fileIDs) == 0 {
		return CommandResult{}, fmt.Errorf("no file ids given; pass ids from the rename_preview tool")
	}
	return RunCommand(ctx, c, "RenameFiles", map[string]any{
		key: mediaID, "files": fileIDs,
	})
}

// SonarrRenameFiles renames episode files to match the naming config.
func SonarrRenameFiles(ctx context.Context, c *Client, seriesID int, fileIDs []int) (CommandResult, error) {
	return renameFiles(ctx, c, "seriesId", seriesID, fileIDs)
}

// RadarrRenameFiles renames movie files to match the naming config.
func RadarrRenameFiles(ctx context.Context, c *Client, movieID int, fileIDs []int) (CommandResult, error) {
	return renameFiles(ctx, c, "movieId", movieID, fileIDs)
}

// updateMediaFiles rewrites the quality, languages or release group recorded
// for a set of files.
//
// The bulk editor takes a bare array of partial file resources and applies only
// the members that are present, so every key the caller did not name has to
// stay absent from the body rather than be sent as a zero value.
func updateMediaFiles(ctx context.Context, c *Client, path string, fileIDs []int,
	quality *string, languages []string, releaseGroup *string) ([]MediaFile, error) {
	if len(fileIDs) == 0 {
		return nil, fmt.Errorf("no file ids given; pass ids from the file listing tool")
	}
	if quality == nil && len(languages) == 0 && releaseGroup == nil {
		return nil, fmt.Errorf("nothing to change; give quality, languages or releaseGroup")
	}

	var lookupFiles []ManualImportFile
	probe := ManualImportFile{Languages: languages}
	if quality != nil {
		probe.Quality = *quality
	}
	lookupFiles = append(lookupFiles, probe)

	qualities, langs, err := importLookups(ctx, c, lookupFiles)
	if err != nil {
		return nil, err
	}

	shared := map[string]any{}
	if quality != nil {
		ref, err := qualities.resolve(*quality)
		if err != nil {
			return nil, err
		}
		shared["quality"] = map[string]any{"quality": ref}
	}
	if len(languages) > 0 {
		resolved, err := langs.resolveAll(languages)
		if err != nil {
			return nil, err
		}
		shared["languages"] = resolved
	}
	if releaseGroup != nil {
		shared["releaseGroup"] = *releaseGroup
	}

	payload := make([]map[string]any, 0, len(fileIDs))
	for _, id := range fileIDs {
		entry := map[string]any{"id": id}
		for k, v := range shared {
			entry[k] = v
		}
		payload = append(payload, entry)
	}

	body, err := c.Put(ctx, path, payload)
	if err != nil {
		return nil, err
	}
	var raw []rawMediaFile
	if err := unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]MediaFile, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.toMediaFile())
	}
	return out, nil
}

// SonarrUpdateEpisodeFiles corrects the quality, languages or release group
// recorded for episode files.
func SonarrUpdateEpisodeFiles(ctx context.Context, c *Client, fileIDs []int,
	quality *string, languages []string, releaseGroup *string) ([]MediaFile, error) {
	return updateMediaFiles(ctx, c, "/episodeFile/bulk", fileIDs, quality, languages, releaseGroup)
}

// RadarrUpdateMovieFiles corrects the quality, languages or release group
// recorded for movie files.
func RadarrUpdateMovieFiles(ctx context.Context, c *Client, fileIDs []int,
	quality *string, languages []string, releaseGroup *string) ([]MediaFile, error) {
	return updateMediaFiles(ctx, c, "/movieFile/bulk", fileIDs, quality, languages, releaseGroup)
}

// --- tags and the queue ---

// UpdateTag renames an existing tag, keeping everything that carries it.
func UpdateTag(ctx context.Context, c *Client, id int, label string) (Tag, error) {
	if label == "" {
		return Tag{}, fmt.Errorf("no label given; a tag cannot be renamed to an empty string")
	}
	// The body carries the id as well as the route: the service rejects an
	// update whose body disagrees with the path.
	body, err := c.Put(ctx, "/tag/"+itoa(id), Tag{ID: id, Label: label})
	if err != nil {
		return Tag{}, err
	}
	var out Tag
	if err := unmarshal(body, &out); err != nil {
		return Tag{}, err
	}
	return out, nil
}

// DeleteQueueItems removes several downloads from the queue in one call and
// reports how many were removed.
//
// The bulk route takes the ids in a request body and the flags in the query
// string, which is why this reaches past the Delete helper.
func DeleteQueueItems(ctx context.Context, c *Client, ids []int, removeFromClient, blocklist bool) (int, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("no queue ids given; pass ids from the queue tool")
	}
	_, err := c.do(ctx, http.MethodDelete, "/queue/bulk",
		struct {
			IDs []int `json:"ids"`
		}{IDs: ids},
		Query{
			"removeFromClient": btoa(removeFromClient),
			"blocklist":        btoa(blocklist),
		})
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

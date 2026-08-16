package server

import (
	"context"
	"fmt"

	"github.com/GauranshMathur/ARR_MCP/pkg/arr"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// InstanceArg is embedded in every tool input to select which configured
// instance of a service the call targets.
type InstanceArg struct {
	Instance string `json:"instance,omitempty" jsonschema:"which configured instance to use; omit to use the default"`
}

// instanceName reports the selected instance, if any.
func (a InstanceArg) instanceName() string { return a.Instance }

// instanceSelector is satisfied by any tool input embedding InstanceArg.
type instanceSelector interface{ instanceName() string }

// toolMeta describes a tool independently of its handler.
type toolMeta struct {
	name        string
	description string
	access      Access
}

// register adds one tool, wiring instance resolution, permission gating and
// client construction around the service call in fn.
func register[In instanceSelector, Out any](
	s *Server, service string, spec arr.ServiceSpec, meta toolMeta,
	fn func(context.Context, *arr.Client, In) (Out, error),
) {
	if !s.registersForService(service, meta.access) {
		return
	}

	schema, err := jsonschema.For[In](nil)
	if err != nil {
		s.log.Error("building schema for %s: %v", meta.name, err)
		return
	}
	// Advertise the configured instance names so the model picks from a
	// closed set instead of guessing.
	if prop, ok := schema.Properties["instance"]; ok {
		names := s.cfg.InstanceNames(service)
		enum := make([]any, 0, len(names))
		for _, n := range names {
			enum = append(enum, n)
		}
		prop.Enum = enum
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        meta.name,
		Description: meta.description,
		Annotations: meta.access.Annotations(),
		InputSchema: schema,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		var zero Out

		inst, err := s.cfg.Resolve(service, in.instanceName())
		if err != nil {
			return nil, zero, err
		}
		if err := s.gateFor(inst).Authorize(ctx, sessionConfirmer{req.Session}, meta.name, meta.access); err != nil {
			return nil, zero, err
		}

		client := arr.NewClient(inst.URL, spec, arr.Credentials{APIKey: inst.APIKey})
		out, err := fn(ctx, client, in)
		if err != nil {
			return nil, zero, fmt.Errorf("%s (%s instance %q): %w", meta.name, service, inst.Name, err)
		}
		return nil, out, nil
	})
}

// --- tool input types ---

// EmptyArgs is the input for tools that only need an instance.
type EmptyArgs struct{ InstanceArg }

// SearchArgs is the input for title lookup tools.
type SearchArgs struct {
	InstanceArg
	Query string `json:"query" jsonschema:"title to search for"`
}

// AddSeriesArgs is the input for sonarr_add_series.
type AddSeriesArgs struct {
	InstanceArg
	TVDBID           int    `json:"tvdbId" jsonschema:"TheTVDB id from sonarr_search_series"`
	Title            string `json:"title,omitempty"`
	QualityProfileID int    `json:"qualityProfileId" jsonschema:"id from sonarr_list_quality_profiles"`
	RootFolderPath   string `json:"rootFolderPath" jsonschema:"path from sonarr_list_root_folders"`
	SearchNow        bool   `json:"searchNow,omitempty" jsonschema:"start searching for episodes immediately"`
}

// AddMovieArgs is the input for radarr_add_movie.
type AddMovieArgs struct {
	InstanceArg
	TMDBID           int    `json:"tmdbId" jsonschema:"TMDB id from radarr_search_movies"`
	Title            string `json:"title,omitempty"`
	QualityProfileID int    `json:"qualityProfileId" jsonschema:"id from radarr_list_quality_profiles"`
	RootFolderPath   string `json:"rootFolderPath" jsonschema:"path from radarr_list_root_folders"`
	SearchNow        bool   `json:"searchNow,omitempty" jsonschema:"start searching for the movie immediately"`
}

// DeleteArgs is the input for deletion tools.
type DeleteArgs struct {
	InstanceArg
	ID          int  `json:"id" jsonschema:"internal id of the item to remove"`
	DeleteFiles bool `json:"deleteFiles,omitempty" jsonschema:"also delete downloaded files from disk"`
}

// ProwlarrSearchArgs is the input for prowlarr_search.
type ProwlarrSearchArgs struct {
	InstanceArg
	Query      string `json:"query" jsonschema:"search term"`
	Categories []int  `json:"categories,omitempty" jsonschema:"newznab category ids to filter by"`
	Limit      int    `json:"limit,omitempty" jsonschema:"maximum results to return; defaults to 25"`
}

// --- tool output types ---

// SeriesList wraps series results.
type SeriesList struct {
	Series []arr.Series `json:"series"`
	Count  int          `json:"count"`
}

// MovieList wraps movie results. Total is set only by the paged tools, where
// the page is capped and the count alone would understate the library.
type MovieList struct {
	Movies []arr.Movie `json:"movies"`
	Count  int         `json:"count"`
	Total  int         `json:"total,omitempty" jsonschema:"records available across all pages"`
}

// ProfileList wraps quality profile results.
type ProfileList struct {
	Profiles []arr.QualityProfile `json:"profiles"`
}

// FolderList wraps root folder results.
type FolderList struct {
	Folders []arr.RootFolder `json:"folders"`
}

// IndexerList wraps indexer results.
type IndexerList struct {
	Indexers []arr.Indexer `json:"indexers"`
	Count    int           `json:"count"`
}

// ReleaseList wraps indexer search results.
type ReleaseList struct {
	Releases []arr.SearchResult `json:"releases"`
	Count    int                `json:"count"`
}

// Deleted reports the outcome of a deletion.
type Deleted struct {
	ID      int  `json:"id"`
	Deleted bool `json:"deleted"`
}

// --- shared tool inputs ---

// LimitArgs is the input for paged listing tools.
type LimitArgs struct {
	InstanceArg
	Limit int `json:"limit,omitempty" jsonschema:"maximum records to return; defaults to 20"`
}

// DeleteQueueArgs is the input for queue removal tools.
type DeleteQueueArgs struct {
	InstanceArg
	ID               int  `json:"id" jsonschema:"queue item id from the queue tool"`
	RemoveFromClient bool `json:"removeFromClient,omitempty" jsonschema:"also remove the download from the download client"`
	Blocklist        bool `json:"blocklist,omitempty" jsonschema:"blocklist the release so it is not grabbed again"`
}

// CommandArgs is the input for triggering a background command.
type CommandArgs struct {
	InstanceArg
	Name string `json:"name" jsonschema:"command name, e.g. RefreshSeries, RssSync, Backup"`
}

// CalendarArgs is the input for calendar tools.
type CalendarArgs struct {
	InstanceArg
	Start string `json:"start,omitempty" jsonschema:"start date as YYYY-MM-DD"`
	End   string `json:"end,omitempty" jsonschema:"end date as YYYY-MM-DD"`
}

// EpisodesArgs is the input for listing a series' episodes.
type EpisodesArgs struct {
	InstanceArg
	SeriesID int `json:"seriesId" jsonschema:"series id from sonarr_list_series"`
}

// --- shared tool outputs ---

// HealthList wraps health check results.
type HealthList struct {
	Issues []arr.HealthIssue `json:"issues"`
	Count  int               `json:"count"`
}

// DiskSpaceList wraps disk space results.
type DiskSpaceList struct {
	Disks []arr.DiskSpace `json:"disks"`
}

// QueueList wraps download queue results.
type QueueList struct {
	Items []arr.QueueItem `json:"items"`
	Count int             `json:"count"`
}

// HistoryList wraps history results.
type HistoryList struct {
	Records []arr.HistoryRecord `json:"records"`
	Count   int                 `json:"count"`
}

// EpisodeList wraps episode results. Total is set only by the paged tools,
// where the page is capped and the count alone would understate the library.
type EpisodeList struct {
	Episodes []arr.Episode `json:"episodes"`
	Count    int           `json:"count"`
	Total    int           `json:"total,omitempty" jsonschema:"records available across all pages"`
}

// IndexerStatList wraps Prowlarr indexer statistics.
type IndexerStatList struct {
	Stats []arr.IndexerStat `json:"stats"`
}

// --- bazarr tool inputs ---

// PageArgs is the input for Bazarr's start/length paged listings.
type PageArgs struct {
	InstanceArg
	Start  int `json:"start,omitempty" jsonschema:"offset into the result set; defaults to 0"`
	Length int `json:"length,omitempty" jsonschema:"maximum records to return; defaults to 50"`
}

// LanguagesArgs is the input for listing Bazarr languages.
type LanguagesArgs struct {
	InstanceArg
	All bool `json:"all,omitempty" jsonschema:"return every ISO language instead of only the enabled ones"`
}

// EpisodeSubtitleArgs identifies one episode subtitle language to search for.
type EpisodeSubtitleArgs struct {
	InstanceArg
	SeriesID  int    `json:"seriesId" jsonschema:"sonarrSeriesId from bazarr_wanted_episodes"`
	EpisodeID int    `json:"episodeId" jsonschema:"sonarrEpisodeId from bazarr_wanted_episodes"`
	Language  string `json:"language" jsonschema:"two-letter language code, e.g. en"`
	Forced    bool   `json:"forced,omitempty"`
	HI        bool   `json:"hi,omitempty" jsonschema:"hearing impaired subtitle"`
}

// MovieSubtitleArgs identifies one movie subtitle language to search for.
type MovieSubtitleArgs struct {
	InstanceArg
	RadarrID int    `json:"radarrId" jsonschema:"radarrId from bazarr_wanted_movies"`
	Language string `json:"language" jsonschema:"two-letter language code, e.g. en"`
	Forced   bool   `json:"forced,omitempty"`
	HI       bool   `json:"hi,omitempty" jsonschema:"hearing impaired subtitle"`
}

// DeleteEpisodeSubtitleArgs identifies an existing subtitle file to remove.
// Path is required and comes from bazarr_list_episode_subtitles.
type DeleteEpisodeSubtitleArgs struct {
	InstanceArg
	SeriesID  int    `json:"seriesId" jsonschema:"sonarrSeriesId of the episode"`
	EpisodeID int    `json:"episodeId" jsonschema:"sonarrEpisodeId of the episode"`
	Language  string `json:"language" jsonschema:"two-letter language code of the subtitle to delete"`
	Path      string `json:"path" jsonschema:"subtitle file path from bazarr_list_episode_subtitles"`
	Forced    bool   `json:"forced,omitempty"`
	HI        bool   `json:"hi,omitempty" jsonschema:"hearing impaired subtitle"`
}

// DeleteMovieSubtitleArgs identifies an existing movie subtitle file to remove.
type DeleteMovieSubtitleArgs struct {
	InstanceArg
	RadarrID int    `json:"radarrId" jsonschema:"radarrId of the movie"`
	Language string `json:"language" jsonschema:"two-letter language code of the subtitle to delete"`
	Path     string `json:"path" jsonschema:"subtitle file path of the subtitle to delete"`
	Forced   bool   `json:"forced,omitempty"`
	HI       bool   `json:"hi,omitempty" jsonschema:"hearing impaired subtitle"`
}

// SeriesSubtitlesArgs selects a series whose per-episode subtitles to list.
type SeriesSubtitlesArgs struct {
	InstanceArg
	SeriesID int `json:"seriesId" jsonschema:"sonarrSeriesId from bazarr_list_series"`
}

// --- bazarr tool outputs ---

// WantedEpisodeList wraps episodes missing subtitles.
type WantedEpisodeList struct {
	Episodes []arr.WantedEpisode `json:"episodes"`
	Returned int                 `json:"returned"`
	Total    int                 `json:"total" jsonschema:"total missing across the whole library"`
}

// WantedMovieList wraps movies missing subtitles.
type WantedMovieList struct {
	Movies   []arr.WantedMovie `json:"movies"`
	Returned int               `json:"returned"`
	Total    int               `json:"total"`
}

// BazarrSeriesList wraps a page of Bazarr's series view.
type BazarrSeriesList struct {
	Series   []arr.BazarrSeries `json:"series"`
	Returned int                `json:"returned"`
	Total    int                `json:"total" jsonschema:"total series in the library, which may exceed those returned"`
}

// BazarrMovieList wraps a page of Bazarr's movie view.
type BazarrMovieList struct {
	Movies   []arr.BazarrMovie `json:"movies"`
	Returned int               `json:"returned"`
	Total    int               `json:"total" jsonschema:"total movies in the library, which may exceed those returned"`
}

// BazarrHealthList wraps Bazarr health issues, which use their own shape.
type BazarrHealthList struct {
	Issues []arr.BazarrHealthIssue `json:"issues"`
	Count  int                     `json:"count"`
}

// EpisodeSubtitlesList wraps per-episode subtitle state.
type EpisodeSubtitlesList struct {
	Episodes []arr.EpisodeSubtitles `json:"episodes"`
	Count    int                    `json:"count"`
}

// ProviderList wraps subtitle provider status.
type ProviderList struct {
	Providers []arr.SubtitleProvider `json:"providers"`
}

// LanguageList wraps subtitle languages.
type LanguageList struct {
	Languages []arr.SubtitleLanguage `json:"languages"`
}

// StatusMap wraps a free-form status payload.
type StatusMap struct {
	Status map[string]any `json:"status"`
}

// Requested reports that a background request was accepted.
type Requested struct {
	Requested bool   `json:"requested"`
	Detail    string `json:"detail,omitempty"`
}

// --- media service tool inputs ---

// LabelArgs is the input for tag creation.
type LabelArgs struct {
	InstanceArg
	Label string `json:"label" jsonschema:"tag label, e.g. kids or 4k"`
}

// IDArgs is the input for tools that act on a single record by id.
type IDArgs struct {
	InstanceArg
	ID int `json:"id" jsonschema:"internal id of the record"`
}

// --- media service tool outputs ---

// TagList wraps tag results.
type TagList struct {
	Tags  []arr.Tag `json:"tags"`
	Count int       `json:"count"`
}

// TagDetailList wraps tag usage results.
type TagDetailList struct {
	Tags  []arr.TagDetail `json:"tags"`
	Count int             `json:"count"`
}

// CustomFormatList wraps custom format results.
type CustomFormatList struct {
	Formats []arr.CustomFormat `json:"formats"`
	Count   int                `json:"count"`
}

// DelayProfileList wraps delay profile results.
type DelayProfileList struct {
	Profiles []arr.DelayProfile `json:"profiles"`
	Count    int                `json:"count"`
}

// ReleaseProfileList wraps release profile results.
type ReleaseProfileList struct {
	Profiles []arr.ReleaseProfile `json:"profiles"`
	Count    int                  `json:"count"`
}

// ProviderList wraps indexer, download client, import list and notification
// results, which all share the upstream provider shape.
type ProviderList struct {
	Providers []arr.Provider `json:"providers"`
	Count     int            `json:"count"`
}

// QualityDefinitionList wraps quality size limit results.
type QualityDefinitionList struct {
	Definitions []arr.QualityDefinition `json:"definitions"`
	Count       int                     `json:"count"`
}

// EditSeriesArgs is the input for sonarr_edit_series. Optional fields are
// pointers so an omitted argument stays absent from the upstream request
// instead of resetting the setting to its zero value.
type EditSeriesArgs struct {
	InstanceArg
	SeriesIDs        []int  `json:"seriesIds" jsonschema:"series ids from sonarr_list_series"`
	Monitored        *bool  `json:"monitored,omitempty" jsonschema:"monitor or unmonitor the series"`
	QualityProfileID *int   `json:"qualityProfileId,omitempty" jsonschema:"id from sonarr_list_quality_profiles"`
	SeasonFolder     *bool  `json:"seasonFolder,omitempty"`
	RootFolderPath   string `json:"rootFolderPath,omitempty" jsonschema:"path from sonarr_list_root_folders"`
	SeriesType       string `json:"seriesType,omitempty" jsonschema:"standard, daily or anime"`
	MonitorNewItems  string `json:"monitorNewItems,omitempty" jsonschema:"all or none"`
	Tags             []int  `json:"tags,omitempty" jsonschema:"tag ids from sonarr_list_tags"`
	ApplyTags        string `json:"applyTags,omitempty" jsonschema:"how to apply tags: add, remove or replace; defaults to add"`
	MoveFiles        bool   `json:"moveFiles,omitempty" jsonschema:"move files on disk when rootFolderPath changes"`
}

// EditMoviesArgs is the input for radarr_edit_movies.
type EditMoviesArgs struct {
	InstanceArg
	MovieIDs            []int  `json:"movieIds" jsonschema:"movie ids from radarr_list_movies"`
	Monitored           *bool  `json:"monitored,omitempty" jsonschema:"monitor or unmonitor the movies"`
	QualityProfileID    *int   `json:"qualityProfileId,omitempty" jsonschema:"id from radarr_list_quality_profiles"`
	MinimumAvailability string `json:"minimumAvailability,omitempty" jsonschema:"tba, announced, inCinemas or released"`
	RootFolderPath      string `json:"rootFolderPath,omitempty" jsonschema:"path from radarr_list_root_folders"`
	Tags                []int  `json:"tags,omitempty" jsonschema:"tag ids from radarr_list_tags"`
	ApplyTags           string `json:"applyTags,omitempty" jsonschema:"how to apply tags: add, remove or replace; defaults to add"`
	MoveFiles           bool   `json:"moveFiles,omitempty" jsonschema:"move files on disk when rootFolderPath changes"`
}

// SeasonMonitorArgs is the input for sonarr_set_season_monitored.
type SeasonMonitorArgs struct {
	InstanceArg
	SeriesID     int  `json:"seriesId" jsonschema:"series id from sonarr_list_series"`
	SeasonNumber int  `json:"seasonNumber" jsonschema:"season number; 0 is specials"`
	Monitored    bool `json:"monitored" jsonschema:"true to monitor the season, false to unmonitor it"`
}

// EpisodeMonitorArgs is the input for sonarr_monitor_episodes.
type EpisodeMonitorArgs struct {
	InstanceArg
	EpisodeIDs []int `json:"episodeIds" jsonschema:"episode ids from sonarr_list_episodes"`
	Monitored  bool  `json:"monitored" jsonschema:"true to monitor the episodes, false to unmonitor them"`
}

// Updated reports how many records a bulk edit touched.
type Updated struct {
	Updated int `json:"updated"`
}

// MovieArgs is the input for tools scoped to one movie.
type MovieArgs struct {
	InstanceArg
	MovieID int `json:"movieId" jsonschema:"movie id from radarr_list_movies"`
}

// FileIDsArgs is the input for file deletion.
type FileIDsArgs struct {
	InstanceArg
	FileIDs []int `json:"fileIds" jsonschema:"file ids from the file listing tool"`
}

// SeriesRenameArgs is the input for sonarr_rename_preview.
type SeriesRenameArgs struct {
	InstanceArg
	SeriesID     int  `json:"seriesId" jsonschema:"series id from sonarr_list_series"`
	SeasonNumber *int `json:"seasonNumber,omitempty" jsonschema:"limit the preview to one season; omit for the whole series"`
}

// MediaFileList wraps episode and movie file results.
type MediaFileList struct {
	Files []arr.MediaFile `json:"files"`
	Count int             `json:"count"`
}

// RenamePreviewList wraps rename preview results.
type RenamePreviewList struct {
	Renames []arr.RenamePreview `json:"renames"`
	Count   int                 `json:"count"`
}

// DeletedCount reports how many records a bulk deletion removed. A partial
// failure reports the count reached before the error, because deleted files do
// not come back and the caller must not blindly retry the whole list.
type DeletedCount struct {
	Deleted int `json:"deleted"`
}

// SearchScopeArgs is the input for sonarr_trigger_search.
type SearchScopeArgs struct {
	InstanceArg
	SeriesID     int   `json:"seriesId" jsonschema:"series id from sonarr_list_series"`
	SeasonNumber *int  `json:"seasonNumber,omitempty" jsonschema:"search one season only"`
	EpisodeIDs   []int `json:"episodeIds,omitempty" jsonschema:"search specific episodes only; overrides seriesId and seasonNumber"`
}

// MovieIDsArgs is the input for tools acting on several movies.
type MovieIDsArgs struct {
	InstanceArg
	MovieIDs []int `json:"movieIds" jsonschema:"movie ids from radarr_list_movies"`
}

// BlocklistList wraps blocklist results.
type BlocklistList struct {
	Items []arr.BlocklistItem `json:"items"`
	Count int                 `json:"count"`
	Total int                 `json:"total" jsonschema:"releases blocklisted across all pages"`
}

// TaskList wraps scheduled task results.
type TaskList struct {
	Tasks []arr.Task `json:"tasks"`
	Count int        `json:"count"`
}

// UpdateList wraps release results.
type UpdateList struct {
	Updates []arr.UpdatePackage `json:"updates"`
	Count   int                 `json:"count"`
}

// CollectionList wraps Radarr collection results.
type CollectionList struct {
	Collections []arr.Collection `json:"collections"`
	Count       int              `json:"count"`
}

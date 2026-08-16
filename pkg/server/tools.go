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

// MovieList wraps movie results.
type MovieList struct {
	Movies []arr.Movie `json:"movies"`
	Count  int         `json:"count"`
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

// EpisodeList wraps episode results.
type EpisodeList struct {
	Episodes []arr.Episode `json:"episodes"`
	Count    int           `json:"count"`
}

// IndexerStatList wraps Prowlarr indexer statistics.
type IndexerStatList struct {
	Stats []arr.IndexerStat `json:"stats"`
}

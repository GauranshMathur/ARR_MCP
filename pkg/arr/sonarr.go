package arr

import (
	"context"
	"fmt"
	"strings"
)

// Series is the trimmed view of a Sonarr series returned to MCP clients.
//
// Sonarr's /api/v3/series objects carry ~40 fields each, including nested
// `seasons`, `images`, `ratings` and `statistics` blocks. A library of 200
// shows serialised in full is well over a megabyte, which would swamp the
// model's context on a single list call and crowd out the actual conversation.
// So we project down to the fields a user is plausibly asking about.
//
// TODO(field-selection): decide the exact field set. See ARR-MCP notes.
type Series struct {
	ID        int    `json:"id" jsonschema:"Sonarr's internal series id, used by other tools"`
	Title     string `json:"title"`
	Year      int    `json:"year,omitempty"`
	Status    string `json:"status,omitempty" jsonschema:"continuing, ended, or upcoming"`
	Monitored bool   `json:"monitored"`
	TVDBID    int    `json:"tvdbId,omitempty" jsonschema:"TheTVDB id, required when adding the series"`
}

// rawSeries mirrors the upstream Sonarr payload we care about before trimming.
type rawSeries struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Year      int    `json:"year"`
	Status    string `json:"status"`
	Monitored bool   `json:"monitored"`
	TVDBID    int    `json:"tvdbId"`
}

// toSeries projects an upstream payload onto the trimmed view.
func (r rawSeries) toSeries() Series {
	return Series(r)
}

// SonarrListSeries returns every series in the library.
func SonarrListSeries(ctx context.Context, c *Client) ([]Series, error) {
	raw, err := GetJSON[[]rawSeries](ctx, c, "/series")
	if err != nil {
		return nil, err
	}
	return trimSeries(raw), nil
}

// SonarrLookupSeries searches configured indexers for series matching term.
func SonarrLookupSeries(ctx context.Context, c *Client, term string) ([]Series, error) {
	raw, err := GetJSON[[]rawSeries](ctx, c, "/series/lookup", Query{"term": term})
	if err != nil {
		return nil, err
	}
	return trimSeries(raw), nil
}

// trimSeries projects a batch of upstream payloads.
func trimSeries(raw []rawSeries) []Series {
	out := make([]Series, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.toSeries())
	}
	return out
}

// AddSeriesRequest describes a series to add to a Sonarr library. SeasonFolder
// is a pointer so that omitting it leaves the service's own default in place
// rather than forcing a flat folder layout.
type AddSeriesRequest struct {
	TVDBID           int    `json:"tvdbId"`
	Title            string `json:"title"`
	QualityProfileID int    `json:"qualityProfileId"`
	RootFolderPath   string `json:"rootFolderPath"`
	Monitored        bool   `json:"monitored"`
	SeasonFolder     *bool  `json:"seasonFolder,omitempty"`
	SeriesType       string `json:"seriesType,omitempty" jsonschema:"standard, daily or anime"`
	Tags             []int  `json:"tags,omitempty"`
	AddOptions       struct {
		SearchForMissingEpisodes bool `json:"searchForMissingEpisodes"`
		// Monitor selects which episodes to monitor on add: all, future,
		// missing, existing, firstSeason, lastSeason or none.
		Monitor string `json:"monitor,omitempty"`
	} `json:"addOptions"`
}

// SonarrAddSeries adds a series to the library and returns the created record.
func SonarrAddSeries(ctx context.Context, c *Client, req AddSeriesRequest) (Series, error) {
	body, err := c.Post(ctx, "/series", req)
	if err != nil {
		return Series{}, err
	}
	var raw rawSeries
	if err := unmarshal(body, &raw); err != nil {
		return Series{}, err
	}
	return raw.toSeries(), nil
}

// SonarrDeleteSeries removes a series, optionally deleting its files.
func SonarrDeleteSeries(ctx context.Context, c *Client, id int, deleteFiles bool) error {
	_, err := c.Delete(ctx, "/series/"+itoa(id), Query{"deleteFiles": btoa(deleteFiles)})
	return err
}

// SeriesEditRequest describes a bulk change to one or more series. Every field
// but SeriesIDs is optional upstream, so the optional ones are pointers or use
// omitempty: sending a zero value would reset a setting the caller never named.
type SeriesEditRequest struct {
	SeriesIDs        []int  `json:"seriesIds"`
	Monitored        *bool  `json:"monitored,omitempty"`
	QualityProfileID *int   `json:"qualityProfileId,omitempty"`
	SeasonFolder     *bool  `json:"seasonFolder,omitempty"`
	RootFolderPath   string `json:"rootFolderPath,omitempty"`
	SeriesType       string `json:"seriesType,omitempty" jsonschema:"standard, daily or anime"`
	MonitorNewItems  string `json:"monitorNewItems,omitempty" jsonschema:"all or none"`
	Tags             []int  `json:"tags,omitempty"`
	ApplyTags        string `json:"applyTags,omitempty" jsonschema:"add, remove or replace"`
	MoveFiles        bool   `json:"moveFiles"`
}

// SonarrEditSeries applies a change to a set of series at once. This is how
// monitoring, quality profiles, tags and the root folder are changed: the
// endpoint takes a partial resource, so nothing the caller omits is touched.
func SonarrEditSeries(ctx context.Context, c *Client, req SeriesEditRequest) ([]Series, error) {
	if len(req.Tags) > 0 && req.ApplyTags == "" {
		// "add" is the only default that cannot remove a tag by surprise.
		req.ApplyTags = "add"
	}
	body, err := c.Put(ctx, "/series/editor", req)
	if err != nil {
		return nil, err
	}
	var raw []rawSeries
	if err := unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return trimSeries(raw), nil
}

// SonarrMonitorEpisodes monitors or unmonitors specific episodes.
func SonarrMonitorEpisodes(ctx context.Context, c *Client, episodeIDs []int, monitored bool) error {
	if len(episodeIDs) == 0 {
		return fmt.Errorf("no episode ids given; pass the ids from sonarr_list_episodes")
	}
	_, err := c.Put(ctx, "/episode/monitor", struct {
		EpisodeIDs []int `json:"episodeIds"`
		Monitored  bool  `json:"monitored"`
	}{EpisodeIDs: episodeIDs, Monitored: monitored})
	return err
}

// SonarrSetSeasonMonitored monitors or unmonitors one season of a series.
//
// There is no endpoint for a single season: seasons live inside the series
// resource, so the record is read back, the one season edited, and the whole
// thing written again. It is decoded into a map rather than a struct on
// purpose — a typed round trip would drop every field this package does not
// model and silently reset it on the instance.
func SonarrSetSeasonMonitored(ctx context.Context, c *Client, seriesID, seasonNumber int, monitored bool) (Series, error) {
	current, err := GetJSON[map[string]any](ctx, c, "/series/"+itoa(seriesID))
	if err != nil {
		return Series{}, err
	}

	seasons, ok := current["seasons"].([]any)
	if !ok {
		return Series{}, fmt.Errorf("series %d has no seasons to monitor", seriesID)
	}

	found := false
	available := make([]string, 0, len(seasons))
	for _, entry := range seasons {
		season, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		number, ok := season["seasonNumber"].(float64)
		if !ok {
			continue
		}
		available = append(available, itoa(int(number)))
		if int(number) == seasonNumber {
			season["monitored"] = monitored
			found = true
		}
	}
	if !found {
		return Series{}, fmt.Errorf("series %d has no season %d; available seasons: %s",
			seriesID, seasonNumber, strings.Join(available, ", "))
	}

	body, err := c.Put(ctx, "/series/"+itoa(seriesID), current)
	if err != nil {
		return Series{}, err
	}
	var raw rawSeries
	if err := unmarshal(body, &raw); err != nil {
		return Series{}, err
	}
	return raw.toSeries(), nil
}

// SonarrListEpisodeFiles returns the files on disk for one series.
func SonarrListEpisodeFiles(ctx context.Context, c *Client, seriesID int) ([]MediaFile, error) {
	return listMediaFiles(ctx, c, "/episodefile", Query{"seriesId": itoa(seriesID)})
}

// SonarrDeleteEpisodeFiles deletes episode files from disk and returns how many
// were removed. This cannot be undone.
func SonarrDeleteEpisodeFiles(ctx context.Context, c *Client, ids []int) (int, error) {
	return deleteFiles(ctx, c, "/episodefile", ids, "episode file")
}

// SonarrRenamePreview lists the files whose names do not match the naming
// config, and what they would be renamed to. A nil season covers the series.
func SonarrRenamePreview(ctx context.Context, c *Client, seriesID int, seasonNumber *int) ([]RenamePreview, error) {
	q := Query{"seriesId": itoa(seriesID)}
	if seasonNumber != nil {
		q["seasonNumber"] = itoa(*seasonNumber)
	}
	return GetJSON[[]RenamePreview](ctx, c, "/rename", q)
}

// SonarrWantedMissing returns monitored episodes that have aired but have no
// file, plus the total number missing across the whole library.
func SonarrWantedMissing(ctx context.Context, c *Client, pageSize int) ([]Episode, int, error) {
	return sonarrWanted(ctx, c, "/wanted/missing", pageSize)
}

// SonarrWantedCutoff returns monitored episodes whose file is below the quality
// cutoff, plus the total number across the library.
func SonarrWantedCutoff(ctx context.Context, c *Client, pageSize int) ([]Episode, int, error) {
	return sonarrWanted(ctx, c, "/wanted/cutoff", pageSize)
}

// sonarrWanted fetches one of the paged wanted lists. Episode already omits the
// overview these records carry, so the projection happens by decoding.
func sonarrWanted(ctx context.Context, c *Client, path string, pageSize int) ([]Episode, int, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	env, err := GetJSON[paged[Episode]](ctx, c, path, Query{"pageSize": itoa(pageSize)})
	if err != nil {
		return nil, 0, err
	}
	return env.Records, env.TotalRecords, nil
}

// SonarrTriggerSearch starts an indexer search for a series, one of its seasons
// or a specific set of episodes, whichever the arguments describe. The three
// scopes are three different commands upstream, taking different parameters.
func SonarrTriggerSearch(ctx context.Context, c *Client, seriesID int, seasonNumber *int, episodeIDs []int) (CommandResult, error) {
	switch {
	case len(episodeIDs) > 0:
		// EpisodeSearch ignores the series entirely; sending one would only be
		// ambiguous about which scope the service honours.
		return RunCommand(ctx, c, "EpisodeSearch", map[string]any{"episodeIds": episodeIDs})
	case seasonNumber != nil:
		return RunCommand(ctx, c, "SeasonSearch", map[string]any{
			"seriesId": seriesID, "seasonNumber": *seasonNumber,
		})
	default:
		return RunCommand(ctx, c, "SeriesSearch", map[string]any{"seriesId": seriesID})
	}
}

// SonarrRefreshSeries rescans one series' metadata and files.
func SonarrRefreshSeries(ctx context.Context, c *Client, seriesID int) (CommandResult, error) {
	return RunCommand(ctx, c, "RefreshSeries", map[string]any{"seriesId": seriesID})
}

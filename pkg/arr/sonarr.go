package arr

import "context"

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
	return Series{
		ID: r.ID, Title: r.Title, Year: r.Year,
		Status: r.Status, Monitored: r.Monitored, TVDBID: r.TVDBID,
	}
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

// AddSeriesRequest describes a series to add to a Sonarr library.
type AddSeriesRequest struct {
	TVDBID           int    `json:"tvdbId"`
	Title            string `json:"title"`
	QualityProfileID int    `json:"qualityProfileId"`
	RootFolderPath   string `json:"rootFolderPath"`
	Monitored        bool   `json:"monitored"`
	SeasonFolder     bool   `json:"seasonFolder"`
	AddOptions       struct {
		SearchForMissingEpisodes bool `json:"searchForMissingEpisodes"`
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

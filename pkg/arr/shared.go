package arr

import (
	"context"
	"strconv"
)

// HealthIssue is a warning or error reported by a service's health checks.
type HealthIssue struct {
	Source  string `json:"source,omitempty"`
	Type    string `json:"type,omitempty" jsonschema:"ok, notice, warning, or error"`
	Message string `json:"message"`
	WikiURL string `json:"wikiUrl,omitempty"`
}

// DiskSpace reports free and total bytes for a library path.
type DiskSpace struct {
	Path       string `json:"path"`
	Label      string `json:"label,omitempty"`
	FreeSpace  int64  `json:"freeSpace" jsonschema:"free bytes"`
	TotalSpace int64  `json:"totalSpace" jsonschema:"total bytes"`
}

// QueueItem is an in-progress or pending download.
type QueueItem struct {
	ID             int    `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status,omitempty"`
	TimeLeft       string `json:"timeleft,omitempty"`
	Size           int64  `json:"size,omitempty"`
	SizeLeft       int64  `json:"sizeleft,omitempty"`
	Protocol       string `json:"protocol,omitempty"`
	DownloadClient string `json:"downloadClient,omitempty"`
	ErrorMessage   string `json:"errorMessage,omitempty"`
}

// HistoryRecord is a past grab, import, or failure.
type HistoryRecord struct {
	ID          int    `json:"id"`
	EventType   string `json:"eventType,omitempty"`
	Date        string `json:"date,omitempty"`
	SourceTitle string `json:"sourceTitle,omitempty"`
}

// CommandResult reports the outcome of triggering a background command.
type CommandResult struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

// Episode is the trimmed view of a Sonarr episode.
type Episode struct {
	ID            int    `json:"id"`
	SeriesID      int    `json:"seriesId"`
	Title         string `json:"title"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	AirDateUTC    string `json:"airDateUtc,omitempty"`
	HasFile       bool   `json:"hasFile"`
	Monitored     bool   `json:"monitored"`
}

// IndexerStat summarises how one Prowlarr indexer has performed.
type IndexerStat struct {
	IndexerID             int    `json:"indexerId"`
	IndexerName           string `json:"indexerName"`
	NumberOfQueries       int    `json:"numberOfQueries"`
	NumberOfGrabs         int    `json:"numberOfGrabs"`
	NumberOfFailedQueries int    `json:"numberOfFailedQueries,omitempty"`
}

// paged is the envelope the queue and history endpoints wrap results in.
type paged[T any] struct {
	Page         int `json:"page"`
	PageSize     int `json:"pageSize"`
	TotalRecords int `json:"totalRecords"`
	Records      []T `json:"records"`
}

// ListHealthIssues returns the service's current health warnings and errors.
func ListHealthIssues(ctx context.Context, c *Client) ([]HealthIssue, error) {
	return GetJSON[[]HealthIssue](ctx, c, "/health")
}

// ListDiskSpace returns free and total space for each library path.
func ListDiskSpace(ctx context.Context, c *Client) ([]DiskSpace, error) {
	return GetJSON[[]DiskSpace](ctx, c, "/diskspace")
}

// ListQueue returns the current download queue, newest first.
func ListQueue(ctx context.Context, c *Client, pageSize int) ([]QueueItem, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	env, err := GetJSON[paged[QueueItem]](ctx, c, "/queue", Query{
		"pageSize": strconv.Itoa(pageSize),
	})
	if err != nil {
		return nil, err
	}
	return env.Records, nil
}

// ListHistory returns recent grab, import and failure events.
func ListHistory(ctx context.Context, c *Client, pageSize int) ([]HistoryRecord, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	env, err := GetJSON[paged[HistoryRecord]](ctx, c, "/history", Query{
		"pageSize":      strconv.Itoa(pageSize),
		"sortKey":       "date",
		"sortDirection": "descending",
	})
	if err != nil {
		return nil, err
	}
	return env.Records, nil
}

// DeleteQueueItem removes a download from the queue, optionally telling the
// download client to drop it and blocklisting the release.
func DeleteQueueItem(ctx context.Context, c *Client, id int, removeFromClient, blocklist bool) error {
	_, err := c.Delete(ctx, "/queue/"+itoa(id), Query{
		"removeFromClient": btoa(removeFromClient),
		"blocklist":        btoa(blocklist),
	})
	return err
}

// RunCommand triggers a named background command such as RefreshSeries.
func RunCommand(ctx context.Context, c *Client, name string, extra map[string]any) (CommandResult, error) {
	body := map[string]any{"name": name}
	for k, v := range extra {
		body[k] = v
	}
	raw, err := c.Post(ctx, "/command", body)
	if err != nil {
		return CommandResult{}, err
	}
	var out CommandResult
	if err := unmarshal(raw, &out); err != nil {
		return CommandResult{}, err
	}
	return out, nil
}

// SonarrCalendar returns episodes airing between start and end (YYYY-MM-DD).
func SonarrCalendar(ctx context.Context, c *Client, start, end string) ([]Episode, error) {
	q := Query{}
	if start != "" {
		q["start"] = start
	}
	if end != "" {
		q["end"] = end
	}
	return GetJSON[[]Episode](ctx, c, "/calendar", q)
}

// SonarrListEpisodes returns every episode of one series.
func SonarrListEpisodes(ctx context.Context, c *Client, seriesID int) ([]Episode, error) {
	return GetJSON[[]Episode](ctx, c, "/episode", Query{"seriesId": itoa(seriesID)})
}

// RadarrCalendar returns movies releasing between start and end (YYYY-MM-DD).
func RadarrCalendar(ctx context.Context, c *Client, start, end string) ([]Movie, error) {
	q := Query{}
	if start != "" {
		q["start"] = start
	}
	if end != "" {
		q["end"] = end
	}
	raw, err := GetJSON[[]rawMovie](ctx, c, "/calendar", q)
	if err != nil {
		return nil, err
	}
	return trimMovies(raw), nil
}

// ProwlarrIndexerStats reports query and grab counts per indexer.
func ProwlarrIndexerStats(ctx context.Context, c *Client) ([]IndexerStat, error) {
	env, err := GetJSON[struct {
		Indexers []IndexerStat `json:"indexers"`
	}](ctx, c, "/indexerstats")
	if err != nil {
		return nil, err
	}
	return env.Indexers, nil
}

// --- tags ---

// Tag is a label attached to series, movies, indexers and profiles.
type Tag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// TagDetail reports how widely a tag is used. The upstream resource lists every
// tagged id, which on a real library is thousands of integers; the counts are
// what actually answer "is this tag still in use, and where?".
type TagDetail struct {
	ID                  int    `json:"id"`
	Label               string `json:"label"`
	MediaCount          int    `json:"mediaCount" jsonschema:"series or movies carrying this tag"`
	IndexerCount        int    `json:"indexerCount,omitempty"`
	DownloadClientCount int    `json:"downloadClientCount,omitempty"`
	ImportListCount     int    `json:"importListCount,omitempty"`
	NotificationCount   int    `json:"notificationCount,omitempty"`
	DelayProfileCount   int    `json:"delayProfileCount,omitempty"`
	ReleaseProfileCount int    `json:"releaseProfileCount,omitempty"`
	AutoTagCount        int    `json:"autoTagCount,omitempty"`
}

// rawTagDetail mirrors the upstream resource. Sonarr and Radarr disagree on two
// field names for the same concepts: Sonarr sends seriesIds and restrictionIds
// where Radarr sends movieIds and releaseProfileIds. Decoding both keeps one
// projection working for either service.
type rawTagDetail struct {
	ID                int    `json:"id"`
	Label             string `json:"label"`
	SeriesIDs         []int  `json:"seriesIds"`
	MovieIDs          []int  `json:"movieIds"`
	IndexerIDs        []int  `json:"indexerIds"`
	DownloadClientIDs []int  `json:"downloadClientIds"`
	ImportListIDs     []int  `json:"importListIds"`
	NotificationIDs   []int  `json:"notificationIds"`
	DelayProfileIDs   []int  `json:"delayProfileIds"`
	RestrictionIDs    []int  `json:"restrictionIds"`
	ReleaseProfileIDs []int  `json:"releaseProfileIds"`
	AutoTagIDs        []int  `json:"autoTagIds"`
}

func (r rawTagDetail) toTagDetail() TagDetail {
	return TagDetail{
		ID:                  r.ID,
		Label:               r.Label,
		MediaCount:          len(r.SeriesIDs) + len(r.MovieIDs),
		IndexerCount:        len(r.IndexerIDs),
		DownloadClientCount: len(r.DownloadClientIDs),
		ImportListCount:     len(r.ImportListIDs),
		NotificationCount:   len(r.NotificationIDs),
		DelayProfileCount:   len(r.DelayProfileIDs),
		ReleaseProfileCount: len(r.RestrictionIDs) + len(r.ReleaseProfileIDs),
		AutoTagCount:        len(r.AutoTagIDs),
	}
}

// ListTags returns every tag configured on an instance.
func ListTags(ctx context.Context, c *Client) ([]Tag, error) {
	return GetJSON[[]Tag](ctx, c, "/tag")
}

// ListTagDetails returns each tag with a count of what carries it.
func ListTagDetails(ctx context.Context, c *Client) ([]TagDetail, error) {
	raw, err := GetJSON[[]rawTagDetail](ctx, c, "/tag/detail")
	if err != nil {
		return nil, err
	}
	out := make([]TagDetail, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.toTagDetail())
	}
	return out, nil
}

// CreateTag adds a tag and returns it with the id the service assigned.
func CreateTag(ctx context.Context, c *Client, label string) (Tag, error) {
	body, err := c.Post(ctx, "/tag", Tag{Label: label})
	if err != nil {
		return Tag{}, err
	}
	var out Tag
	if err := unmarshal(body, &out); err != nil {
		return Tag{}, err
	}
	return out, nil
}

// DeleteTag removes a tag, detaching it from everything that carried it.
func DeleteTag(ctx context.Context, c *Client, id int) error {
	_, err := c.Delete(ctx, "/tag/"+itoa(id))
	return err
}

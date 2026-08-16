package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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

// --- profiles and configuration ---

// termList decodes a release profile term set. Current releases send an array
// of strings, older ones sent a single comma-separated string; accepting only
// one shape would fail the entire call against the other.
type termList []string

// UnmarshalJSON accepts either a JSON array of strings or one delimited string.
func (t *termList) UnmarshalJSON(data []byte) error {
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		*t = list
		return nil
	}
	var joined string
	if err := json.Unmarshal(data, &joined); err != nil {
		return fmt.Errorf("release profile terms are neither an array nor a string: %s", data)
	}
	*t = nil
	for _, part := range strings.Split(joined, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			*t = append(*t, trimmed)
		}
	}
	return nil
}

// CustomFormat is the trimmed view of a custom format. The upstream resource
// embeds every matching rule with its regular expressions; a real Sonarr
// returns nearly 300 KB for the full list, so only the identity and the rule
// count survive the projection.
type CustomFormat struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	IncludeWhenRenaming bool   `json:"includeCustomFormatWhenRenaming"`
	SpecificationCount  int    `json:"specificationCount" jsonschema:"number of matching rules in this format"`
}

// rawCustomFormat mirrors the upstream payload before trimming.
type rawCustomFormat struct {
	ID                  int               `json:"id"`
	Name                string            `json:"name"`
	IncludeWhenRenaming bool              `json:"includeCustomFormatWhenRenaming"`
	Specifications      []json.RawMessage `json:"specifications"`
}

// DelayProfile decides how long to wait before grabbing a release.
type DelayProfile struct {
	ID                             int    `json:"id"`
	PreferredProtocol              string `json:"preferredProtocol,omitempty" jsonschema:"usenet or torrent"`
	UsenetDelay                    int    `json:"usenetDelay" jsonschema:"minutes to wait before grabbing a usenet release"`
	TorrentDelay                   int    `json:"torrentDelay" jsonschema:"minutes to wait before grabbing a torrent"`
	BypassIfHighestQuality         bool   `json:"bypassIfHighestQuality"`
	BypassIfAboveCustomFormatScore bool   `json:"bypassIfAboveCustomFormatScore"`
	MinimumCustomFormatScore       int    `json:"minimumCustomFormatScore,omitempty"`
	Order                          int    `json:"order,omitempty"`
	Tags                           []int  `json:"tags,omitempty" jsonschema:"tag ids this profile applies to; empty means the default profile"`
}

// ReleaseProfile accepts or rejects releases by term.
type ReleaseProfile struct {
	ID        int      `json:"id"`
	Name      string   `json:"name,omitempty"`
	Enabled   bool     `json:"enabled"`
	Required  []string `json:"required,omitempty" jsonschema:"terms a release title must contain"`
	Ignored   []string `json:"ignored,omitempty" jsonschema:"terms that reject a release"`
	IndexerID int      `json:"indexerId,omitempty" jsonschema:"indexer this profile is limited to; 0 means all"`
	Tags      []int    `json:"tags,omitempty"`
}

// rawReleaseProfile mirrors the upstream payload, tolerating both term shapes.
type rawReleaseProfile struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Enabled   bool     `json:"enabled"`
	Required  termList `json:"required"`
	Ignored   termList `json:"ignored"`
	IndexerID int      `json:"indexerId"`
	Tags      []int    `json:"tags"`
}

// Provider is the trimmed view of a configured indexer, download client,
// import list or notification connection. These resources all share one shape
// whose `fields` array holds the connection settings — including indexer API
// keys, download client passwords and notification webhook URLs. That array is
// deliberately dropped: nothing here should hand a model another service's
// credentials.
type Provider struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Implementation string `json:"implementation,omitempty"`
	Protocol       string `json:"protocol,omitempty" jsonschema:"usenet or torrent, for indexers and download clients"`
	Enabled        bool   `json:"enabled"`
	Priority       int    `json:"priority,omitempty"`
	Tags           []int  `json:"tags,omitempty"`

	SupportsSearch          bool `json:"supportsSearch,omitempty"`
	EnableRSS               bool `json:"enableRss,omitempty"`
	EnableAutomaticSearch   bool `json:"enableAutomaticSearch,omitempty"`
	EnableInteractiveSearch bool `json:"enableInteractiveSearch,omitempty"`

	RootFolderPath   string `json:"rootFolderPath,omitempty" jsonschema:"import lists only"`
	QualityProfileID int    `json:"qualityProfileId,omitempty" jsonschema:"import lists only"`
}

// rawProvider mirrors the fields of the upstream provider resources that are
// safe to expose. `fields` is absent by design, so it cannot leak by accident.
type rawProvider struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Implementation string `json:"implementation"`
	Protocol       string `json:"protocol"`
	Priority       int    `json:"priority"`
	Tags           []int  `json:"tags"`

	// Enable is the download client flag; Enabled and EnableAuto are Radarr's
	// import list flags and EnableAutomaticAdd is Sonarr's.
	Enable             bool `json:"enable"`
	Enabled            bool `json:"enabled"`
	EnableAuto         bool `json:"enableAuto"`
	EnableAutomaticAdd bool `json:"enableAutomaticAdd"`

	SupportsSearch          bool `json:"supportsSearch"`
	EnableRSS               bool `json:"enableRss"`
	EnableAutomaticSearch   bool `json:"enableAutomaticSearch"`
	EnableInteractiveSearch bool `json:"enableInteractiveSearch"`

	RootFolderPath   string `json:"rootFolderPath"`
	QualityProfileID int    `json:"qualityProfileId"`
}

// toProvider projects an upstream provider, collapsing the four different
// "is this on?" flags the services use into one answer.
func (r rawProvider) toProvider() Provider {
	return Provider{
		ID:             r.ID,
		Name:           r.Name,
		Implementation: r.Implementation,
		Protocol:       r.Protocol,
		Enabled: r.Enable || r.Enabled || r.EnableAutomaticAdd || r.EnableAuto ||
			r.EnableRSS || r.EnableAutomaticSearch || r.EnableInteractiveSearch,
		Priority:                r.Priority,
		Tags:                    r.Tags,
		SupportsSearch:          r.SupportsSearch,
		EnableRSS:               r.EnableRSS,
		EnableAutomaticSearch:   r.EnableAutomaticSearch,
		EnableInteractiveSearch: r.EnableInteractiveSearch,
		RootFolderPath:          r.RootFolderPath,
		QualityProfileID:        r.QualityProfileID,
	}
}

// NamingConfig is the file and folder naming policy. The Sonarr-only and
// Radarr-only fields are both present and omitted when empty, because the two
// services describe the same setting with different names.
//
// colonReplacementFormat is deliberately absent: Sonarr sends it as an integer
// and Radarr as a string, so no single Go field decodes both.
type NamingConfig struct {
	ID                       int    `json:"id"`
	ReplaceIllegalCharacters bool   `json:"replaceIllegalCharacters"`
	RenameEpisodes           bool   `json:"renameEpisodes,omitempty" jsonschema:"Sonarr only"`
	RenameMovies             bool   `json:"renameMovies,omitempty" jsonschema:"Radarr only"`
	StandardEpisodeFormat    string `json:"standardEpisodeFormat,omitempty"`
	DailyEpisodeFormat       string `json:"dailyEpisodeFormat,omitempty"`
	AnimeEpisodeFormat       string `json:"animeEpisodeFormat,omitempty"`
	SeriesFolderFormat       string `json:"seriesFolderFormat,omitempty"`
	SeasonFolderFormat       string `json:"seasonFolderFormat,omitempty"`
	SpecialsFolderFormat     string `json:"specialsFolderFormat,omitempty"`
	StandardMovieFormat      string `json:"standardMovieFormat,omitempty"`
	MovieFolderFormat        string `json:"movieFolderFormat,omitempty"`
}

// QualityDefinition is the size policy for one quality level.
type QualityDefinition struct {
	ID            int      `json:"id"`
	Title         string   `json:"title"`
	Quality       string   `json:"quality,omitempty"`
	Resolution    int      `json:"resolution,omitempty"`
	MinSize       *float64 `json:"minSize,omitempty" jsonschema:"megabytes per minute; null means no lower limit"`
	MaxSize       *float64 `json:"maxSize,omitempty" jsonschema:"megabytes per minute; null means unlimited"`
	PreferredSize *float64 `json:"preferredSize,omitempty"`
}

// rawQualityDefinition mirrors the upstream payload, whose quality name and
// resolution sit one level down.
type rawQualityDefinition struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Quality struct {
		Name       string `json:"name"`
		Resolution int    `json:"resolution"`
	} `json:"quality"`
	MinSize       *float64 `json:"minSize"`
	MaxSize       *float64 `json:"maxSize"`
	PreferredSize *float64 `json:"preferredSize"`
}

// ListCustomFormats returns the custom formats configured on an instance.
func ListCustomFormats(ctx context.Context, c *Client) ([]CustomFormat, error) {
	raw, err := GetJSON[[]rawCustomFormat](ctx, c, "/customformat")
	if err != nil {
		return nil, err
	}
	out := make([]CustomFormat, 0, len(raw))
	for _, r := range raw {
		out = append(out, CustomFormat{
			ID:                  r.ID,
			Name:                r.Name,
			IncludeWhenRenaming: r.IncludeWhenRenaming,
			SpecificationCount:  len(r.Specifications),
		})
	}
	return out, nil
}

// ListDelayProfiles returns the configured grab delay profiles.
func ListDelayProfiles(ctx context.Context, c *Client) ([]DelayProfile, error) {
	return GetJSON[[]DelayProfile](ctx, c, "/delayprofile")
}

// ListReleaseProfiles returns the configured release term profiles.
func ListReleaseProfiles(ctx context.Context, c *Client) ([]ReleaseProfile, error) {
	raw, err := GetJSON[[]rawReleaseProfile](ctx, c, "/releaseprofile")
	if err != nil {
		return nil, err
	}
	out := make([]ReleaseProfile, 0, len(raw))
	for _, r := range raw {
		out = append(out, ReleaseProfile{
			ID: r.ID, Name: r.Name, Enabled: r.Enabled,
			Required: r.Required, Ignored: r.Ignored,
			IndexerID: r.IndexerID, Tags: r.Tags,
		})
	}
	return out, nil
}

// listProviders fetches one of the provider-shaped endpoints and trims it.
func listProviders(ctx context.Context, c *Client, path string) ([]Provider, error) {
	raw, err := GetJSON[[]rawProvider](ctx, c, path)
	if err != nil {
		return nil, err
	}
	out := make([]Provider, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.toProvider())
	}
	return out, nil
}

// ListIndexers returns the indexers a media service searches.
func ListIndexers(ctx context.Context, c *Client) ([]Provider, error) {
	return listProviders(ctx, c, "/indexer")
}

// ListDownloadClients returns the configured download clients.
func ListDownloadClients(ctx context.Context, c *Client) ([]Provider, error) {
	return listProviders(ctx, c, "/downloadclient")
}

// ListImportLists returns the configured import lists.
func ListImportLists(ctx context.Context, c *Client) ([]Provider, error) {
	return listProviders(ctx, c, "/importlist")
}

// ListNotifications returns the configured notification connections.
func ListNotifications(ctx context.Context, c *Client) ([]Provider, error) {
	return listProviders(ctx, c, "/notification")
}

// GetNamingConfig returns the file and folder naming policy.
func GetNamingConfig(ctx context.Context, c *Client) (NamingConfig, error) {
	return GetJSON[NamingConfig](ctx, c, "/config/naming")
}

// ListQualityDefinitions returns the size limits for each quality level.
func ListQualityDefinitions(ctx context.Context, c *Client) ([]QualityDefinition, error) {
	raw, err := GetJSON[[]rawQualityDefinition](ctx, c, "/qualitydefinition")
	if err != nil {
		return nil, err
	}
	out := make([]QualityDefinition, 0, len(raw))
	for _, r := range raw {
		out = append(out, QualityDefinition{
			ID: r.ID, Title: r.Title,
			Quality: r.Quality.Name, Resolution: r.Quality.Resolution,
			MinSize: r.MinSize, MaxSize: r.MaxSize, PreferredSize: r.PreferredSize,
		})
	}
	return out, nil
}

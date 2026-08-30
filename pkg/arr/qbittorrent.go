package arr

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// qBittorrent's WebUI API v2 (wiki: WebUI-API-(qBittorrent-5.0)). Every
// mutation is a form-encoded POST that answers 200 with an empty body; the
// only ones that report failure in-band are login and torrents/add, which
// answer "Ok." or "Fails." in the body. Torrent selectors are hashes joined
// with "|", or the literal "all". Version 5 renamed pause/resume to
// stop/start and the add-time flag from paused to stopped.

// hashesAll is the selector qBittorrent accepts in place of a hash list.
const hashesAll = "all"

// shareLimitDefault is the value qBittorrent reads as "use the global
// setting" for ratio and seeding time limits; -1 means no limit.
const shareLimitDefault = -2

// Torrent is the trimmed view of one torrent. The upstream object carries
// about fifty fields including magnet URIs, tracker URLs and session
// counters; these are the ones that answer "what is it doing?".
type Torrent struct {
	Hash             string   `json:"hash"`
	Name             string   `json:"name"`
	State            string   `json:"state" jsonschema:"downloading, uploading, stalledDL, stalledUP, stoppedDL, stoppedUP, queuedDL, queuedUP, checkingDL, checkingUP, metaDL, moving, error, missingFiles or unknown"`
	Progress         float64  `json:"progress" jsonschema:"fraction complete, 0 to 1"`
	Size             int64    `json:"size" jsonschema:"bytes selected for download"`
	Downloaded       int64    `json:"downloaded" jsonschema:"bytes downloaded"`
	Uploaded         int64    `json:"uploaded" jsonschema:"bytes uploaded"`
	DownloadSpeed    int64    `json:"downloadSpeed" jsonschema:"bytes per second"`
	UploadSpeed      int64    `json:"uploadSpeed" jsonschema:"bytes per second"`
	ETA              int64    `json:"eta" jsonschema:"seconds remaining; 8640000 means unknown"`
	Ratio            float64  `json:"ratio" jsonschema:"share ratio so far"`
	Category         string   `json:"category,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	SavePath         string   `json:"savePath"`
	AddedOn          string   `json:"addedOn,omitempty" jsonschema:"RFC 3339 UTC"`
	CompletionOn     string   `json:"completionOn,omitempty" jsonschema:"RFC 3339 UTC; empty until the download finishes"`
	Seeds            int      `json:"seeds" jsonschema:"connected seeds"`
	Leechers         int      `json:"leechers" jsonschema:"connected leechers"`
	Priority         int      `json:"priority" jsonschema:"queue position; -1 when queueing is off or the torrent is not queued"`
	RatioLimit       float64  `json:"ratioLimit" jsonschema:"-2 global default, -1 unlimited"`
	SeedingTimeLimit int      `json:"seedingTimeLimit" jsonschema:"minutes; -2 global default, -1 unlimited"`
	DownloadLimit    int64    `json:"downloadLimit" jsonschema:"bytes per second; 0 unlimited"`
	UploadLimit      int64    `json:"uploadLimit" jsonschema:"bytes per second; 0 unlimited"`
}

// rawTorrent is the subset of torrents/info this package reads.
type rawTorrent struct {
	Hash             string  `json:"hash"`
	Name             string  `json:"name"`
	State            string  `json:"state"`
	Progress         float64 `json:"progress"`
	Size             int64   `json:"size"`
	Downloaded       int64   `json:"downloaded"`
	Uploaded         int64   `json:"uploaded"`
	DLSpeed          int64   `json:"dlspeed"`
	UPSpeed          int64   `json:"upspeed"`
	ETA              int64   `json:"eta"`
	Ratio            float64 `json:"ratio"`
	Category         string  `json:"category"`
	Tags             string  `json:"tags"`
	SavePath         string  `json:"save_path"`
	AddedOn          int64   `json:"added_on"`
	CompletionOn     int64   `json:"completion_on"`
	NumSeeds         int     `json:"num_seeds"`
	NumLeechs        int     `json:"num_leechs"`
	Priority         int     `json:"priority"`
	RatioLimit       float64 `json:"ratio_limit"`
	SeedingTimeLimit int     `json:"seeding_time_limit"`
	DLLimit          int64   `json:"dl_limit"`
	UPLimit          int64   `json:"up_limit"`
}

func (r rawTorrent) toTorrent() Torrent {
	return Torrent{
		Hash: r.Hash, Name: r.Name, State: r.State, Progress: r.Progress,
		Size: r.Size, Downloaded: r.Downloaded, Uploaded: r.Uploaded,
		DownloadSpeed: r.DLSpeed, UploadSpeed: r.UPSpeed, ETA: r.ETA, Ratio: r.Ratio,
		Category: r.Category, Tags: splitTags(r.Tags), SavePath: r.SavePath,
		AddedOn: unixTime(r.AddedOn), CompletionOn: unixTime(r.CompletionOn),
		Seeds: r.NumSeeds, Leechers: r.NumLeechs, Priority: r.Priority,
		RatioLimit: r.RatioLimit, SeedingTimeLimit: r.SeedingTimeLimit,
		DownloadLimit: r.DLLimit, UploadLimit: r.UPLimit,
	}
}

// splitTags turns qBittorrent's comma-separated tag string into a list.
func splitTags(s string) []string {
	var tags []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// unixTime renders a unix timestamp as RFC 3339 UTC; zero stays empty.
func unixTime(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

// TorrentFile is one file inside a torrent.
type TorrentFile struct {
	Index    int     `json:"index"`
	Name     string  `json:"name" jsonschema:"path relative to the torrent's save path"`
	Size     int64   `json:"size" jsonschema:"bytes"`
	Progress float64 `json:"progress" jsonschema:"fraction complete, 0 to 1"`
	Priority int     `json:"priority" jsonschema:"0 do not download, 1 normal, 6 high, 7 maximal"`
}

// TorrentCategory is a category with its optional save path.
type TorrentCategory struct {
	Name     string `json:"name"`
	SavePath string `json:"savePath,omitempty" jsonschema:"empty means the default download path"`
}

// TransferInfo is the global transfer state.
type TransferInfo struct {
	DownloadSpeed          int64  `json:"downloadSpeed" jsonschema:"bytes per second"`
	UploadSpeed            int64  `json:"uploadSpeed" jsonschema:"bytes per second"`
	DownloadLimit          int64  `json:"downloadLimit" jsonschema:"global limit in bytes per second; 0 unlimited"`
	UploadLimit            int64  `json:"uploadLimit" jsonschema:"global limit in bytes per second; 0 unlimited"`
	DHTNodes               int    `json:"dhtNodes"`
	ConnectionStatus       string `json:"connectionStatus" jsonschema:"connected, firewalled or disconnected"`
	AlternativeSpeedLimits bool   `json:"alternativeSpeedLimits" jsonschema:"whether the alternative speed limits are active"`
}

// rawTransferInfo is the subset of transfer/info this package reads.
type rawTransferInfo struct {
	DLSpeed    int64  `json:"dl_info_speed"`
	UPSpeed    int64  `json:"up_info_speed"`
	DLLimit    int64  `json:"dl_rate_limit"`
	UPLimit    int64  `json:"up_rate_limit"`
	DHTNodes   int    `json:"dht_nodes"`
	ConnStatus string `json:"connection_status"`
}

// QBittorrentVersion reports the application and WebUI API versions.
type QBittorrentVersion struct {
	Version    string `json:"version"`
	APIVersion string `json:"apiVersion"`
}

// TorrentFilter narrows a torrent listing. Category and Tag are pointers
// because the empty string is meaningful to qBittorrent: category="" selects
// uncategorised torrents, whereas nil applies no category filter.
type TorrentFilter struct {
	Filter   string
	Category *string
	Tag      *string
	Hashes   []string
	Limit    int
}

// AddTorrentRequest describes torrents to add by URL.
type AddTorrentRequest struct {
	URLs             []string
	SavePath         string
	Category         string
	Tags             []string
	Stopped          bool
	Rename           string
	DownloadLimit    *int64
	UploadLimit      *int64
	RatioLimit       *float64
	SeedingTimeLimit *int
	AutoTMM          *bool
}

// ShareLimits are per-torrent seeding limits. Nil fields are sent as -2, the
// global default, because the endpoint requires all three on every call.
type ShareLimits struct {
	RatioLimit               *float64
	SeedingTimeLimit         *int
	InactiveSeedingTimeLimit *int
}

// QBittorrentSystemStatus reports the application and API versions. Both
// endpoints answer with plain text rather than JSON.
func QBittorrentSystemStatus(ctx context.Context, c *Client) (QBittorrentVersion, error) {
	version, err := c.Get(ctx, "/app/version")
	if err != nil {
		return QBittorrentVersion{}, err
	}
	api, err := c.Get(ctx, "/app/webapiVersion")
	if err != nil {
		return QBittorrentVersion{}, err
	}
	return QBittorrentVersion{
		Version:    strings.TrimSpace(string(version)),
		APIVersion: strings.TrimSpace(string(api)),
	}, nil
}

// QBittorrentListTorrents lists torrents matching the filter.
func QBittorrentListTorrents(ctx context.Context, c *Client, f TorrentFilter) ([]Torrent, error) {
	q := Query{}
	if f.Filter != "" {
		q["filter"] = f.Filter
	}
	if f.Category != nil {
		q["category"] = *f.Category
	}
	if f.Tag != nil {
		q["tag"] = *f.Tag
	}
	if len(f.Hashes) > 0 {
		hashes, err := joinHashes(f.Hashes)
		if err != nil {
			return nil, err
		}
		q["hashes"] = hashes
	}
	if f.Limit > 0 {
		q["limit"] = itoa(f.Limit)
	}
	raw, err := GetJSON[[]rawTorrent](ctx, c, "/torrents/info", q)
	if err != nil {
		return nil, err
	}
	torrents := make([]Torrent, 0, len(raw))
	for _, r := range raw {
		torrents = append(torrents, r.toTorrent())
	}
	return torrents, nil
}

// QBittorrentTorrentFiles lists the files inside one torrent.
func QBittorrentTorrentFiles(ctx context.Context, c *Client, hash string) ([]TorrentFile, error) {
	if hash == "" {
		return nil, fmt.Errorf("torrent hash is required")
	}
	return GetJSON[[]TorrentFile](ctx, c, "/torrents/files", Query{"hash": hash})
}

// QBittorrentTransferInfo reports global speeds, limits and whether the
// alternative speed limits are active.
func QBittorrentTransferInfo(ctx context.Context, c *Client) (TransferInfo, error) {
	raw, err := GetJSON[rawTransferInfo](ctx, c, "/transfer/info")
	if err != nil {
		return TransferInfo{}, err
	}
	alt, err := qbitAlternativeMode(ctx, c)
	if err != nil {
		return TransferInfo{}, err
	}
	return TransferInfo{
		DownloadSpeed: raw.DLSpeed, UploadSpeed: raw.UPSpeed,
		DownloadLimit: raw.DLLimit, UploadLimit: raw.UPLimit,
		DHTNodes: raw.DHTNodes, ConnectionStatus: raw.ConnStatus,
		AlternativeSpeedLimits: alt,
	}, nil
}

// qbitAlternativeMode reads transfer/speedLimitsMode, which answers "1" when
// the alternative limits are active and "0" otherwise.
func qbitAlternativeMode(ctx context.Context, c *Client) (bool, error) {
	body, err := c.Get(ctx, "/transfer/speedLimitsMode")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(body)) == "1", nil
}

// QBittorrentListCategories lists categories sorted by name. The upstream
// response is an object keyed by category name.
func QBittorrentListCategories(ctx context.Context, c *Client) ([]TorrentCategory, error) {
	raw, err := GetJSON[map[string]TorrentCategory](ctx, c, "/torrents/categories")
	if err != nil {
		return nil, err
	}
	cats := make([]TorrentCategory, 0, len(raw))
	for name, cat := range raw {
		if cat.Name == "" {
			cat.Name = name
		}
		cats = append(cats, cat)
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].Name < cats[j].Name })
	return cats, nil
}

// QBittorrentListTags lists every tag known to the instance.
func QBittorrentListTags(ctx context.Context, c *Client) ([]string, error) {
	return GetJSON[[]string](ctx, c, "/torrents/tags")
}

// QBittorrentAddTorrent adds torrents by http, https or magnet URL. qBittorrent
// answers 200 for every outcome and reports failure only through the body.
func QBittorrentAddTorrent(ctx context.Context, c *Client, req AddTorrentRequest) error {
	if len(req.URLs) == 0 {
		return fmt.Errorf("at least one torrent url is required")
	}
	for _, u := range req.URLs {
		lower := strings.ToLower(u)
		if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") && !strings.HasPrefix(lower, "magnet:") {
			return fmt.Errorf("unsupported torrent url %q: only http, https and magnet urls can be added", u)
		}
	}

	form := url.Values{"urls": {strings.Join(req.URLs, "\n")}}
	if req.SavePath != "" {
		form.Set("savepath", req.SavePath)
	}
	if req.Category != "" {
		form.Set("category", req.Category)
	}
	if len(req.Tags) > 0 {
		form.Set("tags", strings.Join(req.Tags, ","))
	}
	if req.Stopped {
		form.Set("stopped", "true")
	}
	if req.Rename != "" {
		form.Set("rename", req.Rename)
	}
	if req.DownloadLimit != nil {
		form.Set("dlLimit", strconv.FormatInt(*req.DownloadLimit, 10))
	}
	if req.UploadLimit != nil {
		form.Set("upLimit", strconv.FormatInt(*req.UploadLimit, 10))
	}
	if req.RatioLimit != nil {
		form.Set("ratioLimit", ftoa(*req.RatioLimit))
	}
	if req.SeedingTimeLimit != nil {
		form.Set("seedingTimeLimit", itoa(*req.SeedingTimeLimit))
	}
	if req.AutoTMM != nil {
		form.Set("autoTMM", btoa(*req.AutoTMM))
	}

	body, err := c.PostForm(ctx, "/torrents/add", form)
	if err != nil {
		return err
	}
	if got := strings.TrimSpace(string(body)); got != "Ok." {
		return fmt.Errorf("qbittorrent did not accept the torrent: %q", got)
	}
	return nil
}

// QBittorrentStopTorrents stops (pauses) torrents.
func QBittorrentStopTorrents(ctx context.Context, c *Client, hashes []string) error {
	return qbitHashAction(ctx, c, "/torrents/stop", hashes, nil)
}

// QBittorrentStartTorrents starts (resumes) torrents.
func QBittorrentStartTorrents(ctx context.Context, c *Client, hashes []string) error {
	return qbitHashAction(ctx, c, "/torrents/start", hashes, nil)
}

// QBittorrentRecheckTorrents re-verifies torrent data on disk.
func QBittorrentRecheckTorrents(ctx context.Context, c *Client, hashes []string) error {
	return qbitHashAction(ctx, c, "/torrents/recheck", hashes, nil)
}

// QBittorrentDeleteTorrents removes torrents, optionally deleting their files.
func QBittorrentDeleteTorrents(ctx context.Context, c *Client, hashes []string, deleteFiles bool) error {
	return qbitHashAction(ctx, c, "/torrents/delete", hashes, url.Values{"deleteFiles": {btoa(deleteFiles)}})
}

// QBittorrentSetCategory assigns a category; an empty name clears it.
func QBittorrentSetCategory(ctx context.Context, c *Client, hashes []string, category string) error {
	return qbitHashAction(ctx, c, "/torrents/setCategory", hashes, url.Values{"category": {category}})
}

// QBittorrentCreateCategory creates a category with an optional save path.
func QBittorrentCreateCategory(ctx context.Context, c *Client, name, savePath string) error {
	if name == "" {
		return fmt.Errorf("category name is required")
	}
	return qbitAction(ctx, c, "/torrents/createCategory", url.Values{"category": {name}, "savePath": {savePath}})
}

// QBittorrentEditCategory changes a category's save path.
func QBittorrentEditCategory(ctx context.Context, c *Client, name, savePath string) error {
	if name == "" {
		return fmt.Errorf("category name is required")
	}
	return qbitAction(ctx, c, "/torrents/editCategory", url.Values{"category": {name}, "savePath": {savePath}})
}

// QBittorrentRemoveCategories deletes categories; torrents in them become
// uncategorised. Names are newline-separated on the wire.
func QBittorrentRemoveCategories(ctx context.Context, c *Client, names []string) error {
	if len(names) == 0 {
		return fmt.Errorf("at least one category name is required")
	}
	return qbitAction(ctx, c, "/torrents/removeCategories", url.Values{"categories": {strings.Join(names, "\n")}})
}

// QBittorrentAddTags applies tags to torrents, creating any that do not exist.
func QBittorrentAddTags(ctx context.Context, c *Client, hashes, tags []string) error {
	if len(tags) == 0 {
		return fmt.Errorf("at least one tag is required")
	}
	return qbitHashAction(ctx, c, "/torrents/addTags", hashes, url.Values{"tags": {strings.Join(tags, ",")}})
}

// QBittorrentRemoveTags strips tags from torrents; no tags means all of them.
func QBittorrentRemoveTags(ctx context.Context, c *Client, hashes, tags []string) error {
	return qbitHashAction(ctx, c, "/torrents/removeTags", hashes, url.Values{"tags": {strings.Join(tags, ",")}})
}

// QBittorrentSetLocation moves torrents' data to a new directory.
func QBittorrentSetLocation(ctx context.Context, c *Client, hashes []string, location string) error {
	if location == "" {
		return fmt.Errorf("location is required")
	}
	return qbitHashAction(ctx, c, "/torrents/setLocation", hashes, url.Values{"location": {location}})
}

// QBittorrentRenameTorrent changes a torrent's display name.
func QBittorrentRenameTorrent(ctx context.Context, c *Client, hash, name string) error {
	if hash == "" || name == "" {
		return fmt.Errorf("torrent hash and new name are required")
	}
	return qbitAction(ctx, c, "/torrents/rename", url.Values{"hash": {hash}, "name": {name}})
}

// priorityEndpoints maps a queue position to the endpoint that moves there.
var priorityEndpoints = map[string]string{
	"top": "/torrents/topPrio", "bottom": "/torrents/bottomPrio",
	"up": "/torrents/increasePrio", "down": "/torrents/decreasePrio",
}

// QBittorrentSetPriority moves torrents in the download queue. qBittorrent
// answers 409 when queueing is disabled, which surfaces as the client error.
func QBittorrentSetPriority(ctx context.Context, c *Client, hashes []string, position string) error {
	path, ok := priorityEndpoints[position]
	if !ok {
		return fmt.Errorf("unknown queue position %q; use top, bottom, up or down", position)
	}
	return qbitHashAction(ctx, c, path, hashes, nil)
}

// QBittorrentSetTorrentLimits sets per-torrent speed and share limits,
// calling only the endpoints whose inputs were given.
func QBittorrentSetTorrentLimits(ctx context.Context, c *Client, hashes []string, dl, up *int64, share *ShareLimits) error {
	if dl == nil && up == nil && share == nil {
		return fmt.Errorf("no limits given; set at least one of the download, upload or share limits")
	}
	if dl != nil {
		form := url.Values{"limit": {strconv.FormatInt(*dl, 10)}}
		if err := qbitHashAction(ctx, c, "/torrents/setDownloadLimit", hashes, form); err != nil {
			return err
		}
	}
	if up != nil {
		form := url.Values{"limit": {strconv.FormatInt(*up, 10)}}
		if err := qbitHashAction(ctx, c, "/torrents/setUploadLimit", hashes, form); err != nil {
			return err
		}
	}
	if share != nil {
		form := url.Values{
			"ratioLimit":               {ftoa(shareLimitDefault)},
			"seedingTimeLimit":         {itoa(shareLimitDefault)},
			"inactiveSeedingTimeLimit": {itoa(shareLimitDefault)},
		}
		if share.RatioLimit != nil {
			form.Set("ratioLimit", ftoa(*share.RatioLimit))
		}
		if share.SeedingTimeLimit != nil {
			form.Set("seedingTimeLimit", itoa(*share.SeedingTimeLimit))
		}
		if share.InactiveSeedingTimeLimit != nil {
			form.Set("inactiveSeedingTimeLimit", itoa(*share.InactiveSeedingTimeLimit))
		}
		if err := qbitHashAction(ctx, c, "/torrents/setShareLimits", hashes, form); err != nil {
			return err
		}
	}
	return nil
}

// QBittorrentSetGlobalLimits sets the global speed limits and switches the
// alternative limits on or off. The mode endpoint is a toggle, so it is only
// called when the current state differs from the one requested. Returns the
// refreshed transfer state.
func QBittorrentSetGlobalLimits(ctx context.Context, c *Client, dl, up *int64, altMode *bool) (TransferInfo, error) {
	if dl == nil && up == nil && altMode == nil {
		return TransferInfo{}, fmt.Errorf("no limits given; set at least one of the download limit, upload limit or alternative mode")
	}
	if dl != nil {
		if err := qbitAction(ctx, c, "/transfer/setDownloadLimit", url.Values{"limit": {strconv.FormatInt(*dl, 10)}}); err != nil {
			return TransferInfo{}, err
		}
	}
	if up != nil {
		if err := qbitAction(ctx, c, "/transfer/setUploadLimit", url.Values{"limit": {strconv.FormatInt(*up, 10)}}); err != nil {
			return TransferInfo{}, err
		}
	}
	if altMode != nil {
		current, err := qbitAlternativeMode(ctx, c)
		if err != nil {
			return TransferInfo{}, err
		}
		if current != *altMode {
			if err := qbitAction(ctx, c, "/transfer/toggleSpeedLimitsMode", url.Values{}); err != nil {
				return TransferInfo{}, err
			}
		}
	}
	return QBittorrentTransferInfo(ctx, c)
}

// joinHashes renders a hash selector: "all" for the sentinel alone, otherwise
// the hashes joined with "|". An empty list is refused rather than sent,
// because qBittorrent would silently apply the action to nothing.
func joinHashes(hashes []string) (string, error) {
	if len(hashes) == 0 {
		return "", fmt.Errorf("at least one torrent hash is required; pass [\"all\"] to target every torrent")
	}
	if len(hashes) == 1 && hashes[0] == hashesAll {
		return hashesAll, nil
	}
	for _, h := range hashes {
		if h == "" || h == hashesAll {
			return "", fmt.Errorf("invalid hash selector %q: the all sentinel must be the only entry", h)
		}
	}
	return strings.Join(hashes, "|"), nil
}

// qbitHashAction posts a form carrying a hash selector plus extra fields.
func qbitHashAction(ctx context.Context, c *Client, path string, hashes []string, form url.Values) error {
	selector, err := joinHashes(hashes)
	if err != nil {
		return err
	}
	if form == nil {
		form = url.Values{}
	}
	form.Set("hashes", selector)
	return qbitAction(ctx, c, path, form)
}

// qbitAction posts a form and discards the empty body every action returns.
func qbitAction(ctx context.Context, c *Client, path string, form url.Values) error {
	_, err := c.PostForm(ctx, path, form)
	return err
}

// ftoa formats a float for a form field without trailing zeros.
func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

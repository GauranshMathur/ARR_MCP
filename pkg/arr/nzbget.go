package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// NZBGet speaks JSON-RPC 1.1 over a single POST /jsonrpc endpoint rather than
// REST: every call is a method name plus positional params, and the answer is
// an envelope holding either result or error. The helpers below keep that
// envelope in one place so the typed calls read like the REST services.

// rpcRequest is the JSON-RPC envelope NZBGet expects. Params must be a JSON
// array even when empty; NZBGet rejects null.
type rpcRequest struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
	ID     int    `json:"id"`
}

// rpcError is the error object NZBGet returns for a bad method or parameter.
type rpcError struct {
	Name    string `json:"name"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// rpcResponse is the JSON-RPC envelope around a typed result.
type rpcResponse[T any] struct {
	Result T         `json:"result"`
	Error  *rpcError `json:"error"`
}

// nzbCall invokes one JSON-RPC method and decodes its result into T. A
// JSON-RPC error object becomes a Go error naming the method, so the model
// can tell an unknown procedure from a failed command.
func nzbCall[T any](ctx context.Context, c *Client, method string, params ...any) (T, error) {
	var out T
	if params == nil {
		params = []any{}
	}
	body, err := c.Post(ctx, "/jsonrpc", rpcRequest{Method: method, Params: params, ID: 1})
	if err != nil {
		return out, err
	}
	var resp rpcResponse[T]
	if err := json.Unmarshal(body, &resp); err != nil {
		return out, fmt.Errorf("decoding nzbget %s response: %w", method, err)
	}
	if resp.Error != nil {
		return out, fmt.Errorf("nzbget %s: %s (code %d)", method, resp.Error.Message, resp.Error.Code)
	}
	return resp.Result, nil
}

// NZBStatus is the trimmed view of NZBGet's status call. Rates are reported in
// KiB/s to match the unit the rate limit is set in; upstream reports bytes.
type NZBStatus struct {
	Version          string `json:"version"`
	DownloadRateKB   int    `json:"downloadRateKB" jsonschema:"current download speed in KiB/s"`
	DownloadLimitKB  int    `json:"downloadLimitKB" jsonschema:"configured speed limit in KiB/s; 0 means unlimited"`
	RemainingSizeMB  int    `json:"remainingSizeMB" jsonschema:"size of everything still queued"`
	DownloadedSizeMB int    `json:"downloadedSizeMB" jsonschema:"downloaded since the server started"`
	FreeDiskSpaceMB  int    `json:"freeDiskSpaceMB"`
	DownloadPaused   bool   `json:"downloadPaused"`
	PostPaused       bool   `json:"postPaused" jsonschema:"post-processing (par repair, unpack, scripts) is paused"`
	ScanPaused       bool   `json:"scanPaused" jsonschema:"scanning of the incoming nzb directory is paused"`
	ServerStandBy    bool   `json:"serverStandBy" jsonschema:"true when nothing is downloading"`
	ThreadCount      int    `json:"threadCount"`
	PostJobCount     int    `json:"postJobCount" jsonschema:"jobs waiting for post-processing"`
	UpTimeSec        int    `json:"upTimeSec"`
	DownloadTimeSec  int    `json:"downloadTimeSec"`
}

// rawNZBStatus is the subset of the status result that NZBStatus is built
// from. The download rate arrives split into 32-bit halves.
type rawNZBStatus struct {
	RemainingSizeMB  int   `json:"RemainingSizeMB"`
	DownloadedSizeMB int   `json:"DownloadedSizeMB"`
	FreeDiskSpaceMB  int   `json:"FreeDiskSpaceMB"`
	DownloadRateLo   int64 `json:"DownloadRateLo"`
	DownloadRateHi   int64 `json:"DownloadRateHi"`
	DownloadLimit    int64 `json:"DownloadLimit"`
	DownloadPaused   bool  `json:"DownloadPaused"`
	PostPaused       bool  `json:"PostPaused"`
	ScanPaused       bool  `json:"ScanPaused"`
	ServerStandBy    bool  `json:"ServerStandBy"`
	ThreadCount      int   `json:"ThreadCount"`
	PostJobCount     int   `json:"PostJobCount"`
	UpTimeSec        int   `json:"UpTimeSec"`
	DownloadTimeSec  int   `json:"DownloadTimeSec"`
}

// NZBGroup is one queue entry (an nzb and its files) in the trimmed form the
// tools return. Upstream also carries per-file counts, article statistics and
// duplicate metadata that do not help decide what to do with the queue.
type NZBGroup struct {
	ID              int     `json:"id" jsonschema:"NZBID used by every queue editing tool"`
	Name            string  `json:"name"`
	Status          string  `json:"status" jsonschema:"e.g. DOWNLOADING, PAUSED, QUEUED, UNPACKING, PP_QUEUED"`
	Category        string  `json:"category,omitempty"`
	SizeMB          int     `json:"sizeMB"`
	RemainingMB     int     `json:"remainingMB"`
	PausedMB        int     `json:"pausedMB" jsonschema:"part of remainingMB belonging to paused files"`
	DownloadedMB    int     `json:"downloadedMB"`
	HealthPercent   float64 `json:"healthPercent" jsonschema:"expected completeness; below 100 means articles are missing"`
	ActiveDownloads int     `json:"activeDownloads" jsonschema:"connections currently downloading this item"`
	Priority        int     `json:"priority" jsonschema:"-100 very low, -50 low, 0 normal, 50 high, 100 very high, 900 force"`
	DestDir         string  `json:"destDir,omitempty"`
}

// rawNZBGroup is the subset of a listgroups entry that NZBGroup is built from.
type rawNZBGroup struct {
	NZBID            int    `json:"NZBID"`
	NZBName          string `json:"NZBName"`
	Status           string `json:"Status"`
	Category         string `json:"Category"`
	FileSizeMB       int    `json:"FileSizeMB"`
	RemainingSizeMB  int    `json:"RemainingSizeMB"`
	PausedSizeMB     int    `json:"PausedSizeMB"`
	DownloadedSizeMB int    `json:"DownloadedSizeMB"`
	Health           int    `json:"Health"`
	ActiveDownloads  int    `json:"ActiveDownloads"`
	MaxPriority      int    `json:"MaxPriority"`
	DestDir          string `json:"DestDir"`
}

// NZBHistoryItem is one finished, failed or deleted download.
type NZBHistoryItem struct {
	ID            int     `json:"id" jsonschema:"NZBID used by the history tools"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind" jsonschema:"NZB, URL (a fetch that failed) or DUP (a hidden duplicate)"`
	Status        string  `json:"status" jsonschema:"e.g. SUCCESS/UNPACK, FAILURE/PAR, DELETED/MANUAL, WARNING/HEALTH"`
	Category      string  `json:"category,omitempty"`
	SizeMB        int     `json:"sizeMB"`
	Time          string  `json:"time,omitempty" jsonschema:"when the item entered history, RFC 3339 UTC"`
	DestDir       string  `json:"destDir,omitempty"`
	FinalDir      string  `json:"finalDir,omitempty" jsonschema:"where post-processing moved the files, when different from destDir"`
	HealthPercent float64 `json:"healthPercent"`
	DupeKey       string  `json:"dupeKey,omitempty"`
	DeleteStatus  string  `json:"deleteStatus,omitempty" jsonschema:"why it was deleted: NONE, MANUAL, HEALTH, DUPE, BAD, SCAN or COPY"`
	MarkStatus    string  `json:"markStatus,omitempty" jsonschema:"NONE, GOOD or BAD"`
}

// rawNZBHistoryItem is the subset of a history entry NZBHistoryItem is built
// from. NZBID is preferred over the deprecated ID, which aliases it.
type rawNZBHistoryItem struct {
	NZBID        int    `json:"NZBID"`
	Name         string `json:"Name"`
	Kind         string `json:"Kind"`
	Status       string `json:"Status"`
	Category     string `json:"Category"`
	FileSizeMB   int    `json:"FileSizeMB"`
	HistoryTime  int64  `json:"HistoryTime"`
	DestDir      string `json:"DestDir"`
	FinalDir     string `json:"FinalDir"`
	Health       int    `json:"Health"`
	DupeKey      string `json:"DupeKey"`
	DeleteStatus string `json:"DeleteStatus"`
	MarkStatus   string `json:"MarkStatus"`
}

// AppendNZBRequest describes an nzb to add. Exactly one of URL and Content is
// set: NZBGet takes either in the same positional slot and tells them apart
// itself.
type AppendNZBRequest struct {
	Filename  string
	URL       string
	Content   string
	Category  string
	Priority  int
	AddToTop  bool
	AddPaused bool
	DupeKey   string
	DupeScore int
	DupeMode  string
}

// healthPercent converts NZBGet's permille health to a percentage.
func healthPercent(permille int) float64 { return float64(permille) / 10 }

// NZBGetVersion returns the server version string.
func NZBGetVersion(ctx context.Context, c *Client) (string, error) {
	return nzbCall[string](ctx, c, "version")
}

// NZBGetStatus returns the trimmed server status together with its version.
func NZBGetStatus(ctx context.Context, c *Client) (NZBStatus, error) {
	raw, err := nzbCall[rawNZBStatus](ctx, c, "status")
	if err != nil {
		return NZBStatus{}, err
	}
	version, err := NZBGetVersion(ctx, c)
	if err != nil {
		return NZBStatus{}, err
	}
	rate := raw.DownloadRateHi<<32 | raw.DownloadRateLo
	return NZBStatus{
		Version:          version,
		DownloadRateKB:   int(rate / 1024),
		DownloadLimitKB:  int(raw.DownloadLimit / 1024),
		RemainingSizeMB:  raw.RemainingSizeMB,
		DownloadedSizeMB: raw.DownloadedSizeMB,
		FreeDiskSpaceMB:  raw.FreeDiskSpaceMB,
		DownloadPaused:   raw.DownloadPaused,
		PostPaused:       raw.PostPaused,
		ScanPaused:       raw.ScanPaused,
		ServerStandBy:    raw.ServerStandBy,
		ThreadCount:      raw.ThreadCount,
		PostJobCount:     raw.PostJobCount,
		UpTimeSec:        raw.UpTimeSec,
		DownloadTimeSec:  raw.DownloadTimeSec,
	}, nil
}

// NZBGetListGroups returns the download queue, one entry per nzb.
func NZBGetListGroups(ctx context.Context, c *Client) ([]NZBGroup, error) {
	raws, err := nzbCall[[]rawNZBGroup](ctx, c, "listgroups")
	if err != nil {
		return nil, err
	}
	groups := make([]NZBGroup, 0, len(raws))
	for _, r := range raws {
		groups = append(groups, NZBGroup{
			ID: r.NZBID, Name: r.NZBName, Status: r.Status, Category: r.Category,
			SizeMB: r.FileSizeMB, RemainingMB: r.RemainingSizeMB, PausedMB: r.PausedSizeMB,
			DownloadedMB: r.DownloadedSizeMB, HealthPercent: healthPercent(r.Health),
			ActiveDownloads: r.ActiveDownloads, Priority: r.MaxPriority, DestDir: r.DestDir,
		})
	}
	return groups, nil
}

// NZBGetHistory returns history entries, newest first as NZBGet orders them.
// hidden also includes records hidden by HistoryDelete and DUP entries. NZBGet
// has no paging, so limit truncates client-side; 0 returns everything.
func NZBGetHistory(ctx context.Context, c *Client, hidden bool, limit int) ([]NZBHistoryItem, error) {
	raws, err := nzbCall[[]rawNZBHistoryItem](ctx, c, "history", hidden)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(raws) > limit {
		raws = raws[:limit]
	}
	items := make([]NZBHistoryItem, 0, len(raws))
	for _, r := range raws {
		item := NZBHistoryItem{
			ID: r.NZBID, Name: r.Name, Kind: r.Kind, Status: r.Status, Category: r.Category,
			SizeMB: r.FileSizeMB, DestDir: r.DestDir, FinalDir: r.FinalDir,
			HealthPercent: healthPercent(r.Health), DupeKey: r.DupeKey,
			DeleteStatus: r.DeleteStatus, MarkStatus: r.MarkStatus,
		}
		if r.HistoryTime > 0 {
			item.Time = time.Unix(r.HistoryTime, 0).UTC().Format(time.RFC3339)
		}
		items = append(items, item)
	}
	return items, nil
}

// NZBGetAppend adds an nzb by URL or base64 content and returns its NZBID.
// The append method is positional; the trailing empty array is PPParameters,
// which NZBGet requires even when there are none.
func NZBGetAppend(ctx context.Context, c *Client, req AppendNZBRequest) (int, error) {
	if (req.URL == "") == (req.Content == "") {
		return 0, fmt.Errorf("exactly one of url and content is required")
	}
	content := req.Content
	if content == "" {
		content = req.URL
	}
	dupeMode := req.DupeMode
	if dupeMode == "" {
		dupeMode = "SCORE"
	}
	id, err := nzbCall[int](ctx, c, "append",
		req.Filename, content, req.Category, req.Priority, req.AddToTop, req.AddPaused,
		req.DupeKey, req.DupeScore, dupeMode, []any{})
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, fmt.Errorf("nzbget append rejected the nzb (result %d); check the url is reachable "+
			"or that content is base64 and filename ends in .nzb", id)
	}
	return id, nil
}

// NZBGetEditQueue runs one editqueue command against queue or history entries.
// NZBGet answers false rather than an error when nothing matched, so that is
// turned into an error naming the command and ids.
func NZBGetEditQueue(ctx context.Context, c *Client, command, param string, ids []int) error {
	ok, err := nzbCall[bool](ctx, c, "editqueue", command, param, ids)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("nzbget editqueue %s failed for ids %v; check the ids exist in the queue or history", command, ids)
	}
	return nil
}

// pauseMethods maps a pause scope to its pause and resume method names.
var pauseMethods = map[string][2]string{
	"download": {"pausedownload", "resumedownload"},
	"post":     {"pausepost", "resumepost"},
	"scan":     {"pausescan", "resumescan"},
}

// NZBGetSetPaused pauses or resumes one of NZBGet's three independent queues:
// download, post (post-processing) or scan (the incoming nzb directory).
func NZBGetSetPaused(ctx context.Context, c *Client, scope string, paused bool) error {
	methods, ok := pauseMethods[scope]
	if !ok {
		return fmt.Errorf("unknown pause scope %q; valid scopes: download, post, scan", scope)
	}
	method := methods[1]
	if paused {
		method = methods[0]
	}
	_, err := nzbCall[bool](ctx, c, method)
	return err
}

// NZBGetRate sets the download speed limit in KiB/s; 0 removes the limit.
func NZBGetRate(ctx context.Context, c *Client, limitKB int) error {
	if limitKB < 0 {
		return fmt.Errorf("rate limit must be 0 (unlimited) or a positive number of KiB/s, got %d", limitKB)
	}
	ok, err := nzbCall[bool](ctx, c, "rate", limitKB)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("nzbget rejected rate limit %d KiB/s as out of range", limitKB)
	}
	return nil
}

// NZBGetScan asks NZBGet to scan its incoming nzb directory now.
func NZBGetScan(ctx context.Context, c *Client) error {
	_, err := nzbCall[bool](ctx, c, "scan")
	return err
}

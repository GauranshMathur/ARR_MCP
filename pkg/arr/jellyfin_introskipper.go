package arr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
)

// Intro Skipper is a third-party Jellyfin plugin that detects intros and
// credits. It serves its own API from the Jellyfin server, behind the same key.
// It publishes no API document: these calls follow its source at 12.0.4.0 and
// 12.0.5.0, checked read-only against a live 12.0.4.0. Its names differ from
// Jellyfin's: Introduction and Credits where Jellyfin's media segments say
// Intro and Outro, and boundaries in seconds rather than ticks.

// introSkipperModes and introSkipperActions are the plugin's AnalysisMode and
// AnalyzerAction names, identical in both versions.
var (
	introSkipperModes   = []string{"Introduction", "Credits", "Preview", "Recap", "Commercial"}
	introSkipperActions = []string{"Default", "Chapter", "Chromaprint", "BlackFrame", "None"}
)

// IntroSkipperSegment is one segment as Intro Skipper stores it.
type IntroSkipperSegment struct {
	ID           string  `json:"id" jsonschema:"pass as segmentId to update, delete or restore this segment"`
	Type         string  `json:"type" jsonschema:"Introduction, Credits, Preview, Recap or Commercial"`
	StartSeconds float64 `json:"startSeconds"`
	EndSeconds   float64 `json:"endSeconds"`
	Source       string  `json:"source" jsonschema:"what produced it: User for one made by hand; Chapter, Chromaprint, BlackFrame, CreditsDerived, KeyframeVisuals or Combined for detected ones"`
	Suppressed   bool    `json:"suppressed" jsonschema:"a deleted detected segment, kept so re-analysis does not add it back; restoring makes it active again"`
}

type rawIntroSkipperSegment struct {
	ID         string  `json:"Id"`
	Type       string  `json:"Type"`
	Start      float64 `json:"Start"`
	End        float64 `json:"End"`
	Source     string  `json:"Source"`
	Suppressed bool    `json:"Suppressed"`
}

func (r rawIntroSkipperSegment) trim() IntroSkipperSegment {
	return IntroSkipperSegment{
		ID: r.ID, Type: r.Type, StartSeconds: r.Start, EndSeconds: r.End, Source: r.Source, Suppressed: r.Suppressed,
	}
}

// IntroSkipperChange is the outcome of a segment write.
type IntroSkipperChange struct {
	Segments   []IntroSkipperSegment `json:"segments" jsonschema:"the segment as stored after the change; empty after a delete"`
	Projection string                `json:"projection" jsonschema:"Applied: Jellyfin's media segments, which playback reads, already show the change. Pending: Intro Skipper saved it and Jellyfin's copy catches up on its own. Skipped: Jellyfin's copy is not updated by this change"`
}

// introSkipperChange reads a write's answer. The status is not available here,
// so the shape tells them apart: a bare segment (200 or 201) or no body (204)
// means Jellyfin's copy already shows the change; a 202 wraps the segments with
// ChangeStatus and the state of that copy.
func introSkipperChange(body []byte) (IntroSkipperChange, error) {
	out := IntroSkipperChange{Segments: []IntroSkipperSegment{}, Projection: "Applied"}
	if len(bytes.TrimSpace(body)) == 0 {
		return out, nil
	}
	var raw struct {
		rawIntroSkipperSegment
		ChangeStatus string                   `json:"ChangeStatus"`
		Projection   string                   `json:"Projection"`
		Segments     []rawIntroSkipperSegment `json:"Segments"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return IntroSkipperChange{}, fmt.Errorf("decoding intro skipper response: %w", err)
	}
	if raw.ChangeStatus == "" {
		out.Segments = append(out.Segments, raw.trim())
		return out, nil
	}
	out.Projection = raw.Projection
	for _, s := range raw.Segments {
		out.Segments = append(out.Segments, s.trim())
	}
	return out, nil
}

// introSkipperSegmentPath checks both ids and returns the segment's route.
func introSkipperSegmentPath(itemID, segmentID string) (string, error) {
	if err := jellyfinCheckID("itemId", itemID); err != nil {
		return "", err
	}
	if err := jellyfinCheckID("segmentId", segmentID); err != nil {
		return "", err
	}
	return "/Episode/" + itemID + "/Segments/" + segmentID, nil
}

// introSkipperBounds refuses what the plugin would answer 400 to.
func introSkipperBounds(start, end float64) error {
	if start < 0 || end <= start {
		return fmt.Errorf("invalid segment boundaries %v to %v seconds: start must be 0 or more and end after start", start, end)
	}
	return nil
}

// jellyfinDashed writes a 32-digit id in the dashed GUID form, which every
// .NET GUID parser accepts, for ids sent in a JSON body rather than a route.
func jellyfinDashed(id string) string {
	if len(id) != 32 {
		return id
	}
	return id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
}

// IntroSkipperListSegments reads /Episode/{id}/Segments: every segment the
// plugin stores for an episode or movie, including ones Jellyfin's media
// segments do not show yet. Deleted detected segments (tombstones) are left
// out unless includeSuppressed is set.
func IntroSkipperListSegments(ctx context.Context, c *Client, itemID string, includeSuppressed bool) ([]IntroSkipperSegment, error) {
	if err := jellyfinCheckID("itemId", itemID); err != nil {
		return nil, err
	}
	var q []Query
	if includeSuppressed {
		q = append(q, Query{"includeSuppressed": "true"})
	}
	raw, err := GetJSON[[]rawIntroSkipperSegment](ctx, c, "/Episode/"+itemID+"/Segments", q...)
	if err != nil {
		return nil, err
	}
	out := make([]IntroSkipperSegment, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.trim())
	}
	return out, nil
}

// IntroSkipperCreateSegment adds a user segment. The plugin resolves a segment
// with exactly the same range by turning the existing one into a user segment.
func IntroSkipperCreateSegment(ctx context.Context, c *Client, itemID, segType string, start, end float64) (IntroSkipperChange, error) {
	if err := jellyfinCheckID("itemId", itemID); err != nil {
		return IntroSkipperChange{}, err
	}
	if !slices.Contains(introSkipperModes, segType) {
		return IntroSkipperChange{}, fmt.Errorf("invalid segment type %q: use %s (Intro Skipper's names, not Jellyfin's Intro and Outro)",
			segType, strings.Join(introSkipperModes, ", "))
	}
	if err := introSkipperBounds(start, end); err != nil {
		return IntroSkipperChange{}, err
	}
	body, err := c.Post(ctx, "/Episode/"+itemID+"/Segments", struct {
		Type  string  `json:"Type"`
		Start float64 `json:"Start"`
		End   float64 `json:"End"`
	}{segType, start, end})
	if err != nil {
		return IntroSkipperChange{}, err
	}
	return introSkipperChange(body)
}

// IntroSkipperUpdateSegment moves a segment's boundaries, which makes it a user
// segment. Moving it exactly onto another of the same type merges the two.
func IntroSkipperUpdateSegment(ctx context.Context, c *Client, itemID, segmentID string, start, end float64) (IntroSkipperChange, error) {
	path, err := introSkipperSegmentPath(itemID, segmentID)
	if err != nil {
		return IntroSkipperChange{}, err
	}
	if err := introSkipperBounds(start, end); err != nil {
		return IntroSkipperChange{}, err
	}
	body, err := c.Put(ctx, path, struct {
		Start float64 `json:"Start"`
		End   float64 `json:"End"`
	}{start, end})
	if err != nil {
		return IntroSkipperChange{}, err
	}
	return introSkipperChange(body)
}

// IntroSkipperDeleteSegment deletes a segment. A detected segment is kept as a
// tombstone so re-analysis does not add it back, and can be restored; a user
// segment is removed for good.
func IntroSkipperDeleteSegment(ctx context.Context, c *Client, itemID, segmentID string) (IntroSkipperChange, error) {
	path, err := introSkipperSegmentPath(itemID, segmentID)
	if err != nil {
		return IntroSkipperChange{}, err
	}
	body, err := c.Delete(ctx, path)
	if err != nil {
		return IntroSkipperChange{}, err
	}
	return introSkipperChange(body)
}

// IntroSkipperRestoreSegment makes a tombstoned segment active again with its
// original source.
func IntroSkipperRestoreSegment(ctx context.Context, c *Client, itemID, segmentID string) (IntroSkipperChange, error) {
	path, err := introSkipperSegmentPath(itemID, segmentID)
	if err != nil {
		return IntroSkipperChange{}, err
	}
	body, err := c.Post(ctx, path+"/Restore", nil)
	if err != nil {
		return IntroSkipperChange{}, err
	}
	return introSkipperChange(body)
}

// IntroSkipperAnalyzers reads which analyzer a season uses for each segment
// type. A season the plugin does not know answers 404.
func IntroSkipperAnalyzers(ctx context.Context, c *Client, seasonID string) (map[string]string, error) {
	if err := jellyfinCheckID("seasonId", seasonID); err != nil {
		return nil, err
	}
	return GetJSON[map[string]string](ctx, c, "/Intros/AnalyzerActions/"+seasonID)
}

// IntroSkipperSetAnalyzers sets a season's analyzer for the named segment
// types and returns the season read back; types not named keep theirs. The
// plugin stores actions for any id without checking it, so the season is read
// first and an unknown one fails instead of being written to.
func IntroSkipperSetAnalyzers(ctx context.Context, c *Client, seasonID string, actions map[string]string) (map[string]string, error) {
	if err := jellyfinCheckID("seasonId", seasonID); err != nil {
		return nil, err
	}
	if len(actions) == 0 {
		return nil, fmt.Errorf(`no analyzers to set: name at least one segment type, e.g. {"Credits":"Chapter"}`)
	}
	modes := make([]string, 0, len(actions))
	for mode := range actions {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	for _, mode := range modes {
		if !slices.Contains(introSkipperModes, mode) {
			return nil, fmt.Errorf("invalid segment type %q: use %s (Intro Skipper's names, not Jellyfin's Intro and Outro)",
				mode, strings.Join(introSkipperModes, ", "))
		}
		if !slices.Contains(introSkipperActions, actions[mode]) {
			return nil, fmt.Errorf("invalid analyzer %q for %s: use %s", actions[mode], mode, strings.Join(introSkipperActions, ", "))
		}
	}

	if _, err := IntroSkipperAnalyzers(ctx, c, seasonID); err != nil {
		return nil, fmt.Errorf("looking up season %s in intro skipper: %w", seasonID, err)
	}
	if _, err := c.Post(ctx, "/Intros/AnalyzerActions/UpdateSeason", struct {
		ID              string            `json:"Id"`
		AnalyzerActions map[string]string `json:"AnalyzerActions"`
	}{jellyfinDashed(seasonID), actions}); err != nil {
		return nil, err
	}
	return IntroSkipperAnalyzers(ctx, c, seasonID)
}

// IntroSkipperScanSeason starts a scan of one season. Both 12.0.4.0 and
// 12.0.5.0 first erase every segment of every episode in the season, user
// segments and tombstones included, with its analysis records and fingerprint
// cache, then analyze it again. 12.0.4.0 runs one scan at a time and answers
// 409 to another; 12.0.5.0 queues it.
func IntroSkipperScanSeason(ctx context.Context, c *Client, seriesID, seasonID string) error {
	if err := jellyfinCheckID("seriesId", seriesID); err != nil {
		return err
	}
	if err := jellyfinCheckID("seasonId", seasonID); err != nil {
		return err
	}
	_, err := c.Post(ctx, "/Intros/ScanSeason/"+seriesID+"/"+seasonID, nil)
	var se *StatusError
	if errors.As(err, &se) && se.Status == http.StatusConflict {
		return fmt.Errorf("intro skipper is already scanning and this version does not queue another; "+
			"wait until jellyfin_get_introskipper_scan_status reports isRunning false: %w", err)
	}
	return err
}

// IntroSkipperScanState is whether Intro Skipper is analyzing.
type IntroSkipperScanState struct {
	Scope     string `json:"scope" jsonschema:"season on 12.0.5.0 and later; server on 12.0.4.0, which reports only whether any scan runs"`
	IsRunning bool   `json:"isRunning" jsonschema:"an analysis pass is running on the server"`
	IsQueued  *bool  `json:"isQueued,omitempty" jsonschema:"this season's scan is waiting its turn; season scope only"`
	Failed    *bool  `json:"failed,omitempty" jsonschema:"this season's last scan failed; season scope only"`
}

// IntroSkipperScanStatus reports scan state. 12.0.5.0 serves it per season;
// 12.0.4.0 has only the server-wide route, so a 404 on the first falls back to
// the second.
func IntroSkipperScanStatus(ctx context.Context, c *Client, seasonID string) (IntroSkipperScanState, error) {
	if err := jellyfinCheckID("seasonId", seasonID); err != nil {
		return IntroSkipperScanState{}, err
	}
	type rawState struct {
		IsRunning bool  `json:"isRunning"`
		IsQueued  *bool `json:"isQueued"`
		Failed    *bool `json:"failed"`
	}
	r, err := GetJSON[rawState](ctx, c, "/Intros/ScanStatus/"+seasonID)
	if err == nil {
		return IntroSkipperScanState{Scope: "season", IsRunning: r.IsRunning, IsQueued: r.IsQueued, Failed: r.Failed}, nil
	}
	var se *StatusError
	if !errors.As(err, &se) || se.Status != http.StatusNotFound {
		return IntroSkipperScanState{}, err
	}
	if r, err = GetJSON[rawState](ctx, c, "/Intros/ScanStatus"); err != nil {
		return IntroSkipperScanState{}, err
	}
	return IntroSkipperScanState{Scope: "server", IsRunning: r.IsRunning}, nil
}

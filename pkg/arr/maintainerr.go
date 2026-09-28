package arr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Maintainerr answers an unknown id with 200 and an empty body rather than a
// 404, and reports some failures as 201 with {"code":0}. Both quirks are
// handled here so no tool reports success for something that did not happen.

// maintainerrSlowTimeout bounds calls that queue behind Maintainerr's
// execution lock for up to 30s before doing their own work, and overlay runs,
// which render every poster before answering. Timing out on the default would
// report a failure while the change still lands, inviting a duplicate retry.
const maintainerrSlowTimeout = 5 * time.Minute

// maintainerrArrActions names Maintainerr's ServarrAction enum by index.
var maintainerrArrActions = []string{
	"DELETE",
	"UNMONITOR_DELETE_ALL",
	"UNMONITOR_DELETE_EXISTING",
	"UNMONITOR",
	"DO_NOTHING",
	"DELETE_SHOW_IF_EMPTY",
	"UNMONITOR_SHOW_IF_EMPTY",
	"CHANGE_QUALITY_PROFILE",
}

// maintainerrArrAction names an arrAction value, keeping unknown values visible
// rather than guessing, since a newer Maintainerr may add actions.
func maintainerrArrAction(v int) string {
	if v >= 0 && v < len(maintainerrArrActions) {
		return maintainerrArrActions[v]
	}
	return fmt.Sprintf("UNKNOWN(%d)", v)
}

// MaintainerrStatus reports a Maintainerr instance's version.
type MaintainerrStatus struct {
	Version         string `json:"version"`
	CommitTag       string `json:"commitTag,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

// MaintainerrGetStatus reads /app/status, which serves JSON under a text/html
// content type.
func MaintainerrGetStatus(ctx context.Context, c *Client) (MaintainerrStatus, error) {
	return GetJSON[MaintainerrStatus](ctx, c, "/app/status")
}

// MaintainerrCollection is the trimmed view of a Maintainerr collection: the
// set of media one rule group has marked, and what happens to it.
type MaintainerrCollection struct {
	ID                    int    `json:"id"`
	Title                 string `json:"title"`
	Type                  string `json:"type" jsonschema:"movie, show, season or episode"`
	LibraryID             string `json:"libraryId,omitempty"`
	IsActive              bool   `json:"isActive"`
	ArrAction             string `json:"arrAction" jsonschema:"what happens when the grace period ends; DELETE removes the files from disk through Sonarr or Radarr"`
	DeleteAfterDays       *int   `json:"deleteAfterDays" jsonschema:"grace period in days between an item entering the collection and its arrAction; null means never"`
	MediaCount            int    `json:"mediaCount" jsonschema:"items currently in the collection"`
	HandledMediaAmount    int    `json:"handledMediaAmount" jsonschema:"items already acted on"`
	HandledMediaSizeBytes int64  `json:"handledMediaSizeBytes,omitempty"`
	OverlayEnabled        bool   `json:"overlayEnabled"`
}

// rawMaintainerrCollection is the upstream collection, which also carries
// server-specific settings and, on some endpoints, every media row.
type rawMaintainerrCollection struct {
	ID                    int    `json:"id"`
	Title                 string `json:"title"`
	Type                  string `json:"type"`
	LibraryID             string `json:"libraryId"`
	IsActive              bool   `json:"isActive"`
	ArrAction             int    `json:"arrAction"`
	DeleteAfterDays       *int   `json:"deleteAfterDays"`
	MediaCount            int    `json:"mediaCount"`
	HandledMediaAmount    int    `json:"handledMediaAmount"`
	HandledMediaSizeBytes int64  `json:"handledMediaSizeBytes"`
	OverlayEnabled        bool   `json:"overlayEnabled"`
}

func (r rawMaintainerrCollection) trim() MaintainerrCollection {
	return MaintainerrCollection{
		ID: r.ID, Title: r.Title, Type: r.Type, LibraryID: r.LibraryID,
		IsActive: r.IsActive, ArrAction: maintainerrArrAction(r.ArrAction),
		DeleteAfterDays: r.DeleteAfterDays, MediaCount: r.MediaCount,
		HandledMediaAmount: r.HandledMediaAmount, HandledMediaSizeBytes: r.HandledMediaSizeBytes,
		OverlayEnabled: r.OverlayEnabled,
	}
}

// MaintainerrListCollections lists every collection. The spec marks libraryId
// and typeId as required, but the live API answers without them.
func MaintainerrListCollections(ctx context.Context, c *Client) ([]MaintainerrCollection, error) {
	raw, err := GetJSON[[]rawMaintainerrCollection](ctx, c, "/collections")
	if err != nil {
		return nil, err
	}
	out := make([]MaintainerrCollection, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.trim())
	}
	return out, nil
}

// maintainerrGetOne fetches a single record, turning Maintainerr's empty 200
// for an unknown id into a not-found error naming what was looked up.
func maintainerrGetOne[T any](ctx context.Context, c *Client, path, what string, id int) (T, error) {
	var out T
	body, err := c.Get(ctx, path)
	if err != nil {
		return out, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return out, fmt.Errorf("no %s with id %d", what, id)
	}
	return out, unmarshal(body, &out)
}

// MaintainerrMediaItem is one item in a collection, awaiting its arrAction.
type MaintainerrMediaItem struct {
	MediaServerID    string `json:"mediaServerId" jsonschema:"media server item id; pass to the exclusion and postpone tools"`
	Title            string `json:"title"`
	ParentTitle      string `json:"parentTitle,omitempty"`
	GrandparentTitle string `json:"grandparentTitle,omitempty"`
	Type             string `json:"type,omitempty"`
	Year             int    `json:"year,omitempty"`
	TMDBID           int    `json:"tmdbId,omitempty"`
	TVDBID           int    `json:"tvdbId,omitempty"`
	AddDate          string `json:"addDate" jsonschema:"when the item entered the collection; the grace period counts from here"`
	IsManual         bool   `json:"isManual,omitempty" jsonschema:"added by hand rather than by a rule"`
	SizeBytes        int64  `json:"sizeBytes,omitempty"`
}

// rawMaintainerrMediaItem is a collection media row with the media server's
// metadata attached, which also carries summary, cast, genres and sources.
type rawMaintainerrMediaItem struct {
	MediaServerID string `json:"mediaServerId"`
	TMDBID        *int   `json:"tmdbId"`
	TVDBID        *int   `json:"tvdbId"`
	AddDate       string `json:"addDate"`
	IsManual      bool   `json:"isManual"`
	SizeBytes     *int64 `json:"sizeBytes"`
	MediaData     *struct {
		Title            string `json:"title"`
		ParentTitle      string `json:"parentTitle"`
		GrandparentTitle string `json:"grandparentTitle"`
		Type             string `json:"type"`
		Year             int    `json:"year"`
	} `json:"mediaData"`
}

// MaintainerrMediaPage is one page of a collection's pending items.
type MaintainerrMediaPage struct {
	CollectionID    int                    `json:"collectionId"`
	CollectionTitle string                 `json:"collectionTitle"`
	ArrAction       string                 `json:"arrAction"`
	DeleteAfterDays *int                   `json:"deleteAfterDays" jsonschema:"grace period in days; an item's arrAction runs this many days after its addDate"`
	Items           []MaintainerrMediaItem `json:"items"`
	Count           int                    `json:"count"`
	Total           int                    `json:"total" jsonschema:"items in the collection across all pages"`
}

// MaintainerrCollectionMedia lists one page of a collection's items. The page
// does not carry the grace period, so the collection is read first; that also
// turns an unknown collection id into an error instead of an empty page.
func MaintainerrCollectionMedia(ctx context.Context, c *Client, collectionID, page, size int) (MaintainerrMediaPage, error) {
	col, err := maintainerrGetOne[rawMaintainerrCollection](ctx, c,
		"/collections/collection/"+itoa(collectionID), "collection", collectionID)
	if err != nil {
		return MaintainerrMediaPage{}, err
	}

	raw, err := GetJSON[struct {
		TotalSize int                       `json:"totalSize"`
		Items     []rawMaintainerrMediaItem `json:"items"`
	}](ctx, c, "/collections/media/"+itoa(collectionID)+"/content/"+itoa(page), Query{"size": itoa(size)})
	if err != nil {
		return MaintainerrMediaPage{}, err
	}

	items := make([]MaintainerrMediaItem, 0, len(raw.Items))
	for _, r := range raw.Items {
		item := MaintainerrMediaItem{MediaServerID: r.MediaServerID, AddDate: r.AddDate, IsManual: r.IsManual}
		if r.TMDBID != nil {
			item.TMDBID = *r.TMDBID
		}
		if r.TVDBID != nil {
			item.TVDBID = *r.TVDBID
		}
		if r.SizeBytes != nil {
			item.SizeBytes = *r.SizeBytes
		}
		if md := r.MediaData; md != nil {
			item.Title, item.ParentTitle, item.GrandparentTitle = md.Title, md.ParentTitle, md.GrandparentTitle
			item.Type, item.Year = md.Type, md.Year
		}
		items = append(items, item)
	}
	return MaintainerrMediaPage{
		CollectionID: col.ID, CollectionTitle: col.Title,
		ArrAction: maintainerrArrAction(col.ArrAction), DeleteAfterDays: col.DeleteAfterDays,
		Items: items, Count: len(items), Total: raw.TotalSize,
	}, nil
}

// MaintainerrRule is the trimmed view of a rule group: the rules that fill one
// collection. Notification agents are deliberately absent, because their
// options hold webhook URLs and tokens.
type MaintainerrRule struct {
	ID                      int     `json:"id"`
	Name                    string  `json:"name"`
	Description             string  `json:"description,omitempty"`
	LibraryID               string  `json:"libraryId,omitempty"`
	CollectionID            int     `json:"collectionId"`
	IsActive                bool    `json:"isActive"`
	DataType                string  `json:"dataType" jsonschema:"movie, show, season or episode"`
	UseRules                bool    `json:"useRules"`
	RuleHandlerCronSchedule *string `json:"ruleHandlerCronSchedule" jsonschema:"per-group schedule; null means the global rule handler schedule"`
	RuleCount               int     `json:"ruleCount"`
}

// MaintainerrRuleCondition is one condition in a rule group.
type MaintainerrRuleCondition struct {
	ID       int    `json:"id"`
	Section  int    `json:"section" jsonschema:"conditions in the same section combine; sections combine with each other"`
	IsActive bool   `json:"isActive"`
	RuleJSON string `json:"ruleJson" jsonschema:"Maintainerr's encoded condition: firstVal and lastVal are [application, property] ids, action is the comparison"`
}

// MaintainerrRuleDetail is a rule group with its conditions and collection.
type MaintainerrRuleDetail struct {
	MaintainerrRule
	ArrAction       string                     `json:"arrAction"`
	DeleteAfterDays *int                       `json:"deleteAfterDays"`
	Rules           []MaintainerrRuleCondition `json:"rules"`
	RulesYAML       string                     `json:"rulesYaml,omitempty" jsonschema:"the conditions in Maintainerr's YAML, the format maintainerr_create_rule and maintainerr_update_rule take"`
}

// rawMaintainerrRule is the upstream rule group. It omits notifications on
// purpose: an unmapped field is never decoded, so it can never be re-encoded.
type rawMaintainerrRule struct {
	ID                      int                        `json:"id"`
	Name                    string                     `json:"name"`
	Description             string                     `json:"description"`
	LibraryID               string                     `json:"libraryId"`
	CollectionID            int                        `json:"collectionId"`
	IsActive                bool                       `json:"isActive"`
	DataType                string                     `json:"dataType"`
	UseRules                bool                       `json:"useRules"`
	RuleHandlerCronSchedule *string                    `json:"ruleHandlerCronSchedule"`
	Rules                   []MaintainerrRuleCondition `json:"rules"`
	Collection              *rawMaintainerrCollection  `json:"collection"`
}

func (r rawMaintainerrRule) trim() MaintainerrRule {
	return MaintainerrRule{
		ID: r.ID, Name: r.Name, Description: r.Description, LibraryID: r.LibraryID,
		CollectionID: r.CollectionID, IsActive: r.IsActive, DataType: r.DataType,
		UseRules: r.UseRules, RuleHandlerCronSchedule: r.RuleHandlerCronSchedule,
		RuleCount: len(r.Rules),
	}
}

// MaintainerrListRules lists every rule group.
func MaintainerrListRules(ctx context.Context, c *Client) ([]MaintainerrRule, error) {
	raw, err := GetJSON[[]rawMaintainerrRule](ctx, c, "/rules")
	if err != nil {
		return nil, err
	}
	out := make([]MaintainerrRule, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.trim())
	}
	return out, nil
}

// MaintainerrGetRule reads one rule group with its conditions.
func MaintainerrGetRule(ctx context.Context, c *Client, id int) (MaintainerrRuleDetail, error) {
	r, err := maintainerrGetOne[rawMaintainerrRule](ctx, c, "/rules/"+itoa(id), "rule group", id)
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}
	out := MaintainerrRuleDetail{MaintainerrRule: r.trim(), Rules: r.Rules}
	if r.Collection != nil {
		out.ArrAction = maintainerrArrAction(r.Collection.ArrAction)
		out.DeleteAfterDays = r.Collection.DeleteAfterDays
	}
	if len(r.Rules) > 0 {
		rules := make([]json.RawMessage, 0, len(r.Rules))
		for _, rule := range r.Rules {
			rules = append(rules, json.RawMessage(rule.RuleJSON))
		}
		yaml, err := maintainerrEncodeRules(ctx, c, rules, r.DataType)
		if err != nil {
			return MaintainerrRuleDetail{}, fmt.Errorf("rendering rule group %d as YAML: %w", id, err)
		}
		out.RulesYAML = yaml
	}
	return out, nil
}

// MaintainerrExclusion is an item Maintainerr will never act on.
type MaintainerrExclusion struct {
	ID            int     `json:"id" jsonschema:"exclusion id; pass to maintainerr_remove_exclusion"`
	MediaServerID string  `json:"mediaServerId"`
	RuleGroupID   *int    `json:"ruleGroupId" jsonschema:"rule group the exclusion applies to; null means every rule group"`
	Parent        *string `json:"parent,omitempty" jsonschema:"mediaServerId of the show or season this was cascaded from"`
	Type          *string `json:"type,omitempty"`
}

// MaintainerrListExclusions lists exclusions, optionally for one rule group
// (which also includes global exclusions) or one media item.
func MaintainerrListExclusions(ctx context.Context, c *Client, ruleGroupID int, mediaServerID string) ([]MaintainerrExclusion, error) {
	q := Query{}
	if ruleGroupID > 0 {
		q["rulegroupId"] = itoa(ruleGroupID)
	}
	if mediaServerID != "" {
		q["mediaServerId"] = mediaServerID
	}
	return GetJSON[[]MaintainerrExclusion](ctx, c, "/rules/exclusion", q)
}

// maintainerrReturnStatus is Maintainerr's result envelope, where code 0 means
// the operation failed despite a success status.
type maintainerrReturnStatus struct {
	Code    int    `json:"code"`
	Result  string `json:"result"`
	Message string `json:"message"`
	// Skipped counts rules a YAML decode could not resolve.
	Skipped int `json:"skipped"`
}

// maintainerrDecodeStatus decodes Maintainerr's result envelope and turns a
// code-0 answer into an error naming the action that was refused.
func maintainerrDecodeStatus(body []byte, action string) (maintainerrReturnStatus, error) {
	var st maintainerrReturnStatus
	if err := unmarshal(body, &st); err != nil {
		return st, err
	}
	if st.Code == 1 {
		return st, nil
	}
	reason := st.Result
	if st.Message != "" {
		reason += ": " + st.Message
	}
	return st, fmt.Errorf("maintainerr refused to %s: %s", action, reason)
}

// maintainerrCheck turns a code-0 envelope into an error.
func maintainerrCheck(body []byte, action string) error {
	_, err := maintainerrDecodeStatus(body, action)
	return err
}

// MaintainerrAddExclusion excludes an item from one collection's rule group,
// or from every rule group when collectionID is 0. Show exclusions cascade to
// their seasons and episodes.
func MaintainerrAddExclusion(ctx context.Context, c *Client, mediaID string, collectionID int) error {
	body, err := c.WithTimeout(maintainerrSlowTimeout).Post(ctx, "/rules/exclusion", struct {
		MediaID      string `json:"mediaId"`
		CollectionID int    `json:"collectionId,omitempty"`
	}{mediaID, collectionID})
	if err != nil {
		return err
	}
	return maintainerrCheck(body, "add the exclusion")
}

// MaintainerrRemoveExclusion deletes one exclusion row by id.
func MaintainerrRemoveExclusion(ctx context.Context, c *Client, id int) error {
	body, err := c.Delete(ctx, "/rules/exclusion/"+itoa(id))
	if err != nil {
		return err
	}
	return maintainerrCheck(body, "remove the exclusion")
}

// MaintainerrExecuteRules queues rule evaluation for one rule group, or every
// active one when ruleGroupID is 0. Evaluation updates collection membership;
// it does not run arrActions, which the collection handler does on its own
// schedule.
func MaintainerrExecuteRules(ctx context.Context, c *Client, ruleGroupID int) error {
	path := "/rules/execute"
	if ruleGroupID > 0 {
		path = "/rules/" + itoa(ruleGroupID) + "/execute"
	}
	_, err := c.Post(ctx, path, nil)
	return err
}

// MaintainerrRuleStatus reports whether rule evaluation is running.
type MaintainerrRuleStatus struct {
	ProcessingQueue      bool  `json:"processingQueue"`
	ExecutingRuleGroupID *int  `json:"executingRuleGroupId"`
	PendingRuleGroupIDs  []int `json:"pendingRuleGroupIds"`
}

// MaintainerrRuleExecutionStatus reads the rule executor's queue state.
func MaintainerrRuleExecutionStatus(ctx context.Context, c *Client) (MaintainerrRuleStatus, error) {
	return GetJSON[MaintainerrRuleStatus](ctx, c, "/rules/execute/status")
}

// MaintainerrPostponeResult reports an item's new deletion schedule.
type MaintainerrPostponeResult struct {
	CollectionID    int    `json:"collectionId"`
	MediaServerID   string `json:"mediaServerId"`
	AddDate         string `json:"addDate"`
	DeleteAfterDays *int   `json:"deleteAfterDays"`
	DeletionDate    string `json:"deletionDate,omitempty"`
}

// MaintainerrPostpone pushes an item's deletion out by days, or restarts its
// full grace period when days is 0.
func MaintainerrPostpone(ctx context.Context, c *Client, collectionID int, mediaID string, days int) (MaintainerrPostponeResult, error) {
	var out MaintainerrPostponeResult
	body, err := c.WithTimeout(maintainerrSlowTimeout).Post(ctx, "/collections/media/postpone", struct {
		CollectionID int    `json:"collectionId"`
		MediaID      string `json:"mediaId"`
		Days         int    `json:"days,omitempty"`
	}{collectionID, mediaID, days})
	if err != nil {
		return out, err
	}
	return out, unmarshal(body, &out)
}

// MaintainerrOverlayRun counts what an overlay run did.
type MaintainerrOverlayRun struct {
	Processed int `json:"processed"`
	Reverted  int `json:"reverted"`
	Skipped   int `json:"skipped"`
	Errors    int `json:"errors"`
}

// MaintainerrOverlayState reports the scheduled overlay job.
type MaintainerrOverlayState struct {
	Status     string                 `json:"status"`
	LastRun    *string                `json:"lastRun" jsonschema:"last scheduled run only; stays null after manual per-collection runs"`
	LastResult *MaintainerrOverlayRun `json:"lastResult"`
}

// MaintainerrOverlayStatus reads the overlay processor's state.
func MaintainerrOverlayStatus(ctx context.Context, c *Client) (MaintainerrOverlayState, error) {
	return GetJSON[MaintainerrOverlayState](ctx, c, "/overlays/status")
}

// MaintainerrProcessOverlays renders overlays for one collection and waits for
// the run to finish.
func MaintainerrProcessOverlays(ctx context.Context, c *Client, collectionID int) (MaintainerrOverlayRun, error) {
	var out MaintainerrOverlayRun
	body, err := c.WithTimeout(maintainerrSlowTimeout).Post(ctx, "/overlays/process/"+itoa(collectionID), nil)
	if err != nil {
		return out, err
	}
	return out, unmarshal(body, &out)
}

// MaintainerrStatusEntry names one collection or exclusion an item belongs to.
type MaintainerrStatusEntry struct {
	Label      string `json:"label"`
	TargetPath string `json:"targetPath,omitempty"`
}

// MaintainerrItemStatus reports what Maintainerr holds about one media item.
type MaintainerrItemStatus struct {
	ExcludedFrom    []MaintainerrStatusEntry `json:"excludedFrom"`
	ManuallyAddedTo []MaintainerrStatusEntry `json:"manuallyAddedTo"`
}

// MaintainerrMediaStatus reports the exclusions and manual collections that
// apply to a media item, including those inherited from its show or season.
func MaintainerrMediaStatus(ctx context.Context, c *Client, mediaServerID string) (MaintainerrItemStatus, error) {
	if strings.Contains(mediaServerID, "/") || mediaServerID == "" || mediaServerID == ".." {
		// The id becomes a path segment. A slash would let it walk to another
		// endpoint, such as /api/settings, which returns every secret in plaintext.
		return MaintainerrItemStatus{}, fmt.Errorf("invalid mediaServerId %q: want a single media server item id", mediaServerID)
	}
	return GetJSON[MaintainerrItemStatus](ctx, c, "/media-server/meta/"+mediaServerID+"/maintainerr-status")
}

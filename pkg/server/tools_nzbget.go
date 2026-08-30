package server

import "github.com/GauranshMathur/ARR_MCP/pkg/arr"

// --- nzbget tool inputs ---

// NZBHistoryArgs is the input for nzbget_history.
type NZBHistoryArgs struct {
	InstanceArg
	Hidden bool `json:"hidden,omitempty" jsonschema:"also return hidden records: entries removed with nzbget_delete_history_items (non-final) and old duplicates NZBGet keeps for dupe checking"`
	Limit  int  `json:"limit,omitempty" jsonschema:"maximum records to return, newest first; defaults to 20"`
}

// AddNZBArgs is the input for nzbget_add_nzb.
type AddNZBArgs struct {
	InstanceArg
	URL       string `json:"url,omitempty" jsonschema:"link to an .nzb file for NZBGet to fetch; exactly one of url and content is required"`
	Content   string `json:"content,omitempty" jsonschema:"base64-encoded .nzb file content, as an alternative to url"`
	Filename  string `json:"filename,omitempty" jsonschema:"name ending in .nzb; required with content, optional with url"`
	Category  string `json:"category,omitempty" jsonschema:"NZBGet category, e.g. Movies or Series"`
	Priority  int    `json:"priority,omitempty" jsonschema:"-100 very low, -50 low, 0 normal, 50 high, 100 very high, 900 force (downloads even while paused)"`
	AddToTop  bool   `json:"addToTop,omitempty" jsonschema:"put the download at the top of the queue"`
	AddPaused bool   `json:"addPaused,omitempty" jsonschema:"add in a paused state"`
	DupeKey   string `json:"dupeKey,omitempty" jsonschema:"duplicate detection key, e.g. an imdb or tvdb id"`
	DupeScore int    `json:"dupeScore,omitempty" jsonschema:"quality score for duplicate resolution; higher wins"`
	DupeMode  string `json:"dupeMode,omitempty" jsonschema:"SCORE, ALL or FORCE; defaults to SCORE"`
}

// PauseScopeArgs is the input for nzbget_pause_download and
// nzbget_resume_download.
type PauseScopeArgs struct {
	InstanceArg
	Scope string `json:"scope,omitempty" jsonschema:"what to pause or resume: download (the queue), post (post-processing) or scan (the incoming nzb directory); defaults to download"`
}

// NZBIDsArgs is the input for tools acting on queue entries by id.
type NZBIDsArgs struct {
	InstanceArg
	IDs []int `json:"ids" jsonschema:"NZBID values from nzbget_list_queue"`
}

// NZBMoveArgs is the input for nzbget_move_items.
type NZBMoveArgs struct {
	InstanceArg
	IDs      []int  `json:"ids" jsonschema:"NZBID values from nzbget_list_queue"`
	Position string `json:"position" jsonschema:"top, bottom or offset"`
	Offset   int    `json:"offset,omitempty" jsonschema:"positions to move when position is offset; negative moves towards the top"`
}

// NZBPriorityArgs is the input for nzbget_set_priority.
type NZBPriorityArgs struct {
	InstanceArg
	IDs      []int `json:"ids" jsonschema:"NZBID values from nzbget_list_queue"`
	Priority int   `json:"priority" jsonschema:"-100 very low, -50 low, 0 normal, 50 high, 100 very high, 900 force (downloads even while paused)"`
}

// NZBCategoryArgs is the input for nzbget_set_category.
type NZBCategoryArgs struct {
	InstanceArg
	IDs      []int  `json:"ids" jsonschema:"NZBID values from nzbget_list_queue"`
	Category string `json:"category" jsonschema:"category name; controls the completed-download folder and post-processing"`
}

// NZBRenameArgs is the input for nzbget_rename_item.
type NZBRenameArgs struct {
	InstanceArg
	ID   int    `json:"id" jsonschema:"NZBID from nzbget_list_queue"`
	Name string `json:"name" jsonschema:"new display name for the download"`
}

// NZBRetryArgs is the input for nzbget_retry_history_items.
type NZBRetryArgs struct {
	InstanceArg
	IDs        []int `json:"ids" jsonschema:"NZBID values from nzbget_history"`
	Redownload bool  `json:"redownload,omitempty" jsonschema:"fetch everything again instead of returning the item with its already-downloaded data"`
}

// NZBMarkArgs is the input for nzbget_mark_history_items.
type NZBMarkArgs struct {
	InstanceArg
	IDs  []int  `json:"ids" jsonschema:"NZBID values from nzbget_history"`
	Mark string `json:"mark" jsonschema:"good or bad; bad also downloads the next duplicate if one is queued"`
}

// RateLimitArgs is the input for nzbget_set_rate_limit.
type RateLimitArgs struct {
	InstanceArg
	LimitKB int `json:"limitKB" jsonschema:"download speed limit in KiB/s; 0 removes the limit"`
}

// NZBDeleteArgs is the input for the NZBGet deletion tools.
type NZBDeleteArgs struct {
	InstanceArg
	IDs   []int `json:"ids" jsonschema:"NZBID values of the items to delete"`
	Final bool  `json:"final,omitempty" jsonschema:"delete permanently instead of the recoverable default; see the tool description"`
}

// --- nzbget tool outputs ---

// NZBGroupList wraps download queue entries.
type NZBGroupList struct {
	Groups []arr.NZBGroup `json:"groups"`
	Count  int            `json:"count"`
}

// NZBHistoryList wraps NZBGet history entries.
type NZBHistoryList struct {
	Items []arr.NZBHistoryItem `json:"items"`
	Count int                  `json:"count"`
}

// NZBAdded reports the queue id of a newly added nzb.
type NZBAdded struct {
	NZBID int `json:"nzbId" jsonschema:"queue id of the added download; use it with the nzbget queue tools"`
}

// PauseState reports which NZBGet queue was paused or resumed.
type PauseState struct {
	Scope  string `json:"scope"`
	Paused bool   `json:"paused"`
}

// RateLimit reports the download speed limit that was applied.
type RateLimit struct {
	LimitKB int `json:"limitKB" jsonschema:"applied limit in KiB/s; 0 means unlimited"`
}

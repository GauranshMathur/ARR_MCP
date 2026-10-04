package server

import "github.com/GauranshMathur/ARR_MCP/pkg/arr"

// --- jellyfin tool inputs ---

// JellyfinSearchItemsArgs filters jellyfin_search_items.
type JellyfinSearchItemsArgs struct {
	InstanceArg
	SearchTerm       string `json:"searchTerm,omitempty" jsonschema:"text to look for in item names"`
	IncludeItemTypes string `json:"includeItemTypes,omitempty" jsonschema:"comma-separated item types, e.g. Movie, Series, Episode"`
	ParentID         string `json:"parentId,omitempty" jsonschema:"only items under this id, e.g. a library's itemId from jellyfin_list_libraries"`
	Recursive        *bool  `json:"recursive,omitempty" jsonschema:"search below folders and seasons too; defaults to true"`
	Limit            int    `json:"limit,omitempty" jsonschema:"items per page; defaults to 25, at most 100"`
	StartIndex       int    `json:"startIndex,omitempty" jsonschema:"offset of the first item, for paging; defaults to 0"`
}

// JellyfinItemArgs identifies one Jellyfin item.
type JellyfinItemArgs struct {
	InstanceArg
	ItemID string `json:"itemId" jsonschema:"item id from jellyfin_search_items"`
	UserID string `json:"userId,omitempty" jsonschema:"user whose play state to report, from jellyfin_list_users; defaults to the first enabled user"`
}

// JellyfinActivityArgs pages jellyfin_activity_log.
type JellyfinActivityArgs struct {
	InstanceArg
	Limit      int `json:"limit,omitempty" jsonschema:"entries per page, newest first; defaults to 25, at most 100"`
	StartIndex int `json:"startIndex,omitempty" jsonschema:"offset of the first entry, for paging; defaults to 0"`
}

// JellyfinRefreshItemArgs selects the item and depth of a refresh.
type JellyfinRefreshItemArgs struct {
	InstanceArg
	ItemID string `json:"itemId" jsonschema:"item id from jellyfin_search_items or a library itemId"`
	Mode   string `json:"mode,omitempty" jsonschema:"Default scans for new and changed files; FullRefresh also searches for missing metadata and images. Existing metadata is never replaced. Defaults to Default"`
}

// JellyfinTaskArgs identifies one scheduled task.
type JellyfinTaskArgs struct {
	InstanceArg
	TaskID string `json:"taskId" jsonschema:"task id from jellyfin_list_tasks"`
}

// --- jellyfin tool outputs ---

// JellyfinLibraryList wraps library results.
type JellyfinLibraryList struct {
	Libraries []arr.JellyfinLibrary `json:"libraries"`
	Count     int                   `json:"count"`
}

// JellyfinSessionList wraps session results.
type JellyfinSessionList struct {
	Sessions []arr.JellyfinSession `json:"sessions"`
	Count    int                   `json:"count"`
}

// JellyfinUserList wraps user results.
type JellyfinUserList struct {
	Users []arr.JellyfinUser `json:"users"`
	Count int                `json:"count"`
}

// JellyfinTaskList wraps scheduled task results.
type JellyfinTaskList struct {
	Tasks []arr.JellyfinTask `json:"tasks"`
	Count int                `json:"count"`
}

// JellyfinQueued reports that Jellyfin accepted a job. Jellyfin answers 204 and
// runs the job in the background, so success means queued, not finished.
type JellyfinQueued struct {
	Action string `json:"action"`
	ID     string `json:"id,omitempty" jsonschema:"the item or task the job applies to"`
	Queued bool   `json:"queued"`
}

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

// JellyfinSegmentsArgs identifies the item whose media segments to list.
type JellyfinSegmentsArgs struct {
	InstanceArg
	ItemID string `json:"itemId" jsonschema:"an episode or movie id from jellyfin_search_items"`
}

// JellyfinMarkersArgs pages jellyfin_list_episode_markers.
type JellyfinMarkersArgs struct {
	InstanceArg
	ParentID   string `json:"parentId" jsonschema:"a series or season id from jellyfin_search_items; every episode below it is listed"`
	Limit      int    `json:"limit,omitempty" jsonschema:"episodes per page; defaults to 25, at most 100"`
	StartIndex int    `json:"startIndex,omitempty" jsonschema:"offset of the first episode, for paging; defaults to 0"`
}

// JellyfinIntroSkipperSegmentsArgs identifies the item whose Intro Skipper
// segments to list.
type JellyfinIntroSkipperSegmentsArgs struct {
	InstanceArg
	ItemID            string `json:"itemId" jsonschema:"an episode or movie id from jellyfin_search_items"`
	IncludeSuppressed bool   `json:"includeSuppressed,omitempty" jsonschema:"also list deleted detected segments, which jellyfin_restore_introskipper_segment can bring back"`
}

// JellyfinCreateSegmentArgs describes a hand-made Intro Skipper segment.
type JellyfinCreateSegmentArgs struct {
	InstanceArg
	ItemID       string  `json:"itemId" jsonschema:"an episode or movie id from jellyfin_search_items"`
	Type         string  `json:"type" jsonschema:"Introduction, Credits, Preview, Recap or Commercial: Intro Skipper's names, not Jellyfin's Intro and Outro"`
	StartSeconds float64 `json:"startSeconds" jsonschema:"0 or more"`
	EndSeconds   float64 `json:"endSeconds" jsonschema:"after startSeconds"`
}

// JellyfinUpdateSegmentArgs moves an Intro Skipper segment.
type JellyfinUpdateSegmentArgs struct {
	InstanceArg
	ItemID       string  `json:"itemId" jsonschema:"the episode or movie the segment belongs to"`
	SegmentID    string  `json:"segmentId" jsonschema:"segment id from jellyfin_list_introskipper_segments"`
	StartSeconds float64 `json:"startSeconds" jsonschema:"0 or more"`
	EndSeconds   float64 `json:"endSeconds" jsonschema:"after startSeconds"`
}

// JellyfinSegmentRefArgs identifies one Intro Skipper segment.
type JellyfinSegmentRefArgs struct {
	InstanceArg
	ItemID    string `json:"itemId" jsonschema:"the episode or movie the segment belongs to"`
	SegmentID string `json:"segmentId" jsonschema:"segment id from jellyfin_list_introskipper_segments"`
}

// JellyfinSeasonArgs identifies one season.
type JellyfinSeasonArgs struct {
	InstanceArg
	SeasonID string `json:"seasonId" jsonschema:"a season id: jellyfin_search_items with includeItemTypes Season and the series as parentId"`
}

// JellyfinSetAnalyzersArgs chooses a season's analyzers.
type JellyfinSetAnalyzersArgs struct {
	InstanceArg
	SeasonID string            `json:"seasonId" jsonschema:"a season id: jellyfin_search_items with includeItemTypes Season and the series as parentId"`
	Actions  map[string]string `json:"actions" jsonschema:"segment type to analyzer, e.g. {\"Credits\":\"Chapter\"}. Types: Introduction, Credits, Preview, Recap, Commercial. Analyzers: Default, Chapter, Chromaprint, BlackFrame, None. Types not named keep theirs"`
}

// JellyfinScanSeasonArgs identifies the season to re-analyze.
type JellyfinScanSeasonArgs struct {
	InstanceArg
	SeriesID string `json:"seriesId" jsonschema:"the series id"`
	SeasonID string `json:"seasonId" jsonschema:"a season of that series: jellyfin_search_items with includeItemTypes Season and the series as parentId"`
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

// JellyfinSegmentList wraps one item's media segments.
type JellyfinSegmentList struct {
	ItemID   string                     `json:"itemId"`
	Segments []arr.JellyfinMediaSegment `json:"segments"`
	Count    int                        `json:"count"`
}

// JellyfinIntroSkipperSegmentList wraps one item's Intro Skipper segments.
type JellyfinIntroSkipperSegmentList struct {
	ItemID   string                    `json:"itemId"`
	Segments []arr.IntroSkipperSegment `json:"segments"`
	Count    int                       `json:"count"`
}

// JellyfinIntroSkipperAnalyzers is a season's analyzer per segment type.
type JellyfinIntroSkipperAnalyzers struct {
	SeasonID string            `json:"seasonId"`
	Actions  map[string]string `json:"actions" jsonschema:"segment type to the analyzer that finds it; Default follows the plugin's settings"`
}

// JellyfinPluginList wraps installed plugin builds.
type JellyfinPluginList struct {
	Plugins []arr.JellyfinPlugin `json:"plugins"`
	Count   int                  `json:"count"`
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

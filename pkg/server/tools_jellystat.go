package server

import "github.com/GauranshMathur/ARR_MCP/pkg/arr"

// --- jellystat tool inputs ---

// JellystatHistoryArgs selects and pages playback history.
type JellystatHistoryArgs struct {
	InstanceArg
	UserID    string `json:"userId,omitempty" jsonschema:"only this user's plays; a Jellyfin id from jellystat_user_activity. At most one of userId, itemId, libraryId"`
	ItemID    string `json:"itemId,omitempty" jsonschema:"only plays of this item, season or episode; a Jellyfin id"`
	LibraryID string `json:"libraryId,omitempty" jsonschema:"only plays from this library; an id from jellystat_list_libraries"`
	Limit     int    `json:"limit,omitempty" jsonschema:"rows per page; defaults to 25, at most 100"`
	Page      int    `json:"page,omitempty" jsonschema:"page number, from 1; defaults to 1"`
	Search    string `json:"search,omitempty" jsonschema:"text to look for in the title"`
	Sort      string `json:"sort,omitempty" jsonschema:"ActivityDateInserted (default), PlaybackDuration, UserName, NowPlayingItemName, Client, DeviceName, PlayMethod"`
	Desc      *bool  `json:"desc,omitempty" jsonschema:"newest or largest first; defaults to true"`
}

// JellystatRankingArgs selects a type and window for a ranking.
type JellystatRankingArgs struct {
	InstanceArg
	Type string `json:"type" jsonschema:"Movie, Series or Audio"`
	Days int    `json:"days,omitempty" jsonschema:"look back this many days; defaults to 30, at most 365"`
}

// JellystatDaysArgs sets a look-back window.
type JellystatDaysArgs struct {
	InstanceArg
	Days int `json:"days,omitempty" jsonschema:"look back this many days; defaults to 30, at most 365"`
}

// JellystatUserActivityArgs pages jellystat_user_activity.
type JellystatUserActivityArgs struct {
	InstanceArg
	Limit  int `json:"limit,omitempty" jsonschema:"users per page; defaults to 25, at most 100"`
	Offset int `json:"offset,omitempty" jsonschema:"users to skip, for paging; defaults to 0"`
}

// JellystatItemStatsArgs identifies the item and window for jellystat_item_stats.
type JellystatItemStatsArgs struct {
	InstanceArg
	ItemID string `json:"itemId" jsonschema:"a Jellyfin movie, series, season or episode id, e.g. from jellystat_library_items"`
	Hours  int    `json:"hours,omitempty" jsonschema:"window in hours, counted back from midnight today; defaults to 24. Use a large value such as 87600 for all time"`
}

// JellystatLibraryItemsArgs pages the items of one library.
type JellystatLibraryItemsArgs struct {
	InstanceArg
	LibraryID string `json:"libraryId" jsonschema:"library id from jellystat_list_libraries"`
	Limit     int    `json:"limit,omitempty" jsonschema:"items per page; defaults to 25, at most 100"`
	Page      int    `json:"page,omitempty" jsonschema:"page number, from 1; defaults to 1"`
	Search    string `json:"search,omitempty" jsonschema:"text to look for in the title"`
	Sort      string `json:"sort,omitempty" jsonschema:"Date (default, when added), Views, WatchTime, Size or Title. Views with desc false lists what nobody watched"`
	Desc      *bool  `json:"desc,omitempty" jsonschema:"largest or newest first; defaults to true"`
}

// --- jellystat tool outputs ---

// JellystatLibraryCountList wraps library counts.
type JellystatLibraryCountList struct {
	Libraries []arr.JellystatLibraryCount `json:"libraries"`
	Count     int                         `json:"count"`
}

// JellystatLibraryStatsList wraps library statistics.
type JellystatLibraryStatsList struct {
	Libraries []arr.JellystatLibraryStats `json:"libraries"`
	Count     int                         `json:"count"`
}

// JellystatTopItemList wraps a ranking of titles.
type JellystatTopItemList struct {
	Items []arr.JellystatTopItem `json:"items"`
	Count int                    `json:"count"`
}

// JellystatActiveUserList wraps the most active users.
type JellystatActiveUserList struct {
	Users []arr.JellystatActiveUser `json:"users"`
	Count int                       `json:"count"`
}

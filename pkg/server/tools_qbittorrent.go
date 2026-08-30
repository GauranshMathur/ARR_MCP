package server

import "github.com/GauranshMathur/ARR_MCP/pkg/arr"

// --- qbittorrent tool inputs ---

// TorrentFilterArgs is the input for qbittorrent_list_torrents.
type TorrentFilterArgs struct {
	InstanceArg
	Filter   string   `json:"filter,omitempty" jsonschema:"state filter: all, downloading, seeding, completed, stopped, active, inactive, stalled or errored"`
	Category *string  `json:"category,omitempty" jsonschema:"only torrents in this category; an empty string means uncategorised"`
	Tag      *string  `json:"tag,omitempty" jsonschema:"only torrents carrying this tag"`
	Hashes   []string `json:"hashes,omitempty" jsonschema:"only these torrent hashes"`
	Limit    int      `json:"limit,omitempty" jsonschema:"maximum torrents to return; defaults to 100"`
}

// TorrentHashArgs is the input for tools acting on a single torrent.
type TorrentHashArgs struct {
	InstanceArg
	Hash string `json:"hash" jsonschema:"torrent hash from qbittorrent_list_torrents"`
}

// TorrentHashesArgs is the input for bulk torrent actions.
type TorrentHashesArgs struct {
	InstanceArg
	Hashes []string `json:"hashes" jsonschema:"torrent hashes from qbittorrent_list_torrents, or [\"all\"] for every torrent"`
}

// AddTorrentArgs is the input for qbittorrent_add_torrent. Optional limits are
// pointers so an omitted argument leaves the instance defaults untouched.
type AddTorrentArgs struct {
	InstanceArg
	URLs             []string `json:"urls" jsonschema:"http, https or magnet links to add"`
	SavePath         string   `json:"savePath,omitempty" jsonschema:"download directory; omit for the default"`
	Category         string   `json:"category,omitempty" jsonschema:"category from qbittorrent_list_categories"`
	Tags             []string `json:"tags,omitempty" jsonschema:"tags to apply on add"`
	Stopped          bool     `json:"stopped,omitempty" jsonschema:"add without starting the download"`
	Rename           string   `json:"rename,omitempty" jsonschema:"display name for the torrent"`
	DownloadLimit    *int64   `json:"downloadLimit,omitempty" jsonschema:"bytes per second"`
	UploadLimit      *int64   `json:"uploadLimit,omitempty" jsonschema:"bytes per second"`
	RatioLimit       *float64 `json:"ratioLimit,omitempty" jsonschema:"stop seeding at this share ratio"`
	SeedingTimeLimit *int     `json:"seedingTimeLimit,omitempty" jsonschema:"stop seeding after this many minutes"`
	AutoTMM          *bool    `json:"autoTMM,omitempty" jsonschema:"let automatic torrent management pick the save path"`
}

// TorrentCategoryArgs is the input for qbittorrent_set_category.
type TorrentCategoryArgs struct {
	InstanceArg
	Hashes   []string `json:"hashes" jsonschema:"torrent hashes from qbittorrent_list_torrents, or [\"all\"] for every torrent"`
	Category string   `json:"category" jsonschema:"category name from qbittorrent_list_categories; an empty string clears the category"`
}

// CategoryArgs is the input for creating or editing a qBittorrent category.
type CategoryArgs struct {
	InstanceArg
	Name     string `json:"name" jsonschema:"category name"`
	SavePath string `json:"savePath,omitempty" jsonschema:"download directory for the category; omit for the default"`
}

// CategoryNamesArgs is the input for qbittorrent_delete_categories.
type CategoryNamesArgs struct {
	InstanceArg
	Names []string `json:"names" jsonschema:"category names from qbittorrent_list_categories"`
}

// TorrentTagsArgs is the input for the torrent tag tools.
type TorrentTagsArgs struct {
	InstanceArg
	Hashes []string `json:"hashes" jsonschema:"torrent hashes from qbittorrent_list_torrents, or [\"all\"] for every torrent"`
	Tags   []string `json:"tags,omitempty" jsonschema:"tag names; required when adding, and an empty list on remove strips every tag"`
}

// TorrentLocationArgs is the input for qbittorrent_set_location.
type TorrentLocationArgs struct {
	InstanceArg
	Hashes   []string `json:"hashes" jsonschema:"torrent hashes from qbittorrent_list_torrents, or [\"all\"] for every torrent"`
	Location string   `json:"location" jsonschema:"absolute directory to move the data to"`
}

// TorrentRenameArgs is the input for qbittorrent_rename_torrent.
type TorrentRenameArgs struct {
	InstanceArg
	Hash string `json:"hash" jsonschema:"torrent hash from qbittorrent_list_torrents"`
	Name string `json:"name" jsonschema:"new display name"`
}

// TorrentPriorityArgs is the input for qbittorrent_set_priority.
type TorrentPriorityArgs struct {
	InstanceArg
	Hashes   []string `json:"hashes" jsonschema:"torrent hashes from qbittorrent_list_torrents, or [\"all\"] for every torrent"`
	Position string   `json:"position" jsonschema:"where to move them in the queue: top, bottom, up or down"`
}

// TorrentLimitsArgs is the input for qbittorrent_set_torrent_limits. Omitted
// fields leave the current limits untouched.
type TorrentLimitsArgs struct {
	InstanceArg
	Hashes                   []string `json:"hashes" jsonschema:"torrent hashes from qbittorrent_list_torrents, or [\"all\"] for every torrent"`
	DownloadLimit            *int64   `json:"downloadLimit,omitempty" jsonschema:"bytes per second; 0 removes the limit"`
	UploadLimit              *int64   `json:"uploadLimit,omitempty" jsonschema:"bytes per second; 0 removes the limit"`
	RatioLimit               *float64 `json:"ratioLimit,omitempty" jsonschema:"stop seeding at this share ratio; -1 no limit, -2 the global default"`
	SeedingTimeLimit         *int     `json:"seedingTimeLimit,omitempty" jsonschema:"minutes; -1 no limit, -2 the global default"`
	InactiveSeedingTimeLimit *int     `json:"inactiveSeedingTimeLimit,omitempty" jsonschema:"minutes of inactivity; -1 no limit, -2 the global default"`
}

// GlobalLimitsArgs is the input for qbittorrent_set_global_limits.
type GlobalLimitsArgs struct {
	InstanceArg
	DownloadLimit   *int64 `json:"downloadLimit,omitempty" jsonschema:"global limit in bytes per second; 0 removes it"`
	UploadLimit     *int64 `json:"uploadLimit,omitempty" jsonschema:"global limit in bytes per second; 0 removes it"`
	AlternativeMode *bool  `json:"alternativeMode,omitempty" jsonschema:"switch the alternative speed limits on or off"`
}

// DeleteTorrentsArgs is the input for qbittorrent_delete_torrents.
type DeleteTorrentsArgs struct {
	InstanceArg
	Hashes      []string `json:"hashes" jsonschema:"torrent hashes from qbittorrent_list_torrents, or [\"all\"] for every torrent"`
	DeleteFiles bool     `json:"deleteFiles,omitempty" jsonschema:"also delete the downloaded files from disk"`
}

// --- qbittorrent tool outputs ---

// TorrentList wraps torrent results.
type TorrentList struct {
	Torrents []arr.Torrent `json:"torrents"`
	Count    int           `json:"count"`
}

// TorrentFileList wraps the files of one torrent.
type TorrentFileList struct {
	Files []arr.TorrentFile `json:"files"`
	Count int               `json:"count"`
}

// CategoryList wraps qBittorrent category results.
type CategoryList struct {
	Categories []arr.TorrentCategory `json:"categories"`
	Count      int                   `json:"count"`
}

// TagNameList wraps qBittorrent tag names, which are bare strings rather than
// the id-labelled tags the *arr services use.
type TagNameList struct {
	Tags  []string `json:"tags"`
	Count int      `json:"count"`
}

// TorrentActionResult reports which torrents a bulk action was applied to.
type TorrentActionResult struct {
	Action string   `json:"action"`
	Hashes []string `json:"hashes" jsonschema:"the selector that was sent; [\"all\"] means every torrent"`
	Count  int      `json:"count"`
}

package server

import "github.com/GauranshMathur/ARR_MCP/pkg/arr"

// --- interactive release search ---

// SonarrReleaseSearchArgs is the input for sonarr_list_releases. The search is
// scoped either to one episode or to one season of a series.
type SonarrReleaseSearchArgs struct {
	InstanceArg
	EpisodeID    *int `json:"episodeId,omitempty" jsonschema:"episode id from sonarr_list_episodes; searches that episode"`
	SeriesID     *int `json:"seriesId,omitempty" jsonschema:"series id from sonarr_list_series; use with seasonNumber to search a whole season"`
	SeasonNumber *int `json:"seasonNumber,omitempty" jsonschema:"season to search; requires seriesId"`
	Limit        int  `json:"limit,omitempty" jsonschema:"maximum releases to return; defaults to 30"`
}

// RadarrReleaseSearchArgs is the input for radarr_list_releases.
type RadarrReleaseSearchArgs struct {
	InstanceArg
	MovieID int `json:"movieId" jsonschema:"movie id from radarr_list_movies"`
	Limit   int `json:"limit,omitempty" jsonschema:"maximum releases to return; defaults to 30"`
}

// ReleaseCandidateList wraps interactive search results.
type ReleaseCandidateList struct {
	Releases []arr.ReleaseCandidate `json:"releases"`
	Count    int                    `json:"count"`
}

// GrabReleaseArgs is the input for the grab_release tools.
type GrabReleaseArgs struct {
	InstanceArg
	GUID      string `json:"guid" jsonschema:"guid from the list_releases tool"`
	IndexerID int    `json:"indexerId" jsonschema:"indexerId from the same list_releases result as the guid"`
}

// Grabbed reports that a release was handed to a download client.
type Grabbed struct {
	Grabbed bool   `json:"grabbed"`
	GUID    string `json:"guid,omitempty"`
	ID      int    `json:"id,omitempty"`
}

// --- manual import ---

// ManualImportPreviewArgs is the input for the manual_import_preview tools.
// Exactly one of Folder and DownloadID identifies what to inspect.
type ManualImportPreviewArgs struct {
	InstanceArg
	Folder              string `json:"folder,omitempty" jsonschema:"path on the service's filesystem to scan"`
	DownloadID          string `json:"downloadId,omitempty" jsonschema:"downloadId from the queue tool"`
	SeriesID            *int   `json:"seriesId,omitempty" jsonschema:"Sonarr only; series id to match the files against"`
	MovieID             *int   `json:"movieId,omitempty" jsonschema:"Radarr only; movie id to match the files against"`
	FilterExistingFiles *bool  `json:"filterExistingFiles,omitempty" jsonschema:"hide files already in the library; defaults to the service's own behaviour"`
}

// ManualImportCandidateList wraps manual import preview results.
type ManualImportCandidateList struct {
	Files []arr.ManualImportCandidate `json:"files"`
	Count int                         `json:"count"`
}

// ManualImportFileArg is one file to import. Quality and Languages are names as
// the preview reports them, e.g. "WEBDL-1080p" and "English".
type ManualImportFileArg struct {
	Path         string   `json:"path" jsonschema:"path from the manual_import_preview tool, unchanged"`
	SeriesID     *int     `json:"seriesId,omitempty" jsonschema:"Sonarr only; which series the file belongs to"`
	EpisodeIDs   []int    `json:"episodeIds,omitempty" jsonschema:"Sonarr only; which episodes the file contains"`
	MovieID      *int     `json:"movieId,omitempty" jsonschema:"Radarr only; which movie the file is"`
	Quality      string   `json:"quality,omitempty" jsonschema:"quality name, e.g. WEBDL-1080p; taken from the preview unless it was wrong"`
	Languages    []string `json:"languages,omitempty" jsonschema:"language names, e.g. English"`
	ReleaseGroup string   `json:"releaseGroup,omitempty"`
	DownloadID   string   `json:"downloadId,omitempty" jsonschema:"downloadId from the preview, so the import is credited to that download"`
}

// ManualImportArgs is the input for the manual_import tools.
type ManualImportArgs struct {
	InstanceArg
	Files      []ManualImportFileArg `json:"files" jsonschema:"the files to import, from manual_import_preview"`
	ImportMode string                `json:"importMode,omitempty" jsonschema:"auto, move or copy; defaults to auto"`
}

// --- library edits ---

// SonarrRenameFilesArgs is the input for sonarr_rename_files.
type SonarrRenameFilesArgs struct {
	InstanceArg
	SeriesID int   `json:"seriesId" jsonschema:"series id from sonarr_list_series"`
	FileIDs  []int `json:"fileIds" jsonschema:"episodeFileId values from sonarr_rename_preview"`
}

// RadarrRenameFilesArgs is the input for radarr_rename_files.
type RadarrRenameFilesArgs struct {
	InstanceArg
	MovieID int   `json:"movieId" jsonschema:"movie id from radarr_list_movies"`
	FileIDs []int `json:"fileIds" jsonschema:"movieFileId values from radarr_rename_preview"`
}

// UpdateFilesArgs is the input for the update_files tools. Every changeable
// field is optional so an omitted one keeps the value already recorded.
type UpdateFilesArgs struct {
	InstanceArg
	FileIDs      []int    `json:"fileIds" jsonschema:"file ids from the file listing tool"`
	Quality      *string  `json:"quality,omitempty" jsonschema:"quality name, e.g. Bluray-1080p"`
	Languages    []string `json:"languages,omitempty" jsonschema:"language names, e.g. English"`
	ReleaseGroup *string  `json:"releaseGroup,omitempty"`
}

// UpdateTagArgs is the input for the update_tag tools.
type UpdateTagArgs struct {
	InstanceArg
	ID    int    `json:"id" jsonschema:"tag id from the list_tags tool"`
	Label string `json:"label" jsonschema:"the new label for the tag"`
}

// DeleteQueueItemsArgs is the input for the delete_queue_items tools.
type DeleteQueueItemsArgs struct {
	InstanceArg
	IDs              []int `json:"ids" jsonschema:"queue item ids from the queue tool"`
	RemoveFromClient bool  `json:"removeFromClient,omitempty" jsonschema:"also remove the downloads from the download client"`
	Blocklist        bool  `json:"blocklist,omitempty" jsonschema:"blocklist the releases so they are not grabbed again"`
}

// UpdateCollectionArgs is the input for radarr_update_collection. The optional
// fields are pointers so an omitted argument leaves that setting untouched.
type UpdateCollectionArgs struct {
	InstanceArg
	ID                  int     `json:"id" jsonschema:"collection id from radarr_list_collections"`
	Monitored           *bool   `json:"monitored,omitempty" jsonschema:"monitor the collection so new members are added"`
	QualityProfileID    *int    `json:"qualityProfileId,omitempty" jsonschema:"id from radarr_list_quality_profiles"`
	RootFolderPath      *string `json:"rootFolderPath,omitempty" jsonschema:"path from radarr_list_root_folders"`
	SearchOnAdd         *bool   `json:"searchOnAdd,omitempty" jsonschema:"search for movies as they are added from this collection"`
	MinimumAvailability *string `json:"minimumAvailability,omitempty" jsonschema:"tba, announced, inCinemas or released"`
}

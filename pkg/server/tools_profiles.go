package server

// --- quality profiles ---

// CreateQualityProfileArgs is the input for the create_quality_profile tools.
// Qualities are named rather than numbered so a caller never has to guess an id.
type CreateQualityProfileArgs struct {
	InstanceArg
	Name           string   `json:"name" jsonschema:"a name for the new profile"`
	Allowed        []string `json:"allowed" jsonschema:"quality names to accept, e.g. WEBDL-1080p; group names from an existing profile also work"`
	Cutoff         string   `json:"cutoff,omitempty" jsonschema:"quality to stop upgrading at; defaults to the best of allowed"`
	UpgradeAllowed *bool    `json:"upgradeAllowed,omitempty" jsonschema:"replace a file on disk when a better release appears; defaults to the service's own default"`
}

// UpdateQualityProfileArgs is the input for the update_quality_profile tools.
// Every field is optional so an omitted one leaves that setting untouched.
type UpdateQualityProfileArgs struct {
	InstanceArg
	ID                int            `json:"id" jsonschema:"profile id from the list_quality_profiles tool"`
	Name              *string        `json:"name,omitempty" jsonschema:"rename the profile"`
	UpgradeAllowed    *bool          `json:"upgradeAllowed,omitempty" jsonschema:"replace a file on disk when a better release appears"`
	Cutoff            *string        `json:"cutoff,omitempty" jsonschema:"quality name to stop upgrading at, as the get_quality_profile tool reports it"`
	Allow             []string       `json:"allow,omitempty" jsonschema:"quality names to start accepting"`
	Disallow          []string       `json:"disallow,omitempty" jsonschema:"quality names to stop accepting"`
	MinFormatScore    *int           `json:"minFormatScore,omitempty" jsonschema:"custom format score a release must reach to be grabbed"`
	CutoffFormatScore *int           `json:"cutoffFormatScore,omitempty" jsonschema:"custom format score above which upgrading stops"`
	FormatScores      map[string]int `json:"formatScores,omitempty" jsonschema:"custom format name to score, using names from the list_custom_formats tool"`
}

// --- custom formats ---

// CustomFormatSpecArg is one matching rule of a custom format.
type CustomFormatSpecArg struct {
	Name           string         `json:"name" jsonschema:"a name for this rule"`
	Implementation string         `json:"implementation" jsonschema:"rule type, e.g. ReleaseTitleSpecification, ReleaseGroupSpecification, SourceSpecification or ResolutionSpecification"`
	Negate         bool           `json:"negate,omitempty" jsonschema:"invert the rule, so a match rejects instead of accepts"`
	Required       bool           `json:"required,omitempty" jsonschema:"the format only matches when this rule does"`
	Fields         map[string]any `json:"fields,omitempty" jsonschema:"the rule's settings by name; a title or release group rule takes value with a regular expression"`
}

// CreateCustomFormatArgs is the input for the create_custom_format tools.
type CreateCustomFormatArgs struct {
	InstanceArg
	Name                string                `json:"name" jsonschema:"a name for the new format"`
	IncludeWhenRenaming bool                  `json:"includeCustomFormatWhenRenaming,omitempty" jsonschema:"add the format's name to renamed files"`
	Specifications      []CustomFormatSpecArg `json:"specifications" jsonschema:"the rules that make this format match; use the shape the get_custom_format tool reports"`
}

// UpdateCustomFormatArgs is the input for the update_custom_format tools.
// Omitting a field leaves it as it was.
type UpdateCustomFormatArgs struct {
	InstanceArg
	ID                  int                   `json:"id" jsonschema:"format id from the list_custom_formats tool"`
	Name                *string               `json:"name,omitempty" jsonschema:"rename the format"`
	IncludeWhenRenaming *bool                 `json:"includeCustomFormatWhenRenaming,omitempty" jsonschema:"add the format's name to renamed files"`
	Specifications      []CustomFormatSpecArg `json:"specifications,omitempty" jsonschema:"replaces the whole rule set; omit to leave the rules alone"`
}

// --- root folders ---

// PathArgs is the input for the add_root_folder tools.
type PathArgs struct {
	InstanceArg
	Path string `json:"path" jsonschema:"a path on the service's own filesystem, as the existing entries from the list_root_folders tool are written"`
}

// --- configuration ---

// UpdateNamingConfigArgs is the input for the update_naming_config tools. Every
// field is optional so an omitted one leaves that format string untouched, and
// the fields the other service uses are refused rather than silently ignored.
type UpdateNamingConfigArgs struct {
	InstanceArg
	RenameFiles              *bool   `json:"renameFiles,omitempty" jsonschema:"rename files to match these formats"`
	ReplaceIllegalCharacters *bool   `json:"replaceIllegalCharacters,omitempty" jsonschema:"substitute characters the filesystem rejects"`
	StandardEpisodeFormat    *string `json:"standardEpisodeFormat,omitempty" jsonschema:"Sonarr only"`
	DailyEpisodeFormat       *string `json:"dailyEpisodeFormat,omitempty" jsonschema:"Sonarr only"`
	AnimeEpisodeFormat       *string `json:"animeEpisodeFormat,omitempty" jsonschema:"Sonarr only"`
	SeriesFolderFormat       *string `json:"seriesFolderFormat,omitempty" jsonschema:"Sonarr only"`
	SeasonFolderFormat       *string `json:"seasonFolderFormat,omitempty" jsonschema:"Sonarr only"`
	SpecialsFolderFormat     *string `json:"specialsFolderFormat,omitempty" jsonschema:"Sonarr only"`
	StandardMovieFormat      *string `json:"standardMovieFormat,omitempty" jsonschema:"Radarr only"`
	MovieFolderFormat        *string `json:"movieFolderFormat,omitempty" jsonschema:"Radarr only"`
}

// UpdateMediaManagementConfigArgs is the input for the
// update_media_management_config tools. Every field is optional so an omitted
// one leaves that setting untouched.
type UpdateMediaManagementConfigArgs struct {
	InstanceArg
	AutoUnmonitorPreviouslyDownloaded *bool   `json:"autoUnmonitorPreviouslyDownloaded,omitempty" jsonschema:"stop monitoring media whose files are deleted"`
	CreateEmptyMediaFolders           *bool   `json:"createEmptyMediaFolders,omitempty" jsonschema:"create a folder for media with no files yet"`
	RecycleBin                        *string `json:"recycleBin,omitempty" jsonschema:"move deleted files here instead of removing them; empty string deletes outright"`
	RecycleBinCleanupDays             *int    `json:"recycleBinCleanupDays,omitempty" jsonschema:"days to keep files in the recycle bin; 0 keeps them forever"`
	DownloadPropersAndRepacks         *string `json:"downloadPropersAndRepacks,omitempty" jsonschema:"preferAndUpgrade, doNotUpgrade or doNotPrefer"`
	DeleteEmptyFolders                *bool   `json:"deleteEmptyFolders,omitempty"`
	FileDate                          *string `json:"fileDate,omitempty" jsonschema:"none, or a release date the file timestamp is set to"`
	RescanAfterRefresh                *string `json:"rescanAfterRefresh,omitempty" jsonschema:"always, afterManual or never"`
	SkipFreeSpaceCheckWhenImporting   *bool   `json:"skipFreeSpaceCheckWhenImporting,omitempty"`
	MinimumFreeSpaceWhenImporting     *int    `json:"minimumFreeSpaceWhenImporting,omitempty" jsonschema:"megabytes that must stay free"`
	CopyUsingHardlinks                *bool   `json:"copyUsingHardlinks,omitempty" jsonschema:"hardlink instead of copying, so seeding torrents cost no extra space"`
	ImportExtraFiles                  *bool   `json:"importExtraFiles,omitempty" jsonschema:"import subtitles and other files alongside the media"`
	ExtraFileExtensions               *string `json:"extraFileExtensions,omitempty" jsonschema:"comma-separated, e.g. srt,nfo"`
	EnableMediaInfo                   *bool   `json:"enableMediaInfo,omitempty" jsonschema:"read codecs and resolution from the files themselves"`
}

// UpdateDelayProfileArgs is the input for the update_delay_profile tools.
type UpdateDelayProfileArgs struct {
	InstanceArg
	ID                     int     `json:"id" jsonschema:"profile id from the list_delay_profiles tool"`
	PreferredProtocol      *string `json:"preferredProtocol,omitempty" jsonschema:"usenet or torrent"`
	UsenetDelay            *int    `json:"usenetDelay,omitempty" jsonschema:"minutes to hold a usenet release before grabbing it"`
	TorrentDelay           *int    `json:"torrentDelay,omitempty" jsonschema:"minutes to hold a torrent before grabbing it"`
	EnableUsenet           *bool   `json:"enableUsenet,omitempty" jsonschema:"grab usenet releases at all"`
	EnableTorrent          *bool   `json:"enableTorrent,omitempty" jsonschema:"grab torrents at all"`
	BypassIfHighestQuality *bool   `json:"bypassIfHighestQuality,omitempty" jsonschema:"grab immediately when nothing better could arrive"`
}

// --- release profiles ---

// CreateReleaseProfileArgs is the input for sonarr_create_release_profile.
type CreateReleaseProfileArgs struct {
	InstanceArg
	Name      string   `json:"name,omitempty" jsonschema:"a name for the new profile"`
	Enabled   *bool    `json:"enabled,omitempty" jsonschema:"defaults to true"`
	Required  []string `json:"required,omitempty" jsonschema:"terms a release title must contain"`
	Ignored   []string `json:"ignored,omitempty" jsonschema:"terms that reject a release"`
	IndexerID *int     `json:"indexerId,omitempty" jsonschema:"limit to one indexer, from the list_indexers tool; 0 means every indexer"`
	Tags      []int    `json:"tags,omitempty" jsonschema:"tag ids from the list_tags tool; empty applies the profile to everything"`
}

// UpdateReleaseProfileArgs is the input for sonarr_update_release_profile.
// Every field is optional so an omitted one leaves that setting untouched.
type UpdateReleaseProfileArgs struct {
	InstanceArg
	ID        int      `json:"id" jsonschema:"profile id from the list_release_profiles tool"`
	Name      *string  `json:"name,omitempty" jsonschema:"rename the profile"`
	Enabled   *bool    `json:"enabled,omitempty"`
	Required  []string `json:"required,omitempty" jsonschema:"replaces the terms a release title must contain"`
	Ignored   []string `json:"ignored,omitempty" jsonschema:"replaces the terms that reject a release"`
	IndexerID *int     `json:"indexerId,omitempty" jsonschema:"limit to one indexer; 0 means every indexer"`
	Tags      []int    `json:"tags,omitempty" jsonschema:"replaces the tags the profile is limited to"`
}

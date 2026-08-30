package server

import (
	"context"

	"github.com/GauranshMathur/ARR_MCP/pkg/arr"
)

// profileOpts records what differs between the two media services, so one
// implementation can register the profile and configuration editors for both.
type profileOpts struct {
	// noun is what the service manages: "series" or "movies".
	noun string
	// releaseProfiles is true for the service whose release profiles this build
	// manages. Both services answer /releaseprofile, but only Sonarr's web UI
	// exposes them, and only Sonarr documents the term syntax.
	releaseProfiles bool
}

// registerProfiles adds the quality profile, custom format, root folder and
// configuration editors. Everything here behaves identically on Sonarr and
// Radarr apart from the wording, so registering from one place is what keeps
// the two services at parity.
func registerProfiles(s *Server, svc string, spec arr.ServiceSpec, opts profileOpts) {
	registerQualityProfiles(s, svc, spec, opts)
	registerCustomFormats(s, svc, spec)
	registerServiceConfig(s, svc, spec, opts)
	if opts.releaseProfiles {
		registerReleaseProfiles(s, svc, spec)
	}
}

// registerQualityProfiles adds the quality profile reader and editors.
func registerQualityProfiles(s *Server, svc string, spec arr.ServiceSpec, opts profileOpts) {
	register(s, svc, spec, toolMeta{
		name: svc + "_get_quality_profile",
		description: "Report one " + svc + " quality profile in full: which qualities it accepts, " +
			"which quality it stops upgrading at, and the score it gives each custom format. " +
			"Pass the profile id from " + svc + "_list_quality_profiles. " +
			"Qualities and the cutoff are reported by name, which is how the editing tools take them.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (arr.QualityProfileDetail, error) {
		return arr.GetQualityProfile(ctx, c, in.ID)
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_create_quality_profile",
		description: "Create a " + svc + " quality profile. Name the qualities to accept as " +
			svc + "_list_quality_definitions reports them, e.g. WEBDL-1080p. " +
			"The profile starts from the instance's own template, so every setting not named here " +
			"keeps the service's default. Omitting cutoff stops upgrading at the best allowed quality.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in CreateQualityProfileArgs) (arr.QualityProfileDetail, error) {
		return arr.CreateQualityProfile(ctx, c, arr.QualityProfileCreate{
			Name:           in.Name,
			Allowed:        in.Allowed,
			Cutoff:         in.Cutoff,
			UpgradeAllowed: in.UpgradeAllowed,
		})
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_update_quality_profile",
		description: "Change a " + svc + " quality profile: rename it, move the cutoff, accept or stop " +
			"accepting qualities by name, or rescore custom formats. Pass the profile id from " +
			svc + "_list_quality_profiles and quality names from " + svc + "_get_quality_profile; " +
			"format names come from " + svc + "_list_custom_formats. Omitted fields are left untouched.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in UpdateQualityProfileArgs) (arr.QualityProfileDetail, error) {
		return arr.UpdateQualityProfile(ctx, c, arr.QualityProfileUpdate{
			ID:                in.ID,
			Name:              in.Name,
			UpgradeAllowed:    in.UpgradeAllowed,
			Cutoff:            in.Cutoff,
			Allow:             in.Allow,
			Disallow:          in.Disallow,
			MinFormatScore:    in.MinFormatScore,
			CutoffFormatScore: in.CutoffFormatScore,
			FormatScores:      in.FormatScores,
		})
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_delete_quality_profile",
		description: "Delete a " + svc + " quality profile. A profile still assigned to " + opts.noun +
			" or to an import list is refused by the service, and the refusal says what is using it. " +
			"Pass the profile id from " + svc + "_list_quality_profiles.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Deleted, error) {
		if err := arr.DeleteQualityProfile(ctx, c, in.ID); err != nil {
			return Deleted{ID: in.ID}, err
		}
		return Deleted{ID: in.ID, Deleted: true}, nil
	})
}

// registerCustomFormats adds the custom format reader and editors.
func registerCustomFormats(s *Server, svc string, spec arr.ServiceSpec) {
	register(s, svc, spec, toolMeta{
		name: svc + "_get_custom_format",
		description: "Report one " + svc + " custom format with the rules that make it match, " +
			"including each rule's regular expression. Pass the format id from " + svc +
			"_list_custom_formats. The rule shape returned here is the shape the create and " +
			"update tools take.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (arr.CustomFormatDetail, error) {
		return arr.GetCustomFormat(ctx, c, in.ID)
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_create_custom_format",
		description: "Create a " + svc + " custom format from one or more matching rules. " +
			"Copy the rule shape from " + svc + "_get_custom_format: each rule needs a name, an " +
			"implementation such as ReleaseTitleSpecification, and its settings under fields. " +
			"A new format scores nothing until a quality profile gives it a score with " +
			svc + "_update_quality_profile.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in CreateCustomFormatArgs) (arr.CustomFormatDetail, error) {
		return arr.CreateCustomFormat(ctx, c, arr.CustomFormatCreate{
			Name:                in.Name,
			IncludeWhenRenaming: in.IncludeWhenRenaming,
			Specifications:      specifications(in.Specifications),
		})
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_update_custom_format",
		description: "Change a " + svc + " custom format. Pass the format id from " + svc +
			"_list_custom_formats. Giving specifications replaces the whole rule set, so read the " +
			"current rules with " + svc + "_get_custom_format first; omitting them leaves the rules alone.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in UpdateCustomFormatArgs) (arr.CustomFormatDetail, error) {
		return arr.UpdateCustomFormat(ctx, c, arr.CustomFormatUpdate{
			ID:                  in.ID,
			Name:                in.Name,
			IncludeWhenRenaming: in.IncludeWhenRenaming,
			Specifications:      specifications(in.Specifications),
		})
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_delete_custom_format",
		description: "Delete a " + svc + " custom format. Every quality profile scoring it loses that " +
			"score, which changes how releases rank. Pass the format id from " + svc + "_list_custom_formats.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Deleted, error) {
		if err := arr.DeleteCustomFormat(ctx, c, in.ID); err != nil {
			return Deleted{ID: in.ID}, err
		}
		return Deleted{ID: in.ID, Deleted: true}, nil
	})
}

// registerServiceConfig adds the root folder and configuration editors.
func registerServiceConfig(s *Server, svc string, spec arr.ServiceSpec, opts profileOpts) {
	register(s, svc, spec, toolMeta{
		name: svc + "_add_root_folder",
		description: "Add a library root folder to " + svc + ". The path is the service's own view of " +
			"the filesystem, which inside a container is the container's path, not the host's; " +
			"compare it with the entries from " + svc + "_list_root_folders. " +
			"The service reports the folder as inaccessible if it cannot write there.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in PathArgs) (arr.RootFolder, error) {
		return arr.AddRootFolder(ctx, c, in.Path)
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_delete_root_folder",
		description: "Remove a library root folder from " + svc + ". Nothing on disk is deleted and the " +
			opts.noun + " already imported stay in the library, but nothing new is added from the path. " +
			"Pass the folder id from " + svc + "_list_root_folders.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Deleted, error) {
		if err := arr.DeleteRootFolder(ctx, c, in.ID); err != nil {
			return Deleted{ID: in.ID}, err
		}
		return Deleted{ID: in.ID, Deleted: true}, nil
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_update_naming_config",
		description: "Change the " + svc + " file and folder naming formats. Read the current ones with " +
			svc + "_naming_config first, because a format string is replaced whole. " +
			"Omitted fields are left untouched, and a format this service does not have is refused " +
			"rather than ignored. Renaming existing files still needs " + svc + "_rename_files.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in UpdateNamingConfigArgs) (arr.NamingConfig, error) {
		return arr.UpdateNamingConfig(ctx, c, arr.NamingConfigUpdate{
			RenameFiles:              in.RenameFiles,
			ReplaceIllegalCharacters: in.ReplaceIllegalCharacters,
			StandardEpisodeFormat:    in.StandardEpisodeFormat,
			DailyEpisodeFormat:       in.DailyEpisodeFormat,
			AnimeEpisodeFormat:       in.AnimeEpisodeFormat,
			SeriesFolderFormat:       in.SeriesFolderFormat,
			SeasonFolderFormat:       in.SeasonFolderFormat,
			SpecialsFolderFormat:     in.SpecialsFolderFormat,
			StandardMovieFormat:      in.StandardMovieFormat,
			MovieFolderFormat:        in.MovieFolderFormat,
		})
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_media_management_config",
		description: "Report how " + svc + " handles files: the recycle bin, whether imports are " +
			"hard-linked or copied, what happens to empty folders, and how much space must stay free.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.MediaManagementConfig, error) {
		return arr.GetMediaManagementConfig(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_update_media_management_config",
		description: "Change how " + svc + " handles files. Read the current settings with " +
			svc + "_media_management_config first. Omitted fields are left untouched, and a setting " +
			"this service does not have is refused rather than ignored.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in UpdateMediaManagementConfigArgs) (arr.MediaManagementConfig, error) {
		return arr.UpdateMediaManagementConfig(ctx, c, arr.MediaManagementConfigUpdate{
			AutoUnmonitorPreviouslyDownloaded: in.AutoUnmonitorPreviouslyDownloaded,
			CreateEmptyMediaFolders:           in.CreateEmptyMediaFolders,
			RecycleBin:                        in.RecycleBin,
			RecycleBinCleanupDays:             in.RecycleBinCleanupDays,
			DownloadPropersAndRepacks:         in.DownloadPropersAndRepacks,
			DeleteEmptyFolders:                in.DeleteEmptyFolders,
			FileDate:                          in.FileDate,
			RescanAfterRefresh:                in.RescanAfterRefresh,
			SkipFreeSpaceCheckWhenImporting:   in.SkipFreeSpaceCheckWhenImporting,
			MinimumFreeSpaceWhenImporting:     in.MinimumFreeSpaceWhenImporting,
			CopyUsingHardlinks:                in.CopyUsingHardlinks,
			ImportExtraFiles:                  in.ImportExtraFiles,
			ExtraFileExtensions:               in.ExtraFileExtensions,
			EnableMediaInfo:                   in.EnableMediaInfo,
		})
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_update_delay_profile",
		description: "Change a " + svc + " delay profile: how long to hold a usenet or torrent release " +
			"before grabbing it, which protocol to prefer, and whether either is used at all. " +
			"Pass the profile id from " + svc + "_list_delay_profiles. Omitted fields are left untouched.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in UpdateDelayProfileArgs) (arr.DelayProfile, error) {
		return arr.UpdateDelayProfile(ctx, c, arr.DelayProfileUpdate{
			ID:                     in.ID,
			PreferredProtocol:      in.PreferredProtocol,
			UsenetDelay:            in.UsenetDelay,
			TorrentDelay:           in.TorrentDelay,
			EnableUsenet:           in.EnableUsenet,
			EnableTorrent:          in.EnableTorrent,
			BypassIfHighestQuality: in.BypassIfHighestQuality,
		})
	})
}

// registerReleaseProfiles adds the release profile editors.
func registerReleaseProfiles(s *Server, svc string, spec arr.ServiceSpec) {
	register(s, svc, spec, toolMeta{
		name: svc + "_create_release_profile",
		description: "Create a " + svc + " release profile, which requires or rejects releases by term. " +
			"Terms match the release title and may be plain words or /regular expressions/. " +
			"Leave indexerId out to apply the profile to every indexer, and tags out to apply it to " +
			"the whole library.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in CreateReleaseProfileArgs) (arr.ReleaseProfile, error) {
		return arr.CreateReleaseProfile(ctx, c, arr.ReleaseProfileCreate{
			Name:      in.Name,
			Enabled:   in.Enabled,
			Required:  in.Required,
			Ignored:   in.Ignored,
			IndexerID: in.IndexerID,
			Tags:      in.Tags,
		})
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_update_release_profile",
		description: "Change a " + svc + " release profile. Pass the profile id from " + svc +
			"_list_release_profiles. Giving required or ignored replaces that whole term list, " +
			"so read the current terms first; omitted fields are left untouched.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in UpdateReleaseProfileArgs) (arr.ReleaseProfile, error) {
		return arr.UpdateReleaseProfile(ctx, c, arr.ReleaseProfileUpdate{
			ID:        in.ID,
			Name:      in.Name,
			Enabled:   in.Enabled,
			Required:  in.Required,
			Ignored:   in.Ignored,
			IndexerID: in.IndexerID,
			Tags:      in.Tags,
		})
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_delete_release_profile",
		description: "Delete a " + svc + " release profile, so its terms stop filtering releases. " +
			"Pass the profile id from " + svc + "_list_release_profiles.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Deleted, error) {
		if err := arr.DeleteReleaseProfile(ctx, c, in.ID); err != nil {
			return Deleted{ID: in.ID}, err
		}
		return Deleted{ID: in.ID, Deleted: true}, nil
	})
}

// specifications converts the tool-level matching rules into client ones.
func specifications(in []CustomFormatSpecArg) []arr.CustomFormatSpecification {
	if len(in) == 0 {
		return nil
	}
	out := make([]arr.CustomFormatSpecification, 0, len(in))
	for _, spec := range in {
		out = append(out, arr.CustomFormatSpecification{
			Name:           spec.Name,
			Implementation: spec.Implementation,
			Negate:         spec.Negate,
			Required:       spec.Required,
			Fields:         spec.Fields,
		})
	}
	return out
}

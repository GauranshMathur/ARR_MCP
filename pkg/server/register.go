package server

import (
	"context"

	"github.com/GauranshMathur/ARR_MCP/pkg/arr"
)

// registerAll registers the tool set for every service this build supports.
// Services without configured instances register nothing, so the advertised
// tool list always reflects what is actually reachable.
func registerAll(s *Server) {
	registerSonarr(s)
	registerRadarr(s)
	registerProwlarr(s)
	registerBazarr(s)
	registerQBittorrent(s)
	registerNZBGet(s)

	registerOperations(s, "sonarr", arr.SonarrSpec, operationOpts{hasQueue: true})
	registerOperations(s, "radarr", arr.RadarrSpec, operationOpts{hasQueue: true})
	registerOperations(s, "prowlarr", arr.ProwlarrSpec, operationOpts{hasQueue: false})

	registerMedia(s, "sonarr", arr.SonarrSpec, mediaOpts{noun: "series"})
	registerMedia(s, "radarr", arr.RadarrSpec, mediaOpts{noun: "movies"})

	registerLibrary(s, "sonarr", arr.SonarrSpec, libraryOpts{noun: "series", episodes: true})
	registerLibrary(s, "radarr", arr.RadarrSpec, libraryOpts{noun: "movies", collections: true})

	registerProviders(s, "sonarr", arr.SonarrSpec)
	registerProviders(s, "radarr", arr.RadarrSpec)
}

func registerSonarr(s *Server) {
	const svc = "sonarr"
	spec := arr.SonarrSpec

	register(s, svc, spec, toolMeta{
		name:        "sonarr_list_series",
		description: "List the TV series in a Sonarr library.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (SeriesList, error) {
		series, err := arr.SonarrListSeries(ctx, c)
		return SeriesList{Series: series, Count: len(series)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_search_series",
		description: "Search for TV series to add to Sonarr. Returns tvdbId values for sonarr_add_series.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in SearchArgs) (SeriesList, error) {
		series, err := arr.SonarrLookupSeries(ctx, c, in.Query)
		return SeriesList{Series: series, Count: len(series)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_list_quality_profiles",
		description: "List Sonarr quality profiles. Needed before adding a series.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (ProfileList, error) {
		profiles, err := arr.ListQualityProfiles(ctx, c)
		return ProfileList{Profiles: profiles}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_list_root_folders",
		description: "List Sonarr root folders. Needed before adding a series.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (FolderList, error) {
		folders, err := arr.ListRootFolders(ctx, c)
		return FolderList{Folders: folders}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_system_status",
		description: "Report version and health information for a Sonarr instance.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.SystemStatus, error) {
		return arr.GetSystemStatus(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_add_series",
		description: "Add a TV series to a Sonarr library.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in AddSeriesArgs) (arr.Series, error) {
		seasonFolder := true
		if in.SeasonFolder != nil {
			seasonFolder = *in.SeasonFolder
		}
		req := arr.AddSeriesRequest{
			TVDBID:           in.TVDBID,
			Title:            in.Title,
			QualityProfileID: in.QualityProfileID,
			RootFolderPath:   in.RootFolderPath,
			Monitored:        true,
			SeasonFolder:     &seasonFolder,
			SeriesType:       in.SeriesType,
			Tags:             in.Tags,
		}
		req.AddOptions.SearchForMissingEpisodes = in.SearchNow
		req.AddOptions.Monitor = in.Monitor
		return arr.SonarrAddSeries(ctx, c, req)
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_delete_series",
		description: "Remove a series from Sonarr, optionally deleting its files from disk.",
		access:      AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in DeleteArgs) (Deleted, error) {
		if err := arr.SonarrDeleteSeries(ctx, c, in.ID, in.DeleteFiles); err != nil {
			return Deleted{ID: in.ID}, err
		}
		return Deleted{ID: in.ID, Deleted: true}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "sonarr_edit_series",
		description: "Change monitoring, quality profile, series type, tags or root folder for one or more series at once. " +
			"Omitted fields are left untouched.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in EditSeriesArgs) (SeriesList, error) {
		series, err := arr.SonarrEditSeries(ctx, c, arr.SeriesEditRequest{
			SeriesIDs:        in.SeriesIDs,
			Monitored:        in.Monitored,
			QualityProfileID: in.QualityProfileID,
			SeasonFolder:     in.SeasonFolder,
			RootFolderPath:   in.RootFolderPath,
			SeriesType:       in.SeriesType,
			MonitorNewItems:  in.MonitorNewItems,
			Tags:             in.Tags,
			ApplyTags:        in.ApplyTags,
			MoveFiles:        in.MoveFiles,
		})
		return SeriesList{Series: series, Count: len(series)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_set_season_monitored",
		description: "Monitor or unmonitor one season of a series. Season 0 is specials.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in SeasonMonitorArgs) (arr.Series, error) {
		return arr.SonarrSetSeasonMonitored(ctx, c, in.SeriesID, in.SeasonNumber, in.Monitored)
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_monitor_episodes",
		description: "Monitor or unmonitor specific episodes. Unmonitored episodes are never searched for.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in EpisodeMonitorArgs) (Updated, error) {
		if err := arr.SonarrMonitorEpisodes(ctx, c, in.EpisodeIDs, in.Monitored); err != nil {
			return Updated{}, err
		}
		return Updated{Updated: len(in.EpisodeIDs)}, nil
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_list_episode_files",
		description: "List the episode files on disk for one series, with their quality, size and release group.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in EpisodesArgs) (MediaFileList, error) {
		files, err := arr.SonarrListEpisodeFiles(ctx, c, in.SeriesID)
		return MediaFileList{Files: files, Count: len(files)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "sonarr_delete_episode_files",
		description: "Delete episode files from disk. This cannot be undone. " +
			"Pass file ids from sonarr_list_episode_files, not episode ids.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in FileIDsArgs) (DeletedCount, error) {
		deleted, err := arr.SonarrDeleteEpisodeFiles(ctx, c, in.FileIDs)
		return DeletedCount{Deleted: deleted}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_rename_preview",
		description: "Show which episode files do not match the naming config and what they would be renamed to. Renames nothing.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in SeriesRenameArgs) (RenamePreviewList, error) {
		renames, err := arr.SonarrRenamePreview(ctx, c, in.SeriesID, in.SeasonNumber)
		return RenamePreviewList{Renames: renames, Count: len(renames)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_wanted_missing",
		description: "List monitored episodes that have aired but have no file, with the library-wide total.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in LimitArgs) (EpisodeList, error) {
		eps, total, err := arr.SonarrWantedMissing(ctx, c, in.Limit)
		return EpisodeList{Episodes: eps, Count: len(eps), Total: total}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_wanted_cutoff",
		description: "List monitored episodes whose file is below the quality cutoff and would still be upgraded.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in LimitArgs) (EpisodeList, error) {
		eps, total, err := arr.SonarrWantedCutoff(ctx, c, in.Limit)
		return EpisodeList{Episodes: eps, Count: len(eps), Total: total}, err
	})

	register(s, svc, spec, toolMeta{
		name: "sonarr_trigger_search",
		description: "Start an indexer search for a series, one of its seasons, or specific episodes. " +
			"Give episodeIds to search episodes, seasonNumber to search a season, or neither to search the whole series.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in SearchScopeArgs) (arr.CommandResult, error) {
		return arr.SonarrTriggerSearch(ctx, c, in.SeriesID, in.SeasonNumber, in.EpisodeIDs)
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_refresh_series",
		description: "Rescan one series' metadata and files on disk.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in EpisodesArgs) (arr.CommandResult, error) {
		return arr.SonarrRefreshSeries(ctx, c, in.SeriesID)
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_calendar",
		description: "List episodes airing in a date range. Use for questions about what is coming up.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in CalendarArgs) (EpisodeList, error) {
		eps, err := arr.SonarrCalendar(ctx, c, in.Start, in.End)
		return EpisodeList{Episodes: eps, Count: len(eps)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "sonarr_list_episodes",
		description: "List every episode of one series, including which files are present.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in EpisodesArgs) (EpisodeList, error) {
		eps, err := arr.SonarrListEpisodes(ctx, c, in.SeriesID)
		return EpisodeList{Episodes: eps, Count: len(eps)}, err
	})
}

func registerRadarr(s *Server) {
	const svc = "radarr"
	spec := arr.RadarrSpec

	register(s, svc, spec, toolMeta{
		name:        "radarr_list_movies",
		description: "List the movies in a Radarr library.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (MovieList, error) {
		movies, err := arr.RadarrListMovies(ctx, c)
		return MovieList{Movies: movies, Count: len(movies)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_search_movies",
		description: "Search for movies to add to Radarr. Returns tmdbId values for radarr_add_movie.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in SearchArgs) (MovieList, error) {
		movies, err := arr.RadarrLookupMovies(ctx, c, in.Query)
		return MovieList{Movies: movies, Count: len(movies)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_list_quality_profiles",
		description: "List Radarr quality profiles. Needed before adding a movie.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (ProfileList, error) {
		profiles, err := arr.ListQualityProfiles(ctx, c)
		return ProfileList{Profiles: profiles}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_list_root_folders",
		description: "List Radarr root folders. Needed before adding a movie.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (FolderList, error) {
		folders, err := arr.ListRootFolders(ctx, c)
		return FolderList{Folders: folders}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_system_status",
		description: "Report version and health information for a Radarr instance.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.SystemStatus, error) {
		return arr.GetSystemStatus(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_add_movie",
		description: "Add a movie to a Radarr library.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in AddMovieArgs) (arr.Movie, error) {
		monitored := true
		if in.Monitored != nil {
			monitored = *in.Monitored
		}
		availability := in.MinimumAvailability
		if availability == "" {
			availability = "released"
		}
		req := arr.AddMovieRequest{
			TMDBID:              in.TMDBID,
			Title:               in.Title,
			QualityProfileID:    in.QualityProfileID,
			RootFolderPath:      in.RootFolderPath,
			Monitored:           monitored,
			MinimumAvailability: availability,
			Tags:                in.Tags,
		}
		req.AddOptions.SearchForMovie = in.SearchNow
		return arr.RadarrAddMovie(ctx, c, req)
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_delete_movie",
		description: "Remove a movie from Radarr, optionally deleting its files from disk.",
		access:      AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in DeleteArgs) (Deleted, error) {
		if err := arr.RadarrDeleteMovie(ctx, c, in.ID, in.DeleteFiles); err != nil {
			return Deleted{ID: in.ID}, err
		}
		return Deleted{ID: in.ID, Deleted: true}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "radarr_edit_movies",
		description: "Change monitoring, quality profile, minimum availability, tags or root folder for one or more movies at once. " +
			"Omitted fields are left untouched.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in EditMoviesArgs) (MovieList, error) {
		movies, err := arr.RadarrEditMovies(ctx, c, arr.MovieEditRequest{
			MovieIDs:            in.MovieIDs,
			Monitored:           in.Monitored,
			QualityProfileID:    in.QualityProfileID,
			MinimumAvailability: in.MinimumAvailability,
			RootFolderPath:      in.RootFolderPath,
			Tags:                in.Tags,
			ApplyTags:           in.ApplyTags,
			MoveFiles:           in.MoveFiles,
		})
		return MovieList{Movies: movies, Count: len(movies)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_list_movie_files",
		description: "List the files on disk for one movie, with their quality, size and release group.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in MovieArgs) (MediaFileList, error) {
		files, err := arr.RadarrListMovieFiles(ctx, c, in.MovieID)
		return MediaFileList{Files: files, Count: len(files)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "radarr_delete_movie_files",
		description: "Delete movie files from disk. This cannot be undone. " +
			"Pass file ids from radarr_list_movie_files, not movie ids.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in FileIDsArgs) (DeletedCount, error) {
		deleted, err := arr.RadarrDeleteMovieFiles(ctx, c, in.FileIDs)
		return DeletedCount{Deleted: deleted}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_rename_preview",
		description: "Show which files of a movie do not match the naming config and what they would be renamed to. Renames nothing.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in MovieArgs) (RenamePreviewList, error) {
		renames, err := arr.RadarrRenamePreview(ctx, c, in.MovieID)
		return RenamePreviewList{Renames: renames, Count: len(renames)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_wanted_missing",
		description: "List monitored movies that have been released but have no file, with the library-wide total.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in LimitArgs) (MovieList, error) {
		movies, total, err := arr.RadarrWantedMissing(ctx, c, in.Limit)
		return MovieList{Movies: movies, Count: len(movies), Total: total}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_wanted_cutoff",
		description: "List monitored movies whose file is below the quality cutoff and would still be upgraded.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in LimitArgs) (MovieList, error) {
		movies, total, err := arr.RadarrWantedCutoff(ctx, c, in.Limit)
		return MovieList{Movies: movies, Count: len(movies), Total: total}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_trigger_search",
		description: "Start an indexer search for specific movies.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in MovieIDsArgs) (arr.CommandResult, error) {
		return arr.RadarrTriggerSearch(ctx, c, in.MovieIDs)
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_refresh_movies",
		description: "Rescan metadata and files on disk for specific movies.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in MovieIDsArgs) (arr.CommandResult, error) {
		return arr.RadarrRefreshMovies(ctx, c, in.MovieIDs)
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_list_collections",
		description: "List the TMDB collections Radarr tracks, with how many movies each holds and how many are missing.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (CollectionList, error) {
		collections, err := arr.RadarrListCollections(ctx, c)
		return CollectionList{Collections: collections, Count: len(collections)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "radarr_calendar",
		description: "List movies releasing in a date range. Use for questions about upcoming releases.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in CalendarArgs) (MovieList, error) {
		movies, err := arr.RadarrCalendar(ctx, c, in.Start, in.End)
		return MovieList{Movies: movies, Count: len(movies)}, err
	})
}

// registerOperations adds the operational tools every *arr service shares.
// Sonarr, Radarr and Prowlarr expose the same /health, /history and /command
// endpoints, so these are written once and registered per service.
func registerOperations(s *Server, svc string, spec arr.ServiceSpec, opts operationOpts) {
	register(s, svc, spec, toolMeta{
		name:        svc + "_health",
		description: "Report health warnings and errors for a " + svc + " instance.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (HealthList, error) {
		issues, err := arr.ListHealthIssues(ctx, c)
		return HealthList{Issues: issues, Count: len(issues)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_history",
		description: "List recent grab, import and failure events from " + svc + ".",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in LimitArgs) (HistoryList, error) {
		records, err := arr.ListHistory(ctx, c, in.Limit)
		return HistoryList{Records: records, Count: len(records)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_run_command",
		description: "Trigger a background command in " + svc + ", such as RefreshSeries, RssSync or Backup.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in CommandArgs) (arr.CommandResult, error) {
		return arr.RunCommand(ctx, c, in.Name, nil)
	})

	if !opts.hasQueue {
		return
	}

	register(s, svc, spec, toolMeta{
		name:        svc + "_disk_space",
		description: "Report free and total disk space for " + svc + " library paths.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (DiskSpaceList, error) {
		disks, err := arr.ListDiskSpace(ctx, c)
		return DiskSpaceList{Disks: disks}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_queue",
		description: "List downloads currently queued or in progress in " + svc + ".",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in LimitArgs) (QueueList, error) {
		items, err := arr.ListQueue(ctx, c, in.Limit)
		return QueueList{Items: items, Count: len(items)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_delete_queue_item",
		description: "Remove a download from the " + svc + " queue, optionally blocklisting the release.",
		access:      AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in DeleteQueueArgs) (Deleted, error) {
		if err := arr.DeleteQueueItem(ctx, c, in.ID, in.RemoveFromClient, in.Blocklist); err != nil {
			return Deleted{ID: in.ID}, err
		}
		return Deleted{ID: in.ID, Deleted: true}, nil
	})
}

// operationOpts records which shared endpoints a service actually implements.
type operationOpts struct {
	// hasQueue is false for Prowlarr, which has no download queue or library.
	hasQueue bool
}

// mediaOpts records how a media service names the things it manages, so the
// tools registered for Sonarr and Radarr describe themselves accurately.
type mediaOpts struct {
	// noun is what the service manages: "series" or "movies".
	noun string
}

// registerMedia adds the tools Sonarr and Radarr share. Both expose the same
// settings, tag and library-maintenance endpoints under /api/v3, so registering
// them from one place is what keeps the two services at parity.
func registerMedia(s *Server, svc string, spec arr.ServiceSpec, opts mediaOpts) {
	registerTags(s, svc, spec, opts)
	registerSettings(s, svc, spec, opts)
	registerLibraryOps(s, svc, spec, opts)
}

// registerTags adds tag listing and management.
func registerTags(s *Server, svc string, spec arr.ServiceSpec, opts mediaOpts) {
	register(s, svc, spec, toolMeta{
		name:        svc + "_list_tags",
		description: "List the tags configured in " + svc + ". Tags organise " + opts.noun + " and scope profiles, indexers and notifications.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (TagList, error) {
		tags, err := arr.ListTags(ctx, c)
		return TagList{Tags: tags, Count: len(tags)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_tag_details",
		description: "List " + svc + " tags with a count of how many " + opts.noun + ", indexers and lists carry each one. Use to find unused tags.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (TagDetailList, error) {
		tags, err := arr.ListTagDetails(ctx, c)
		return TagDetailList{Tags: tags, Count: len(tags)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_create_tag",
		description: "Create a tag in " + svc + ". Returns the id needed to apply it to " + opts.noun + ".",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in LabelArgs) (arr.Tag, error) {
		return arr.CreateTag(ctx, c, in.Label)
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_delete_tag",
		description: "Delete a tag from " + svc + ". This detaches it from every " +
			opts.noun + ", profile and indexer that carried it.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Deleted, error) {
		if err := arr.DeleteTag(ctx, c, in.ID); err != nil {
			return Deleted{ID: in.ID}, err
		}
		return Deleted{ID: in.ID, Deleted: true}, nil
	})
}

// registerSettings adds the profile and configuration listings. These are the
// settings a power user asks about by name, and every one of them is identical
// between Sonarr and Radarr apart from the wording of the description.
func registerSettings(s *Server, svc string, spec arr.ServiceSpec, opts mediaOpts) {
	register(s, svc, spec, toolMeta{
		name:        svc + "_list_custom_formats",
		description: "List " + svc + " custom formats with the number of rules in each. Custom formats score releases during grabbing.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (CustomFormatList, error) {
		formats, err := arr.ListCustomFormats(ctx, c)
		return CustomFormatList{Formats: formats, Count: len(formats)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_list_delay_profiles",
		description: "List " + svc + " delay profiles, which decide how long to wait before grabbing a usenet or torrent release.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (DelayProfileList, error) {
		profiles, err := arr.ListDelayProfiles(ctx, c)
		return DelayProfileList{Profiles: profiles, Count: len(profiles)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_list_release_profiles",
		description: "List " + svc + " release profiles, which require or reject releases by term.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (ReleaseProfileList, error) {
		profiles, err := arr.ListReleaseProfiles(ctx, c)
		return ReleaseProfileList{Profiles: profiles, Count: len(profiles)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_list_indexers",
		description: "List the indexers " + svc + " searches. Connection settings and API keys are not returned.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (ProviderList, error) {
		providers, err := arr.ListIndexers(ctx, c)
		return ProviderList{Providers: providers, Count: len(providers)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_list_download_clients",
		description: "List the download clients " + svc + " sends grabs to. Credentials are not returned.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (ProviderList, error) {
		providers, err := arr.ListDownloadClients(ctx, c)
		return ProviderList{Providers: providers, Count: len(providers)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_list_import_lists",
		description: "List the import lists that add " + opts.noun + " to " + svc + " automatically.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (ProviderList, error) {
		providers, err := arr.ListImportLists(ctx, c)
		return ProviderList{Providers: providers, Count: len(providers)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_list_notifications",
		description: "List the notification connections configured in " + svc + ". Webhook URLs and tokens are not returned.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (ProviderList, error) {
		providers, err := arr.ListNotifications(ctx, c)
		return ProviderList{Providers: providers, Count: len(providers)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_naming_config",
		description: "Report the " + svc + " file and folder naming format strings.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.NamingConfig, error) {
		return arr.GetNamingConfig(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_list_quality_definitions",
		description: "List " + svc + " quality definitions with their size limits in megabytes per minute.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (QualityDefinitionList, error) {
		defs, err := arr.ListQualityDefinitions(ctx, c)
		return QualityDefinitionList{Definitions: defs, Count: len(defs)}, err
	})
}

// registerLibraryOps adds the blocklist and system views both media services
// share. Sonarr and Radarr expose them identically.
func registerLibraryOps(s *Server, svc string, spec arr.ServiceSpec, opts mediaOpts) {
	register(s, svc, spec, toolMeta{
		name:        svc + "_blocklist",
		description: "List releases " + svc + " has blocklisted and will not grab again, newest first.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in LimitArgs) (BlocklistList, error) {
		items, total, err := arr.ListBlocklist(ctx, c, in.Limit)
		return BlocklistList{Items: items, Count: len(items), Total: total}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_delete_blocklist_item",
		description: "Remove a release from the " + svc + " blocklist so it can be grabbed again.",
		access:      AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Deleted, error) {
		if err := arr.DeleteBlocklistItem(ctx, c, in.ID); err != nil {
			return Deleted{ID: in.ID}, err
		}
		return Deleted{ID: in.ID, Deleted: true}, nil
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_list_tasks",
		description: "List the scheduled background jobs in " + svc + " with when they last and next run.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (TaskList, error) {
		tasks, err := arr.ListTasks(ctx, c)
		return TaskList{Tasks: tasks, Count: len(tasks)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_list_updates",
		description: "List the " + svc + " releases available, and which one is installed.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (UpdateList, error) {
		updates, err := arr.ListUpdates(ctx, c)
		return UpdateList{Updates: updates, Count: len(updates)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        svc + "_queue_status",
		description: "Summarise the " + svc + " download queue: how many items and whether any have errors. Cheaper than listing the queue.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.QueueStatus, error) {
		return arr.GetQueueStatus(ctx, c)
	})
}

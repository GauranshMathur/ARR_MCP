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

	registerOperations(s, "sonarr", arr.SonarrSpec, operationOpts{hasQueue: true})
	registerOperations(s, "radarr", arr.RadarrSpec, operationOpts{hasQueue: true})
	registerOperations(s, "prowlarr", arr.ProwlarrSpec, operationOpts{hasQueue: false})
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
		req := arr.AddSeriesRequest{
			TVDBID:           in.TVDBID,
			Title:            in.Title,
			QualityProfileID: in.QualityProfileID,
			RootFolderPath:   in.RootFolderPath,
			Monitored:        true,
			SeasonFolder:     true,
		}
		req.AddOptions.SearchForMissingEpisodes = in.SearchNow
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
		req := arr.AddMovieRequest{
			TMDBID:              in.TMDBID,
			Title:               in.Title,
			QualityProfileID:    in.QualityProfileID,
			RootFolderPath:      in.RootFolderPath,
			Monitored:           true,
			MinimumAvailability: "released",
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
		name:        "radarr_calendar",
		description: "List movies releasing in a date range. Use for questions about upcoming releases.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in CalendarArgs) (MovieList, error) {
		movies, err := arr.RadarrCalendar(ctx, c, in.Start, in.End)
		return MovieList{Movies: movies, Count: len(movies)}, err
	})
}

func registerProwlarr(s *Server) {
	const svc = "prowlarr"
	spec := arr.ProwlarrSpec

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_list_indexers",
		description: "List the indexers configured in Prowlarr.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (IndexerList, error) {
		indexers, err := arr.ProwlarrListIndexers(ctx, c)
		return IndexerList{Indexers: indexers, Count: len(indexers)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_search",
		description: "Search all Prowlarr indexers for releases matching a query.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in ProwlarrSearchArgs) (ReleaseList, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 25
		}
		releases, err := arr.ProwlarrSearch(ctx, c, in.Query, in.Categories, limit)
		return ReleaseList{Releases: releases, Count: len(releases)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_system_status",
		description: "Report version and health information for a Prowlarr instance.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.SystemStatus, error) {
		return arr.GetSystemStatus(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_indexer_stats",
		description: "Report query, grab and failure counts per Prowlarr indexer. Use to find failing indexers.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (IndexerStatList, error) {
		stats, err := arr.ProwlarrIndexerStats(ctx, c)
		return IndexerStatList{Stats: stats}, err
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

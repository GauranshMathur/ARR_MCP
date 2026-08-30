package server

import (
	"context"

	"github.com/GauranshMathur/ARR_MCP/pkg/arr"
)

// libraryOpts records what differs between the two media services, so one
// implementation can register the interactive search, manual import and
// library-editing tools for both.
type libraryOpts struct {
	// noun is what the service manages: "series" or "movies".
	noun string
	// episodes is true for Sonarr, whose releases and imports are scoped to
	// episodes and seasons rather than to a single title.
	episodes bool
	// collections is true for Radarr, which groups movies into TMDB
	// collections that carry their own settings.
	collections bool
}

// registerLibrary adds the interactive search, manual import and library-edit
// tools. The tools that behave identically on both services are registered from
// here for either one; the episode-shaped and movie-shaped ones are gated on
// opts so neither service advertises arguments it cannot answer.
func registerLibrary(s *Server, svc string, spec arr.ServiceSpec, opts libraryOpts) {
	registerReleaseSearch(s, svc, spec, opts)
	registerManualImport(s, svc, spec, opts)
	registerLibraryEdits(s, svc, spec, opts)
}

// registerReleaseSearch adds the interactive search and the grab tools.
func registerReleaseSearch(s *Server, svc string, spec arr.ServiceSpec, opts libraryOpts) {
	if opts.episodes {
		register(s, svc, spec, toolMeta{
			name: svc + "_list_releases",
			description: "Search indexers for releases of one episode, or of one whole season. " +
				"Give episodeId from " + svc + "_list_episodes, or seriesId with seasonNumber. " +
				"This runs a real search against every indexer and can take a minute. " +
				"Returns guid and indexerId values for " + svc + "_grab_release, and the rejections " +
				"explaining why an automatic search would have skipped a release.",
			access: AccessRead,
		}, func(ctx context.Context, c *arr.Client, in SonarrReleaseSearchArgs) (ReleaseCandidateList, error) {
			releases, err := arr.SonarrListReleases(ctx, c, in.EpisodeID, in.SeriesID, in.SeasonNumber, in.Limit)
			return ReleaseCandidateList{Releases: releases, Count: len(releases)}, err
		})
	} else {
		register(s, svc, spec, toolMeta{
			name: svc + "_list_releases",
			description: "Search indexers for releases of one movie. " +
				"Give movieId from " + svc + "_list_movies. " +
				"This runs a real search against every indexer and can take a minute. " +
				"Returns guid and indexerId values for " + svc + "_grab_release, and the rejections " +
				"explaining why an automatic search would have skipped a release.",
			access: AccessRead,
		}, func(ctx context.Context, c *arr.Client, in RadarrReleaseSearchArgs) (ReleaseCandidateList, error) {
			releases, err := arr.RadarrListReleases(ctx, c, in.MovieID, in.Limit)
			return ReleaseCandidateList{Releases: releases, Count: len(releases)}, err
		})
	}

	register(s, svc, spec, toolMeta{
		name: svc + "_grab_release",
		description: "Send one release from " + svc + "_list_releases to a download client. " +
			"Pass the guid and indexerId from the same result. This overrides the rejections " +
			"that release listed, so a release the service would normally refuse is grabbed anyway.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in GrabReleaseArgs) (Grabbed, error) {
		if err := arr.GrabRelease(ctx, c, in.GUID, in.IndexerID); err != nil {
			return Grabbed{GUID: in.GUID}, err
		}
		return Grabbed{Grabbed: true, GUID: in.GUID}, nil
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_grab_queue_item",
		description: "Force a pending " + svc + " queue item to be grabbed now. " +
			"Use for a release a delay profile is still holding. Pass the queue item id from " + svc + "_queue.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Grabbed, error) {
		if err := arr.GrabQueueItem(ctx, c, in.ID); err != nil {
			return Grabbed{ID: in.ID}, err
		}
		return Grabbed{Grabbed: true, ID: in.ID}, nil
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_mark_history_failed",
		description: "Mark a past " + svc + " grab as failed. This blocklists the release and lets " +
			"the service search for a replacement. Pass the history record id from " + svc + "_history.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Updated, error) {
		if err := arr.MarkHistoryFailed(ctx, c, in.ID); err != nil {
			return Updated{}, err
		}
		return Updated{Updated: 1}, nil
	})
}

// registerManualImport adds the manual import preview and the import itself.
func registerManualImport(s *Server, svc string, spec arr.ServiceSpec, opts libraryOpts) {
	idHint := "movieId"
	if opts.episodes {
		idHint = "seriesId"
	}

	register(s, svc, spec, toolMeta{
		name: svc + "_manual_import_preview",
		description: "List the files " + svc + " could import from a folder or a finished download, " +
			"with the " + opts.noun + " it matched each one to. Give folder, or downloadId from " + svc + "_queue. " +
			"Imports nothing. Rejections explain why a file cannot be imported as matched; " +
			"correct it by passing " + idHint + " to " + svc + "_manual_import.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in ManualImportPreviewArgs) (ManualImportCandidateList, error) {
		files, err := arr.ListManualImportCandidates(ctx, c, arr.ManualImportQuery{
			Folder:              in.Folder,
			DownloadID:          in.DownloadID,
			SeriesID:            in.SeriesID,
			MovieID:             in.MovieID,
			FilterExistingFiles: in.FilterExistingFiles,
		})
		return ManualImportCandidateList{Files: files, Count: len(files)}, err
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_manual_import",
		description: "Import specific files into the " + svc + " library, using the paths from " +
			svc + "_manual_import_preview. Quality and languages are names as the preview reports them, " +
			"e.g. WEBDL-1080p and English. importMode auto lets the service decide, move moves the files, " +
			"copy leaves the originals in place.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in ManualImportArgs) (arr.CommandResult, error) {
		files := make([]arr.ManualImportFile, 0, len(in.Files))
		for _, f := range in.Files {
			file := arr.ManualImportFile{
				Path:         f.Path,
				EpisodeIDs:   f.EpisodeIDs,
				Quality:      f.Quality,
				Languages:    f.Languages,
				ReleaseGroup: f.ReleaseGroup,
				DownloadID:   f.DownloadID,
			}
			if f.SeriesID != nil {
				file.SeriesID = *f.SeriesID
			}
			if f.MovieID != nil {
				file.MovieID = *f.MovieID
			}
			files = append(files, file)
		}
		return arr.ManualImport(ctx, c, files, in.ImportMode)
	})
}

// registerLibraryEdits adds the renaming, file-metadata, tag, queue and
// collection editors.
func registerLibraryEdits(s *Server, svc string, spec arr.ServiceSpec, opts libraryOpts) {
	if opts.episodes {
		register(s, svc, spec, toolMeta{
			name: svc + "_get_series",
			description: "Report one series in detail: its path, quality profile, series type and tags, " +
				"plus a per-season breakdown of how many episodes exist and how many are on disk. " +
				"Use after " + svc + "_list_series to answer questions about one show.",
			access: AccessRead,
		}, func(ctx context.Context, c *arr.Client, in IDArgs) (arr.SeriesDetail, error) {
			return arr.SonarrGetSeries(ctx, c, in.ID)
		})

		register(s, svc, spec, toolMeta{
			name: svc + "_rename_files",
			description: "Rename episode files to match the naming config. " +
				"Pass seriesId and the episodeFileId values from " + svc + "_rename_preview.",
			access: AccessWrite,
		}, func(ctx context.Context, c *arr.Client, in SonarrRenameFilesArgs) (arr.CommandResult, error) {
			return arr.SonarrRenameFiles(ctx, c, in.SeriesID, in.FileIDs)
		})

		register(s, svc, spec, toolMeta{
			name: svc + "_update_files",
			description: "Correct the quality, languages or release group recorded for episode files. " +
				"Pass file ids from " + svc + "_list_episode_files, not episode ids. " +
				"Changes only the metadata; the files on disk are untouched. Omitted fields are left as they are.",
			access: AccessWrite,
		}, func(ctx context.Context, c *arr.Client, in UpdateFilesArgs) (MediaFileList, error) {
			files, err := arr.SonarrUpdateEpisodeFiles(ctx, c, in.FileIDs, in.Quality, in.Languages, in.ReleaseGroup)
			return MediaFileList{Files: files, Count: len(files)}, err
		})
	} else {
		register(s, svc, spec, toolMeta{
			name: svc + "_get_movie",
			description: "Report one movie in detail: its path, quality profile, minimum availability, " +
				"tags and collection, plus a summary of the file on disk. " +
				"Use after " + svc + "_list_movies to answer questions about one film.",
			access: AccessRead,
		}, func(ctx context.Context, c *arr.Client, in IDArgs) (arr.MovieDetail, error) {
			return arr.RadarrGetMovie(ctx, c, in.ID)
		})

		register(s, svc, spec, toolMeta{
			name: svc + "_rename_files",
			description: "Rename movie files to match the naming config. " +
				"Pass movieId and the movieFileId values from " + svc + "_rename_preview.",
			access: AccessWrite,
		}, func(ctx context.Context, c *arr.Client, in RadarrRenameFilesArgs) (arr.CommandResult, error) {
			return arr.RadarrRenameFiles(ctx, c, in.MovieID, in.FileIDs)
		})

		register(s, svc, spec, toolMeta{
			name: svc + "_update_files",
			description: "Correct the quality, languages or release group recorded for movie files. " +
				"Pass file ids from " + svc + "_list_movie_files, not movie ids. " +
				"Changes only the metadata; the files on disk are untouched. Omitted fields are left as they are.",
			access: AccessWrite,
		}, func(ctx context.Context, c *arr.Client, in UpdateFilesArgs) (MediaFileList, error) {
			files, err := arr.RadarrUpdateMovieFiles(ctx, c, in.FileIDs, in.Quality, in.Languages, in.ReleaseGroup)
			return MediaFileList{Files: files, Count: len(files)}, err
		})
	}

	if opts.collections {
		register(s, svc, spec, toolMeta{
			name: svc + "_update_collection",
			description: "Change the settings a " + svc + " collection applies to the movies added from it: " +
				"monitoring, quality profile, root folder, minimum availability and whether to search on add. " +
				"Pass the collection id from " + svc + "_list_collections. Omitted fields are left untouched.",
			access: AccessWrite,
		}, func(ctx context.Context, c *arr.Client, in UpdateCollectionArgs) (arr.Collection, error) {
			return arr.RadarrUpdateCollection(ctx, c, arr.CollectionUpdate{
				ID:                  in.ID,
				Monitored:           in.Monitored,
				QualityProfileID:    in.QualityProfileID,
				RootFolderPath:      in.RootFolderPath,
				SearchOnAdd:         in.SearchOnAdd,
				MinimumAvailability: in.MinimumAvailability,
			})
		})
	}

	register(s, svc, spec, toolMeta{
		name: svc + "_update_tag",
		description: "Rename an existing " + svc + " tag. Everything carrying the tag keeps it. " +
			"Pass the tag id from " + svc + "_list_tags.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in UpdateTagArgs) (arr.Tag, error) {
		return arr.UpdateTag(ctx, c, in.ID, in.Label)
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_delete_queue_items",
		description: "Remove several downloads from the " + svc + " queue at once, optionally telling the " +
			"download client to drop them and blocklisting the releases. " +
			"Pass queue item ids from " + svc + "_queue.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in DeleteQueueItemsArgs) (DeletedCount, error) {
		deleted, err := arr.DeleteQueueItems(ctx, c, in.IDs, in.RemoveFromClient, in.Blocklist)
		return DeletedCount{Deleted: deleted}, err
	})
}

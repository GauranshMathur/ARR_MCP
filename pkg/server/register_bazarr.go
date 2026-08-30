package server

import (
	"context"
	"fmt"

	"github.com/GauranshMathur/ARR_MCP/pkg/arr"
)

// searchOutcomeUnknown is returned by the subtitle search tools. Bazarr's
// search is synchronous and answers 204 whether or not any provider had a
// match, so reporting a download would be a guess.
const searchOutcomeUnknown = "search completed; Bazarr does not report whether a subtitle was found, " +
	"so re-check the wanted list to confirm"

// registerBazarr adds the subtitle management tools.
func registerBazarr(s *Server) {
	const svc = "bazarr"
	spec := arr.BazarrSpec

	register(s, svc, spec, toolMeta{
		name: "bazarr_badges",
		description: "Summarise outstanding subtitle work: how many episodes and movies are " +
			"missing subtitles, plus provider and health counts. Cheapest first call for " +
			"\"is anything missing?\" because it avoids listing every item.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.BazarrBadgeCounts, error) {
		return arr.BazarrBadges(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name:        "bazarr_wanted_episodes",
		description: "List episodes missing subtitles, with the languages each is missing.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in PageArgs) (WantedEpisodeList, error) {
		eps, total, err := arr.BazarrWantedEpisodes(ctx, c, in.Start, in.Length)
		return WantedEpisodeList{Episodes: eps, Returned: len(eps), Total: total}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "bazarr_wanted_movies",
		description: "List movies missing subtitles, with the languages each is missing.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in PageArgs) (WantedMovieList, error) {
		movies, total, err := arr.BazarrWantedMovies(ctx, c, in.Start, in.Length)
		return WantedMovieList{Movies: movies, Returned: len(movies), Total: total}, err
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_list_series",
		description: "List a page of series Bazarr tracks, with per-series counts of episodes " +
			"missing subtitles. Returns the library total alongside the page.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in PageArgs) (BazarrSeriesList, error) {
		series, total, err := arr.BazarrListSeries(ctx, c, in.Start, in.Length)
		return BazarrSeriesList{Series: series, Returned: len(series), Total: total}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "bazarr_list_movies",
		description: "List a page of movies Bazarr tracks subtitles for, with the library total.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in PageArgs) (BazarrMovieList, error) {
		movies, total, err := arr.BazarrListMovies(ctx, c, in.Start, in.Length)
		return BazarrMovieList{Movies: movies, Returned: len(movies), Total: total}, err
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_list_episode_subtitles",
		description: "List the subtitles present and missing for each episode of a series. " +
			"This is the only source of the file paths the subtitle deletion tools require; " +
			"an empty path means the track is embedded and cannot be deleted separately.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in SeriesSubtitlesArgs) (EpisodeSubtitlesList, error) {
		eps, err := arr.BazarrListEpisodeSubtitles(ctx, c, in.SeriesID)
		return EpisodeSubtitlesList{Episodes: eps, Count: len(eps)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_list_providers",
		description: "List the enabled subtitle providers with their throttle status. " +
			"Status is Good when a provider is working; anything else names the failure.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (SubtitleProviderList, error) {
		providers, err := arr.BazarrProviders(ctx, c)
		return SubtitleProviderList{Providers: providers}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "bazarr_list_languages",
		description: "List subtitle languages. Returns only enabled languages unless all is true.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in LanguagesArgs) (LanguageList, error) {
		langs, err := arr.BazarrLanguages(ctx, c, !in.All)
		return LanguageList{Languages: langs}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "bazarr_health",
		description: "Report Bazarr health issues.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (BazarrHealthList, error) {
		issues, err := arr.BazarrHealth(ctx, c)
		return BazarrHealthList{Issues: issues, Count: len(issues)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "bazarr_system_status",
		description: "Report Bazarr version and environment information.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (StatusMap, error) {
		status, err := arr.BazarrStatus(ctx, c)
		return StatusMap{Status: status}, err
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_search_episode_subtitles",
		description: "Search providers for an episode subtitle and download the best match. " +
			"Take seriesId and episodeId from bazarr_wanted_episodes. Bazarr reports success " +
			"whether or not a provider had a match, so confirm with bazarr_wanted_episodes. " +
			"For a knowable outcome use bazarr_manual_search_episode and then " +
			"bazarr_download_episode_subtitle, which show the candidates before choosing one.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in EpisodeSubtitleArgs) (Requested, error) {
		err := arr.BazarrSearchEpisodeSubtitles(ctx, c, in.SeriesID, in.EpisodeID, in.Language, in.Forced, in.HI)
		if err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true, Detail: searchOutcomeUnknown}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_search_movie_subtitles",
		description: "Search providers for a movie subtitle and download the best match. " +
			"Take radarrId from bazarr_wanted_movies. Bazarr reports success whether or not a " +
			"provider had a match, so confirm with bazarr_wanted_movies. For a knowable outcome " +
			"use bazarr_manual_search_movie and then bazarr_download_movie_subtitle, which show " +
			"the candidates before choosing one.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in MovieSubtitleArgs) (Requested, error) {
		err := arr.BazarrSearchMovieSubtitles(ctx, c, in.RadarrID, in.Language, in.Forced, in.HI)
		if err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true, Detail: searchOutcomeUnknown}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_delete_episode_subtitle",
		description: "Delete a downloaded subtitle file for an episode. The path must come from " +
			"bazarr_list_episode_subtitles; embedded tracks have no path and cannot be deleted.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in DeleteEpisodeSubtitleArgs) (Deleted, error) {
		err := arr.BazarrDeleteEpisodeSubtitle(ctx, c, in.SeriesID, in.EpisodeID, in.Language, in.Path, in.Forced, in.HI)
		if err != nil {
			return Deleted{ID: in.EpisodeID}, err
		}
		return Deleted{ID: in.EpisodeID, Deleted: true}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_delete_movie_subtitle",
		description: "Delete a downloaded subtitle file for a movie. Requires the subtitle file path; " +
			"embedded tracks have no path and cannot be deleted.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in DeleteMovieSubtitleArgs) (Deleted, error) {
		err := arr.BazarrDeleteMovieSubtitle(ctx, c, in.RadarrID, in.Language, in.Path, in.Forced, in.HI)
		if err != nil {
			return Deleted{ID: in.RadarrID}, err
		}
		return Deleted{ID: in.RadarrID, Deleted: true}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_list_language_profiles",
		description: "List the languages profiles: which subtitle languages each profile wants. " +
			"A series or movie with no profile gets no subtitles at all, so this is where the " +
			"profileId for bazarr_set_series_profile and bazarr_set_movie_profile comes from.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (LanguageProfileList, error) {
		profiles, err := arr.BazarrLanguageProfiles(ctx, c)
		return LanguageProfileList{Profiles: profiles, Count: len(profiles)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_set_series_profile",
		description: "Assign a languages profile to one or more series, which is what makes " +
			"Bazarr fetch subtitles for them. Take seriesIds from bazarr_list_series and " +
			"profileId from bazarr_list_language_profiles; profileId 0 unassigns the profile.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in SetSeriesProfileArgs) (Updated, error) {
		if err := arr.BazarrSetSeriesProfile(ctx, c, in.SeriesIDs, in.ProfileID); err != nil {
			return Updated{}, err
		}
		return Updated{Updated: len(in.SeriesIDs)}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_set_movie_profile",
		description: "Assign a languages profile to one or more movies. Take radarrIds from " +
			"bazarr_list_movies and profileId from bazarr_list_language_profiles; " +
			"profileId 0 unassigns the profile.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in SetMovieProfileArgs) (Updated, error) {
		if err := arr.BazarrSetMovieProfile(ctx, c, in.RadarrIDs, in.ProfileID); err != nil {
			return Updated{}, err
		}
		return Updated{Updated: len(in.RadarrIDs)}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_series_action",
		description: "Run a maintenance action on one series: scan-disk re-indexes the subtitle " +
			"files on disk, search-missing searches for the subtitles it is missing, and " +
			"search-wanted searches only the ones on the wanted list.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in SeriesActionArgs) (Requested, error) {
		if err := arr.BazarrSeriesAction(ctx, c, in.SeriesID, in.Action); err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true, Detail: searchOutcomeUnknown}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_movie_action",
		description: "Run a maintenance action on one movie: scan-disk, search-missing or " +
			"search-wanted. Take radarrId from bazarr_list_movies.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in MovieActionArgs) (Requested, error) {
		if err := arr.BazarrMovieAction(ctx, c, in.RadarrID, in.Action); err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true, Detail: searchOutcomeUnknown}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_manual_search_episode",
		description: "List the subtitles providers currently offer for one episode, with a match " +
			"score, the release each was timed for and what it does and does not match. " +
			"Downloads nothing. Take episodeId from bazarr_wanted_episodes or " +
			"bazarr_list_episode_subtitles, then pass a result's provider and subtitle token to " +
			"bazarr_download_episode_subtitle. Queries every provider, so it can take minutes.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in ManualSearchEpisodeArgs) (SubtitleCandidateList, error) {
		found, err := arr.BazarrManualSearchEpisode(ctx, c, in.EpisodeID)
		return SubtitleCandidateList{Candidates: found, Count: len(found)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_manual_search_movie",
		description: "List the subtitles providers currently offer for one movie, with match " +
			"scores and release information. Downloads nothing. Take radarrId from " +
			"bazarr_wanted_movies, then pass a result's provider and subtitle token to " +
			"bazarr_download_movie_subtitle. Queries every provider, so it can take minutes.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in ManualSearchMovieArgs) (SubtitleCandidateList, error) {
		found, err := arr.BazarrManualSearchMovie(ctx, c, in.RadarrID)
		return SubtitleCandidateList{Candidates: found, Count: len(found)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_download_episode_subtitle",
		description: "Download one specific subtitle chosen from bazarr_manual_search_episode. " +
			"Pass that result's provider and subtitle token unchanged, along with its " +
			"hearingImpaired and forced values; the token is opaque and only its own provider " +
			"can resolve it. Unlike the automatic search, the chosen subtitle is the one you get.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in DownloadEpisodeSubtitleArgs) (Requested, error) {
		err := arr.BazarrDownloadEpisodeSubtitle(ctx, c, in.SeriesID, in.EpisodeID,
			in.Provider, in.Subtitle, in.HI, in.Forced, in.OriginalFormat)
		if err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true, Detail: "subtitle downloaded; confirm with bazarr_list_episode_subtitles"}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_download_movie_subtitle",
		description: "Download one specific subtitle chosen from bazarr_manual_search_movie. " +
			"Pass that result's provider and subtitle token unchanged, along with its " +
			"hearingImpaired and forced values.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in DownloadMovieSubtitleArgs) (Requested, error) {
		err := arr.BazarrDownloadMovieSubtitle(ctx, c, in.RadarrID,
			in.Provider, in.Subtitle, in.HI, in.Forced, in.OriginalFormat)
		if err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true, Detail: "subtitle downloaded; confirm with bazarr_wanted_movies"}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_episode_history",
		description: "List episode subtitle history, newest first: which provider supplied each " +
			"subtitle, its score and where it was written. This is where the provider, subsId " +
			"and subtitlesPath for bazarr_blacklist_subtitle come from.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in EpisodeHistoryArgs) (SubtitleHistoryList, error) {
		records, total, err := arr.BazarrEpisodeHistory(ctx, c, in.Start, in.Length, in.EpisodeID)
		return SubtitleHistoryList{Records: records, Count: len(records), Total: total}, err
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_movie_history",
		description: "List movie subtitle history, newest first, with the provider, score and " +
			"path of each subtitle. Source of the identifiers bazarr_blacklist_subtitle needs.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in MovieHistoryArgs) (SubtitleHistoryList, error) {
		records, total, err := arr.BazarrMovieHistory(ctx, c, in.Start, in.Length, in.RadarrID)
		return SubtitleHistoryList{Records: records, Count: len(records), Total: total}, err
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_list_blacklist",
		description: "List blacklisted subtitles, which Bazarr will never download again. " +
			"Set kind to episodes or movies. Each entry's provider and subsId are what " +
			"bazarr_delete_blacklist_item needs to lift the block.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in BlacklistPageArgs) (SubtitleBlacklistList, error) {
		items, err := arr.BazarrBlacklist(ctx, c, in.Kind, in.Start, in.Length)
		return SubtitleBlacklistList{Items: items, Count: len(items)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_blacklist_subtitle",
		description: "Reject a bad subtitle: blacklist it so it is never downloaded again, " +
			"delete the file from disk and start a replacement search. Take provider, subsId, " +
			"language and subtitlesPath from bazarr_episode_history or bazarr_movie_history.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in BlacklistSubtitleArgs) (Requested, error) {
		var err error
		switch in.Kind {
		case "episodes":
			err = arr.BazarrBlacklistEpisodeSubtitle(ctx, c, in.SeriesID, in.EpisodeID,
				in.Provider, in.SubsID, in.Language, in.SubtitlesPath)
		case "movies":
			err = arr.BazarrBlacklistMovieSubtitle(ctx, c, in.RadarrID,
				in.Provider, in.SubsID, in.Language, in.SubtitlesPath)
		default:
			err = fmt.Errorf("unknown kind %q; valid kinds: episodes, movies", in.Kind)
		}
		if err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true,
			Detail: "subtitle blacklisted and deleted; a replacement search was started"}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_delete_blacklist_item",
		description: "Remove a blacklist entry so the subtitle can be downloaded again. Take " +
			"provider and subsId from bazarr_list_blacklist, or set all to empty that whole " +
			"blacklist. Does not touch any subtitle file.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in DeleteBlacklistArgs) (Deleted, error) {
		if err := arr.BazarrDeleteBlacklistItem(ctx, c, in.Kind, in.Provider, in.SubsID, in.All); err != nil {
			return Deleted{}, err
		}
		return Deleted{Deleted: true}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_reset_providers",
		description: "Clear every provider's throttling state so providers disabled by an error " +
			"are tried again immediately. Use when bazarr_list_providers shows a status other " +
			"than Good and the cause has been fixed.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (Requested, error) {
		if err := arr.BazarrResetProviders(ctx, c); err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true, Detail: "provider throttling reset"}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_list_tasks",
		description: "List Bazarr's scheduled jobs, whether each is running and when it next " +
			"runs. Each job_id can be run immediately with bazarr_run_task.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (BazarrTaskList, error) {
		tasks, err := arr.BazarrTasks(ctx, c)
		return BazarrTaskList{Tasks: tasks, Count: len(tasks)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_run_task",
		description: "Run one scheduled job now, such as syncing with Sonarr or searching for " +
			"missing subtitles. Take taskId from bazarr_list_tasks; Bazarr ignores an unknown " +
			"id rather than reporting it.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in RunTaskArgs) (Requested, error) {
		if err := arr.BazarrRunTask(ctx, c, in.TaskID); err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true, Detail: "task started; check bazarr_list_tasks for progress"}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_modify_subtitle",
		description: "Edit an existing subtitle file in place: sync it to the audio, translate " +
			"it to another language, or apply a mod (remove_HI strips hearing-impaired text, " +
			"remove_tags strips formatting, OCR_fixes, common, fix_uppercase, reverse_rtl). " +
			"Take path from bazarr_list_episode_subtitles and a sync reference such as a:0 from " +
			"bazarr_subtitle_info. Rewrites the file, so a bad result means re-downloading.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in ModifySubtitleArgs) (Requested, error) {
		err := arr.BazarrModifySubtitle(ctx, c, arr.SubtitleMod{
			Action: in.Action, Language: in.Language, Path: in.Path,
			MediaType: in.MediaType, MediaID: in.MediaID,
			Forced: in.Forced, HI: in.HI, OriginalFormat: in.OriginalFormat,
			Reference: in.Reference, MaxOffsetSeconds: in.MaxOffsetSeconds,
			NoFixFramerate: in.NoFixFramerate, GSS: in.GSS,
		})
		if err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true, Detail: "subtitle rewritten in place"}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "bazarr_subtitle_info",
		description: "List the audio tracks, embedded subtitle tracks and other external " +
			"subtitles beside one subtitle file. The audio track references (a:0, a:1) are the " +
			"reference values a sync in bazarr_modify_subtitle can use. Take path from " +
			"bazarr_list_episode_subtitles.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in SubtitleInfoArgs) (arr.SubtitleTracks, error) {
		return arr.BazarrSubtitleTracks(ctx, c, in.Path, in.EpisodeID, in.RadarrID)
	})
}

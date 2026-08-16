package server

import (
	"context"

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
		description: "Search providers for an episode subtitle and download it if one is found. " +
			"Take seriesId and episodeId from bazarr_wanted_episodes. Bazarr reports success " +
			"whether or not a provider had a match, so confirm with bazarr_wanted_episodes.",
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
		description: "Search providers for a movie subtitle and download it if one is found. " +
			"Take radarrId from bazarr_wanted_movies. Bazarr reports success whether or not a " +
			"provider had a match, so confirm with bazarr_wanted_movies.",
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
}

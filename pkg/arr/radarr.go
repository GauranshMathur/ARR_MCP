package arr

import (
	"context"
	"encoding/json"
	"fmt"
)

// Movie is the trimmed view of a Radarr movie returned to MCP clients.
type Movie struct {
	ID        int    `json:"id" jsonschema:"Radarr's internal movie id, used by other tools"`
	Title     string `json:"title"`
	Year      int    `json:"year,omitempty"`
	Status    string `json:"status,omitempty"`
	Monitored bool   `json:"monitored"`
	HasFile   bool   `json:"hasFile" jsonschema:"whether the movie is downloaded"`
	TMDBID    int    `json:"tmdbId,omitempty" jsonschema:"TMDB id, required when adding the movie"`
}

// rawMovie mirrors the upstream Radarr payload we care about before trimming.
type rawMovie struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Year      int    `json:"year"`
	Status    string `json:"status"`
	Monitored bool   `json:"monitored"`
	HasFile   bool   `json:"hasFile"`
	TMDBID    int    `json:"tmdbId"`
}

func (r rawMovie) toMovie() Movie {
	return Movie(r)
}

func trimMovies(raw []rawMovie) []Movie {
	out := make([]Movie, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.toMovie())
	}
	return out
}

// RadarrListMovies returns every movie in the library.
func RadarrListMovies(ctx context.Context, c *Client) ([]Movie, error) {
	raw, err := GetJSON[[]rawMovie](ctx, c, "/movie")
	if err != nil {
		return nil, err
	}
	return trimMovies(raw), nil
}

// RadarrLookupMovies searches for movies matching term.
func RadarrLookupMovies(ctx context.Context, c *Client, term string) ([]Movie, error) {
	raw, err := GetJSON[[]rawMovie](ctx, c, "/movie/lookup", Query{"term": term})
	if err != nil {
		return nil, err
	}
	return trimMovies(raw), nil
}

// AddMovieRequest describes a movie to add to a Radarr library.
type AddMovieRequest struct {
	TMDBID              int    `json:"tmdbId"`
	Title               string `json:"title"`
	QualityProfileID    int    `json:"qualityProfileId"`
	RootFolderPath      string `json:"rootFolderPath"`
	Monitored           bool   `json:"monitored"`
	MinimumAvailability string `json:"minimumAvailability,omitempty"`
	Tags                []int  `json:"tags,omitempty"`
	AddOptions          struct {
		SearchForMovie bool `json:"searchForMovie"`
	} `json:"addOptions"`
}

// RadarrAddMovie adds a movie to the library and returns the created record.
func RadarrAddMovie(ctx context.Context, c *Client, req AddMovieRequest) (Movie, error) {
	body, err := c.Post(ctx, "/movie", req)
	if err != nil {
		return Movie{}, err
	}
	var raw rawMovie
	if err := unmarshal(body, &raw); err != nil {
		return Movie{}, err
	}
	return raw.toMovie(), nil
}

// RadarrDeleteMovie removes a movie, optionally deleting its files.
func RadarrDeleteMovie(ctx context.Context, c *Client, id int, deleteFiles bool) error {
	_, err := c.Delete(ctx, "/movie/"+itoa(id), Query{"deleteFiles": btoa(deleteFiles)})
	return err
}

// MovieEditRequest describes a bulk change to one or more movies. As with the
// Sonarr editor, optional fields must stay absent rather than be sent as zero
// values, or they would overwrite settings the caller never named.
type MovieEditRequest struct {
	MovieIDs            []int  `json:"movieIds"`
	Monitored           *bool  `json:"monitored,omitempty"`
	QualityProfileID    *int   `json:"qualityProfileId,omitempty"`
	MinimumAvailability string `json:"minimumAvailability,omitempty" jsonschema:"tba, announced, inCinemas or released"`
	RootFolderPath      string `json:"rootFolderPath,omitempty"`
	Tags                []int  `json:"tags,omitempty"`
	ApplyTags           string `json:"applyTags,omitempty" jsonschema:"add, remove or replace"`
	MoveFiles           bool   `json:"moveFiles"`
}

// RadarrEditMovies applies a change to a set of movies at once.
func RadarrEditMovies(ctx context.Context, c *Client, req MovieEditRequest) ([]Movie, error) {
	if len(req.Tags) > 0 && req.ApplyTags == "" {
		req.ApplyTags = "add"
	}
	body, err := c.Put(ctx, "/movie/editor", req)
	if err != nil {
		return nil, err
	}
	var raw []rawMovie
	if err := unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return trimMovies(raw), nil
}

// RadarrListMovieFiles returns the files on disk for one movie.
func RadarrListMovieFiles(ctx context.Context, c *Client, movieID int) ([]MediaFile, error) {
	return listMediaFiles(ctx, c, "/moviefile", Query{"movieId": itoa(movieID)})
}

// RadarrDeleteMovieFiles deletes movie files from disk and returns how many
// were removed. This cannot be undone.
func RadarrDeleteMovieFiles(ctx context.Context, c *Client, ids []int) (int, error) {
	return deleteFiles(ctx, c, "/moviefile", ids, "movie file")
}

// RadarrRenamePreview lists the files of one movie that do not match the naming
// config, and what they would be renamed to.
func RadarrRenamePreview(ctx context.Context, c *Client, movieID int) ([]RenamePreview, error) {
	return GetJSON[[]RenamePreview](ctx, c, "/rename", Query{"movieId": itoa(movieID)})
}

// Collection is the trimmed view of a TMDB movie collection. The upstream
// resource embeds every movie it contains along with artwork and overviews —
// 259 KB for the list on a real instance — so only the counts survive.
type Collection struct {
	ID                  int    `json:"id"`
	Title               string `json:"title"`
	TMDBID              int    `json:"tmdbId,omitempty"`
	Monitored           bool   `json:"monitored"`
	MovieCount          int    `json:"movieCount"`
	MissingMovies       int    `json:"missingMovies,omitempty"`
	QualityProfileID    int    `json:"qualityProfileId,omitempty"`
	RootFolderPath      string `json:"rootFolderPath,omitempty"`
	SearchOnAdd         bool   `json:"searchOnAdd,omitempty"`
	MinimumAvailability string `json:"minimumAvailability,omitempty"`
	Tags                []int  `json:"tags,omitempty"`
}

// rawCollection mirrors the upstream payload before trimming.
type rawCollection struct {
	ID                  int               `json:"id"`
	Title               string            `json:"title"`
	TMDBID              int               `json:"tmdbId"`
	Monitored           bool              `json:"monitored"`
	MissingMovies       int               `json:"missingMovies"`
	QualityProfileID    int               `json:"qualityProfileId"`
	RootFolderPath      string            `json:"rootFolderPath"`
	SearchOnAdd         bool              `json:"searchOnAdd"`
	MinimumAvailability string            `json:"minimumAvailability"`
	Tags                []int             `json:"tags"`
	Movies              []json.RawMessage `json:"movies"`
}

// RadarrWantedMissing returns monitored movies that have been released but have
// no file, plus the total number missing across the library.
func RadarrWantedMissing(ctx context.Context, c *Client, pageSize int) ([]Movie, int, error) {
	return radarrWanted(ctx, c, "/wanted/missing", pageSize)
}

// RadarrWantedCutoff returns monitored movies whose file is below the quality
// cutoff, plus the total number across the library.
func RadarrWantedCutoff(ctx context.Context, c *Client, pageSize int) ([]Movie, int, error) {
	return radarrWanted(ctx, c, "/wanted/cutoff", pageSize)
}

// radarrWanted fetches one of the paged wanted lists. Unlike Sonarr's, these
// records are whole movie resources — 3.4 KB each with the overview, alternate
// titles and artwork — so they go through the same projection as /movie.
func radarrWanted(ctx context.Context, c *Client, path string, pageSize int) ([]Movie, int, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	env, err := GetJSON[paged[rawMovie]](ctx, c, path, Query{"pageSize": itoa(pageSize)})
	if err != nil {
		return nil, 0, err
	}
	return trimMovies(env.Records), env.TotalRecords, nil
}

// RadarrTriggerSearch starts an indexer search for specific movies.
func RadarrTriggerSearch(ctx context.Context, c *Client, movieIDs []int) (CommandResult, error) {
	if len(movieIDs) == 0 {
		return CommandResult{}, fmt.Errorf("no movie ids given; pass ids from radarr_list_movies")
	}
	return RunCommand(ctx, c, "MoviesSearch", map[string]any{"movieIds": movieIDs})
}

// RadarrRefreshMovies rescans metadata and files for specific movies.
func RadarrRefreshMovies(ctx context.Context, c *Client, movieIDs []int) (CommandResult, error) {
	if len(movieIDs) == 0 {
		return CommandResult{}, fmt.Errorf("no movie ids given; pass ids from radarr_list_movies")
	}
	return RunCommand(ctx, c, "RefreshMovie", map[string]any{"movieIds": movieIDs})
}

// RadarrListCollections returns the TMDB collections Radarr tracks.
func RadarrListCollections(ctx context.Context, c *Client) ([]Collection, error) {
	raw, err := GetJSON[[]rawCollection](ctx, c, "/collection")
	if err != nil {
		return nil, err
	}
	out := make([]Collection, 0, len(raw))
	for _, r := range raw {
		out = append(out, Collection{
			ID: r.ID, Title: r.Title, TMDBID: r.TMDBID, Monitored: r.Monitored,
			MovieCount: len(r.Movies), MissingMovies: r.MissingMovies,
			QualityProfileID: r.QualityProfileID, RootFolderPath: r.RootFolderPath,
			SearchOnAdd: r.SearchOnAdd, MinimumAvailability: r.MinimumAvailability,
			Tags: r.Tags,
		})
	}
	return out, nil
}

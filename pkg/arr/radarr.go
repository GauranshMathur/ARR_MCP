package arr

import "context"

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

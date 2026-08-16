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

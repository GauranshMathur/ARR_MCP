package arr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// RadarrClient extends the base ARR client with Radarr-specific functionality
type RadarrClient struct {
	*Client
}

// NewRadarrClient creates a new Radarr client
func NewRadarrClient(baseURL, apiKey string) *RadarrClient {
	return &RadarrClient{
		Client: NewClient(baseURL, apiKey, "Radarr"),
	}
}

// GetMovies retrieves movies from Radarr
func (c *RadarrClient) GetMovies() ([]map[string]interface{}, error) {
	respBody, err := c.doRequest(http.MethodGet, "/api/v3/movie", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get movies from Radarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetMoviesWithContext retrieves movies from Radarr with context
func (c *RadarrClient) GetMoviesWithContext(ctx context.Context) ([]map[string]interface{}, error) {
	respBody, err := c.doRequestWithContext(ctx, http.MethodGet, "/api/v3/movie", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get movies from Radarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetMovieById retrieves a specific movie by ID from Radarr
func (c *RadarrClient) GetMovieById(movieId int) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/v3/movie/%d", movieId)
	respBody, err := c.doRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie %d from Radarr: %w", movieId, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetMovieByIdWithContext retrieves a specific movie by ID from Radarr with context
func (c *RadarrClient) GetMovieByIdWithContext(ctx context.Context, movieId int) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/v3/movie/%d", movieId)
	respBody, err := c.doRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie %d from Radarr: %w", movieId, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// SearchMovies searches for movies in Radarr
func (c *RadarrClient) SearchMovies(term string) ([]map[string]interface{}, error) {
	// Check if we should use GET or POST based on term length
	if len(term) < 100 {
		// For shorter terms, use the GET endpoint with URL encoding
		params := url.Values{}
		params.Add("term", term)
		
		endpoint := "/api/v3/movie/lookup?" + params.Encode()
		
		respBody, err := c.doRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to search movies in Radarr: %w", err)
		}

		var result []map[string]interface{}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("error parsing response: %w", err)
		}

		return result, nil
	}
	
	// For longer terms, use POST to avoid URL length limitations
	requestBody, err := json.Marshal(map[string]string{
		"term": term,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating request body: %w", err)
	}

	respBody, err := c.doRequest(http.MethodPost, "/api/v3/movie/lookup", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to search movies in Radarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// SearchMoviesWithContext searches for movies in Radarr with context
func (c *RadarrClient) SearchMoviesWithContext(ctx context.Context, term string) ([]map[string]interface{}, error) {
	// Check if we should use GET or POST based on term length
	if len(term) < 100 {
		// For shorter terms, use the GET endpoint with URL encoding
		params := url.Values{}
		params.Add("term", term)
		
		endpoint := "/api/v3/movie/lookup?" + params.Encode()
		
		respBody, err := c.doRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to search movies in Radarr: %w", err)
		}

		var result []map[string]interface{}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("error parsing response: %w", err)
		}

		return result, nil
	}
	
	// For longer terms, use POST to avoid URL length limitations
	requestBody, err := json.Marshal(map[string]string{
		"term": term,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating request body: %w", err)
	}

	respBody, err := c.doRequestWithContext(ctx, http.MethodPost, "/api/v3/movie/lookup", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to search movies in Radarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// AddMovie adds a new movie to Radarr
func (c *RadarrClient) AddMovie(movieData map[string]interface{}) (map[string]interface{}, error) {
	// Required fields for adding a movie: tmdbId, title, qualityProfileId, rootFolderPath
	requiredFields := []string{"tmdbId", "title", "qualityProfileId", "rootFolderPath"}
	for _, field := range requiredFields {
		if _, exists := movieData[field]; !exists {
			return nil, fmt.Errorf("missing required field for adding movie: %s", field)
		}
	}

	// Set default values if not provided
	if _, exists := movieData["monitored"]; !exists {
		movieData["monitored"] = true
	}
	if _, exists := movieData["minimumAvailability"]; !exists {
		movieData["minimumAvailability"] = "released"
	}
	if _, exists := movieData["addOptions"]; !exists {
		movieData["addOptions"] = map[string]interface{}{
			"searchForMovie": true,
		}
	}

	requestBody, err := json.Marshal(movieData)
	if err != nil {
		return nil, fmt.Errorf("error creating request body: %w", err)
	}

	respBody, err := c.doRequest(http.MethodPost, "/api/v3/movie", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to add movie to Radarr: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// AddMovieWithContext adds a new movie to Radarr with context
func (c *RadarrClient) AddMovieWithContext(ctx context.Context, movieData map[string]interface{}) (map[string]interface{}, error) {
	// Required fields for adding a movie: tmdbId, title, qualityProfileId, rootFolderPath
	requiredFields := []string{"tmdbId", "title", "qualityProfileId", "rootFolderPath"}
	for _, field := range requiredFields {
		if _, exists := movieData[field]; !exists {
			return nil, fmt.Errorf("missing required field for adding movie: %s", field)
		}
	}

	// Set default values if not provided
	if _, exists := movieData["monitored"]; !exists {
		movieData["monitored"] = true
	}
	if _, exists := movieData["minimumAvailability"]; !exists {
		movieData["minimumAvailability"] = "released"
	}
	if _, exists := movieData["addOptions"]; !exists {
		movieData["addOptions"] = map[string]interface{}{
			"searchForMovie": true,
		}
	}

	requestBody, err := json.Marshal(movieData)
	if err != nil {
		return nil, fmt.Errorf("error creating request body: %w", err)
	}

	respBody, err := c.doRequestWithContext(ctx, http.MethodPost, "/api/v3/movie", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to add movie to Radarr: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetRootFolders retrieves available root folders from Radarr
func (c *RadarrClient) GetRootFolders() ([]map[string]interface{}, error) {
	respBody, err := c.doRequest(http.MethodGet, "/api/v3/rootfolder", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get root folders from Radarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetQualityProfiles retrieves available quality profiles from Radarr
func (c *RadarrClient) GetQualityProfiles() ([]map[string]interface{}, error) {
	respBody, err := c.doRequest(http.MethodGet, "/api/v3/qualityprofile", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get quality profiles from Radarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}
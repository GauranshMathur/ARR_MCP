package arr

import (
	"fmt"

	"arr-mcp/pkg/api"
)

// SonarrSearchHandler handles Sonarr search requests
type SonarrSearchHandler struct {
	Client *SonarrClient
}

// HandleRequest implements the api.Handler interface for SonarrSearchHandler
func (h *SonarrSearchHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("sonarr client not configured")
	}

	// Extract query parameter
	query, ok := req.Input["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("missing or invalid 'query' parameter")
	}

	// Perform search
	results, err := h.Client.SearchSeries(query)
	if err != nil {
		return nil, fmt.Errorf("sonarr search failed: %w", err)
	}

	return map[string]interface{}{
		"results": results,
	}, nil
}

// SonarrListHandler handles Sonarr list requests
type SonarrListHandler struct {
	Client *SonarrClient
}

// HandleRequest implements the api.Handler interface for SonarrListHandler
func (h *SonarrListHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("sonarr client not configured")
	}

	// Fetch series
	series, err := h.Client.GetSeries()
	if err != nil {
		return nil, fmt.Errorf("failed to get series from Sonarr: %w", err)
	}

	return map[string]interface{}{
		"series": series,
	}, nil
}

// SonarrAddSeriesHandler handles adding a series to Sonarr
type SonarrAddSeriesHandler struct {
	Client *SonarrClient
}

// HandleRequest implements the api.Handler interface for SonarrAddSeriesHandler
func (h *SonarrAddSeriesHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("sonarr client not configured")
	}

	// Extract series data
	seriesData, ok := req.Input["seriesData"].(map[string]interface{})
	if !ok || len(seriesData) == 0 {
		return nil, fmt.Errorf("missing or invalid 'seriesData' parameter")
	}

	// Add series
	result, err := h.Client.AddSeries(seriesData)
	if err != nil {
		return nil, fmt.Errorf("failed to add series to Sonarr: %w", err)
	}

	return map[string]interface{}{
		"series": result,
	}, nil
}

// SonarrGetProfilesHandler handles retrieving quality profiles from Sonarr
type SonarrGetProfilesHandler struct {
	Client *SonarrClient
}

// HandleRequest implements the api.Handler interface for SonarrGetProfilesHandler
func (h *SonarrGetProfilesHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("sonarr client not configured")
	}

	// Fetch quality profiles
	profiles, err := h.Client.GetQualityProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get quality profiles from Sonarr: %w", err)
	}

	return map[string]interface{}{
		"profiles": profiles,
	}, nil
}

// SonarrGetRootFoldersHandler handles retrieving root folders from Sonarr
type SonarrGetRootFoldersHandler struct {
	Client *SonarrClient
}

// HandleRequest implements the api.Handler interface for SonarrGetRootFoldersHandler
func (h *SonarrGetRootFoldersHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("sonarr client not configured")
	}

	// Fetch root folders
	folders, err := h.Client.GetRootFolders()
	if err != nil {
		return nil, fmt.Errorf("failed to get root folders from Sonarr: %w", err)
	}

	return map[string]interface{}{
		"folders": folders,
	}, nil
}

// RadarrSearchHandler handles Radarr search requests
type RadarrSearchHandler struct {
	Client *RadarrClient
}

// HandleRequest implements the api.Handler interface for RadarrSearchHandler
func (h *RadarrSearchHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("radarr client not configured")
	}

	// Extract query parameter
	query, ok := req.Input["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("missing or invalid 'query' parameter")
	}

	// Perform search
	results, err := h.Client.SearchMovies(query)
	if err != nil {
		return nil, fmt.Errorf("radarr search failed: %w", err)
	}

	return map[string]interface{}{
		"results": results,
	}, nil
}

// RadarrListHandler handles Radarr list requests
type RadarrListHandler struct {
	Client *RadarrClient
}

// HandleRequest implements the api.Handler interface for RadarrListHandler
func (h *RadarrListHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("radarr client not configured")
	}

	// Fetch movies
	movies, err := h.Client.GetMovies()
	if err != nil {
		return nil, fmt.Errorf("failed to get movies from Radarr: %w", err)
	}

	return map[string]interface{}{
		"movies": movies,
	}, nil
}

// RadarrAddMovieHandler handles adding a movie to Radarr
type RadarrAddMovieHandler struct {
	Client *RadarrClient
}

// HandleRequest implements the api.Handler interface for RadarrAddMovieHandler
func (h *RadarrAddMovieHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("radarr client not configured")
	}

	// Extract movie data
	movieData, ok := req.Input["movieData"].(map[string]interface{})
	if !ok || len(movieData) == 0 {
		return nil, fmt.Errorf("missing or invalid 'movieData' parameter")
	}

	// Add movie
	result, err := h.Client.AddMovie(movieData)
	if err != nil {
		return nil, fmt.Errorf("failed to add movie to Radarr: %w", err)
	}

	return map[string]interface{}{
		"movie": result,
	}, nil
}

// RadarrGetProfilesHandler handles retrieving quality profiles from Radarr
type RadarrGetProfilesHandler struct {
	Client *RadarrClient
}

// HandleRequest implements the api.Handler interface for RadarrGetProfilesHandler
func (h *RadarrGetProfilesHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("radarr client not configured")
	}

	// Fetch quality profiles
	profiles, err := h.Client.GetQualityProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get quality profiles from Radarr: %w", err)
	}

	return map[string]interface{}{
		"profiles": profiles,
	}, nil
}

// RadarrGetRootFoldersHandler handles retrieving root folders from Radarr
type RadarrGetRootFoldersHandler struct {
	Client *RadarrClient
}

// HandleRequest implements the api.Handler interface for RadarrGetRootFoldersHandler
func (h *RadarrGetRootFoldersHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("radarr client not configured")
	}

	// Fetch root folders
	folders, err := h.Client.GetRootFolders()
	if err != nil {
		return nil, fmt.Errorf("failed to get root folders from Radarr: %w", err)
	}

	return map[string]interface{}{
		"folders": folders,
	}, nil
}

// ProwlarrSearchHandler handles Prowlarr search requests
type ProwlarrSearchHandler struct {
	Client *ProwlarrClient
}

// HandleRequest implements the api.Handler interface for ProwlarrSearchHandler
func (h *ProwlarrSearchHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("prowlarr client not configured")
	}

	// Extract query parameter
	query, ok := req.Input["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("missing or invalid 'query' parameter")
	}

	// Extract categories parameter (optional)
	var categories []int
	if categoriesParam, ok := req.Input["categories"]; ok {
		if categoriesSlice, ok := categoriesParam.([]interface{}); ok {
			for _, cat := range categoriesSlice {
				if catInt, ok := cat.(float64); ok {
					categories = append(categories, int(catInt))
				}
			}
		}
	}

	// Perform search
	results, err := h.Client.Search(query, categories)
	if err != nil {
		return nil, fmt.Errorf("prowlarr search failed: %w", err)
	}

	return map[string]interface{}{
		"results": results,
	}, nil
}

// ProwlarrIndexersHandler handles Prowlarr indexers requests
type ProwlarrIndexersHandler struct {
	Client *ProwlarrClient
}

// HandleRequest implements the api.Handler interface for ProwlarrIndexersHandler
func (h *ProwlarrIndexersHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("prowlarr client not configured")
	}

	// Fetch indexers
	indexers, err := h.Client.GetIndexers()
	if err != nil {
		return nil, fmt.Errorf("failed to get indexers from Prowlarr: %w", err)
	}

	return map[string]interface{}{
		"indexers": indexers,
	}, nil
}
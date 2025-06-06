package arr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// SonarrClient extends the base ARR client with Sonarr-specific functionality
type SonarrClient struct {
	*Client
}

// NewSonarrClient creates a new Sonarr client
func NewSonarrClient(baseURL, apiKey string) *SonarrClient {
	return &SonarrClient{
		Client: NewClient(baseURL, apiKey, "Sonarr"),
	}
}

// GetSeries retrieves TV series from Sonarr
func (c *SonarrClient) GetSeries() ([]map[string]interface{}, error) {
	respBody, err := c.doRequest(http.MethodGet, "/api/v3/series", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get series from Sonarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetSeriesWithContext retrieves TV series from Sonarr with context
func (c *SonarrClient) GetSeriesWithContext(ctx context.Context) ([]map[string]interface{}, error) {
	respBody, err := c.doRequestWithContext(ctx, http.MethodGet, "/api/v3/series", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get series from Sonarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetSeriesById retrieves a specific TV series by ID from Sonarr
func (c *SonarrClient) GetSeriesById(seriesId int) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/v3/series/%d", seriesId)
	respBody, err := c.doRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get series %d from Sonarr: %w", seriesId, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetSeriesByIdWithContext retrieves a specific TV series by ID from Sonarr with context
func (c *SonarrClient) GetSeriesByIdWithContext(ctx context.Context, seriesId int) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/api/v3/series/%d", seriesId)
	respBody, err := c.doRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get series %d from Sonarr: %w", seriesId, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// SearchSeries searches for series in Sonarr
func (c *SonarrClient) SearchSeries(term string) ([]map[string]interface{}, error) {
	// Check if we should use GET or POST based on term length
	if len(term) < 100 {
		// For shorter terms, use the GET endpoint with URL encoding
		params := url.Values{}
		params.Add("term", term)
		
		endpoint := "/api/v3/series/lookup?" + params.Encode()
		
		respBody, err := c.doRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to search series in Sonarr: %w", err)
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

	respBody, err := c.doRequest(http.MethodPost, "/api/v3/series/lookup", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to search series in Sonarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// SearchSeriesWithContext searches for series in Sonarr with context
func (c *SonarrClient) SearchSeriesWithContext(ctx context.Context, term string) ([]map[string]interface{}, error) {
	// Check if we should use GET or POST based on term length
	if len(term) < 100 {
		// For shorter terms, use the GET endpoint with URL encoding
		params := url.Values{}
		params.Add("term", term)
		
		endpoint := "/api/v3/series/lookup?" + params.Encode()
		
		respBody, err := c.doRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to search series in Sonarr: %w", err)
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

	respBody, err := c.doRequestWithContext(ctx, http.MethodPost, "/api/v3/series/lookup", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to search series in Sonarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// AddSeries adds a new series to Sonarr
func (c *SonarrClient) AddSeries(seriesData map[string]interface{}) (map[string]interface{}, error) {
	// Required fields for adding a series: tvdbId, title, qualityProfileId, rootFolderPath
	requiredFields := []string{"tvdbId", "title", "qualityProfileId", "rootFolderPath"}
	for _, field := range requiredFields {
		if _, exists := seriesData[field]; !exists {
			return nil, fmt.Errorf("missing required field for adding series: %s", field)
		}
	}

	// Set default values if not provided
	if _, exists := seriesData["monitored"]; !exists {
		seriesData["monitored"] = true
	}
	if _, exists := seriesData["seasonFolder"]; !exists {
		seriesData["seasonFolder"] = true
	}
	if _, exists := seriesData["addOptions"]; !exists {
		seriesData["addOptions"] = map[string]interface{}{
			"searchForMissingEpisodes": true,
		}
	}

	requestBody, err := json.Marshal(seriesData)
	if err != nil {
		return nil, fmt.Errorf("error creating request body: %w", err)
	}

	respBody, err := c.doRequest(http.MethodPost, "/api/v3/series", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to add series to Sonarr: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// AddSeriesWithContext adds a new series to Sonarr with context
func (c *SonarrClient) AddSeriesWithContext(ctx context.Context, seriesData map[string]interface{}) (map[string]interface{}, error) {
	// Required fields for adding a series: tvdbId, title, qualityProfileId, rootFolderPath
	requiredFields := []string{"tvdbId", "title", "qualityProfileId", "rootFolderPath"}
	for _, field := range requiredFields {
		if _, exists := seriesData[field]; !exists {
			return nil, fmt.Errorf("missing required field for adding series: %s", field)
		}
	}

	// Set default values if not provided
	if _, exists := seriesData["monitored"]; !exists {
		seriesData["monitored"] = true
	}
	if _, exists := seriesData["seasonFolder"]; !exists {
		seriesData["seasonFolder"] = true
	}
	if _, exists := seriesData["addOptions"]; !exists {
		seriesData["addOptions"] = map[string]interface{}{
			"searchForMissingEpisodes": true,
		}
	}

	requestBody, err := json.Marshal(seriesData)
	if err != nil {
		return nil, fmt.Errorf("error creating request body: %w", err)
	}

	respBody, err := c.doRequestWithContext(ctx, http.MethodPost, "/api/v3/series", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to add series to Sonarr: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetRootFolders retrieves available root folders from Sonarr
func (c *SonarrClient) GetRootFolders() ([]map[string]interface{}, error) {
	respBody, err := c.doRequest(http.MethodGet, "/api/v3/rootfolder", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get root folders from Sonarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetQualityProfiles retrieves available quality profiles from Sonarr
func (c *SonarrClient) GetQualityProfiles() ([]map[string]interface{}, error) {
	respBody, err := c.doRequest(http.MethodGet, "/api/v3/qualityprofile", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get quality profiles from Sonarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}
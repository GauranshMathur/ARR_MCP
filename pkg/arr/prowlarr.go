package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ProwlarrClient extends the base ARR client with Prowlarr-specific functionality
type ProwlarrClient struct {
	*Client
}

// NewProwlarrClient creates a new Prowlarr client
func NewProwlarrClient(baseURL, apiKey string) *ProwlarrClient {
	return &ProwlarrClient{
		Client: NewClient(baseURL, apiKey, "Prowlarr"),
	}
}

// GetIndexers retrieves indexers from Prowlarr
func (c *ProwlarrClient) GetIndexers() ([]map[string]interface{}, error) {
	respBody, err := c.doRequest(http.MethodGet, "/api/v1/indexer", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexers from Prowlarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetIndexersWithContext retrieves indexers from Prowlarr with context
func (c *ProwlarrClient) GetIndexersWithContext(ctx context.Context) ([]map[string]interface{}, error) {
	respBody, err := c.doRequestWithContext(ctx, http.MethodGet, "/api/v1/indexer", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexers from Prowlarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetCategories retrieves categories from Prowlarr
func (c *ProwlarrClient) GetCategories() ([]map[string]interface{}, error) {
	respBody, err := c.doRequest(http.MethodGet, "/api/v1/indexer/category", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories from Prowlarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetCategoriesWithContext retrieves categories from Prowlarr with context
func (c *ProwlarrClient) GetCategoriesWithContext(ctx context.Context) ([]map[string]interface{}, error) {
	respBody, err := c.doRequestWithContext(ctx, http.MethodGet, "/api/v1/indexer/category", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories from Prowlarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// Search performs a search through Prowlarr's indexers
func (c *ProwlarrClient) Search(query string, categories []int) ([]map[string]interface{}, error) {
	// Build the query parameters with proper URL encoding
	params := url.Values{}
	params.Add("query", query)
	
	// Add categories if provided
	if len(categories) > 0 {
		categoryStrings := make([]string, len(categories))
		for i, cat := range categories {
			categoryStrings[i] = strconv.Itoa(cat)
		}
		params.Add("categories", strings.Join(categoryStrings, ","))
	}
	
	endpoint := "/api/v1/search?" + params.Encode()

	respBody, err := c.doRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search in Prowlarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// SearchWithContext performs a search through Prowlarr's indexers with context
func (c *ProwlarrClient) SearchWithContext(ctx context.Context, query string, categories []int) ([]map[string]interface{}, error) {
	// Build the query parameters with proper URL encoding
	params := url.Values{}
	params.Add("query", query)
	
	// Add categories if provided
	if len(categories) > 0 {
		categoryStrings := make([]string, len(categories))
		for i, cat := range categories {
			categoryStrings[i] = strconv.Itoa(cat)
		}
		params.Add("categories", strings.Join(categoryStrings, ","))
	}
	
	endpoint := "/api/v1/search?" + params.Encode()

	respBody, err := c.doRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search in Prowlarr: %w", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}
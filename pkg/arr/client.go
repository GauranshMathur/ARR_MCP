package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client represents a client for interacting with ARR stack applications
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	ServiceName string
}

// NewClient creates a new ARR client
func NewClient(baseURL, apiKey string, serviceName string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		ServiceName: serviceName,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the service name for health checking
func (c *Client) Name() string {
	return c.ServiceName
}

// Check performs a health check of the service
func (c *Client) Check() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Create a request with context for timeout
	reqURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	
	// Most ARR applications have a /api/v3/system/status endpoint
	reqURL.Path = "/api/v3/system/status"
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return fmt.Errorf("error creating health check request: %w", err)
	}
	
	req.Header.Set("X-Api-Key", c.APIKey)
	
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}
	
	return nil
}

// doRequest performs an HTTP request to the ARR API
func (c *Client) doRequest(method, path string, body io.Reader) ([]byte, error) {
	reqURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	
	// Ensure path is properly formatted
	if len(path) > 0 && path[0] != '/' {
		path = "/" + path
	}
	
	reqURL.Path = path

	req, err := http.NewRequest(method, reqURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errorBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned error status: %d, details: %s", resp.StatusCode, string(errorBody))
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return responseBody, nil
}

// doRequestWithContext performs an HTTP request with context
func (c *Client) doRequestWithContext(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	reqURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	
	// Ensure path is properly formatted
	if len(path) > 0 && path[0] != '/' {
		path = "/" + path
	}
	
	reqURL.Path = path

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errorBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned error status: %d, details: %s", resp.StatusCode, string(errorBody))
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return responseBody, nil
}

// GetStatus retrieves the status of the ARR application
func (c *Client) GetStatus() (map[string]interface{}, error) {
	respBody, err := c.doRequest(http.MethodGet, "/api/v3/system/status", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}

// GetStatusWithContext retrieves the status with context for timeout
func (c *Client) GetStatusWithContext(ctx context.Context) (map[string]interface{}, error) {
	respBody, err := c.doRequestWithContext(ctx, http.MethodGet, "/api/v3/system/status", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result, nil
}
package arr

import "context"

// Indexer is the trimmed view of a Prowlarr indexer.
type Indexer struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol,omitempty" jsonschema:"usenet or torrent"`
	Enable   bool   `json:"enable"`
	Priority int    `json:"priority,omitempty"`
}

// SearchResult is the trimmed view of a Prowlarr indexer search hit.
type SearchResult struct {
	Title       string `json:"title"`
	Indexer     string `json:"indexer,omitempty"`
	Size        int64  `json:"size,omitempty" jsonschema:"release size in bytes"`
	Seeders     int    `json:"seeders,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	PublishDate string `json:"publishDate,omitempty"`
}

// ProwlarrListIndexers returns the configured indexers.
func ProwlarrListIndexers(ctx context.Context, c *Client) ([]Indexer, error) {
	return GetJSON[[]Indexer](ctx, c, "/indexer")
}

// ProwlarrSearch searches all configured indexers for query.
func ProwlarrSearch(ctx context.Context, c *Client, query string, categories []int, limit int) ([]SearchResult, error) {
	q := Query{"query": query}
	if len(categories) > 0 {
		q["categories"] = joinInts(categories)
	}
	results, err := GetJSON[[]SearchResult](ctx, c, "/search", q)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

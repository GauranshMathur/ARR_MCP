package server

import "github.com/GauranshMathur/ARR_MCP/pkg/arr"

// --- prowlarr tool inputs ---

// ProwlarrSearchArgs is the input for prowlarr_search.
type ProwlarrSearchArgs struct {
	InstanceArg
	Query      string `json:"query" jsonschema:"search term"`
	Categories []int  `json:"categories,omitempty" jsonschema:"newznab category ids to filter by"`
	Limit      int    `json:"limit,omitempty" jsonschema:"maximum results to return; defaults to 25"`
}

// SchemaQueryArgs is the input for prowlarr_list_indexer_schemas.
type SchemaQueryArgs struct {
	InstanceArg
	Query string `json:"query,omitempty" jsonschema:"case-insensitive substring of the definition or site name; omit to list from the start"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum definitions to return; defaults to 50"`
}

// DefinitionArgs selects one indexer definition by name.
type DefinitionArgs struct {
	InstanceArg
	DefinitionName string `json:"definitionName" jsonschema:"definitionName from prowlarr_list_indexer_schemas"`
}

// AddIndexerArgs is the input for prowlarr_add_indexer. The optional settings
// are pointers so an omitted argument keeps the definition's own default rather
// than resetting it to a zero value.
type AddIndexerArgs struct {
	InstanceArg
	DefinitionName string         `json:"definitionName" jsonschema:"definitionName from prowlarr_list_indexer_schemas"`
	Name           string         `json:"name" jsonschema:"display name for the new indexer"`
	Enable         *bool          `json:"enable,omitempty"`
	Priority       *int           `json:"priority,omitempty" jsonschema:"1 is highest, 50 is lowest; 25 is the default"`
	AppProfileID   *int           `json:"appProfileId,omitempty" jsonschema:"sync profile id from prowlarr_list_app_profiles"`
	Tags           []int          `json:"tags,omitempty" jsonschema:"tag ids from prowlarr_list_tags"`
	Fields         map[string]any `json:"fields,omitempty" jsonschema:"indexer settings by field name from prowlarr_get_indexer_schema, e.g. baseUrl and apiKey"`
}

// UpdateIndexerArgs is the input for prowlarr_update_indexer. Every optional
// member is a pointer so an omitted argument leaves that setting alone.
type UpdateIndexerArgs struct {
	InstanceArg
	ID           int            `json:"id" jsonschema:"indexer id from prowlarr_list_indexers"`
	Name         *string        `json:"name,omitempty"`
	Enable       *bool          `json:"enable,omitempty"`
	Priority     *int           `json:"priority,omitempty" jsonschema:"1 is highest, 50 is lowest"`
	AppProfileID *int           `json:"appProfileId,omitempty" jsonschema:"sync profile id from prowlarr_list_app_profiles"`
	Tags         []int          `json:"tags,omitempty" jsonschema:"tag ids from prowlarr_list_tags; replaces the current tags"`
	Fields       map[string]any `json:"fields,omitempty" jsonschema:"indexer settings to change, by field name from prowlarr_get_indexer"`
}

// GrabArgs identifies one release to send to a download client.
type GrabArgs struct {
	InstanceArg
	GUID      string `json:"guid" jsonschema:"guid from prowlarr_search"`
	IndexerID int    `json:"indexerId" jsonschema:"indexerId from prowlarr_search, identifying which indexer the release came from"`
}

// --- prowlarr tool outputs ---

// IndexerList wraps indexer results.
type IndexerList struct {
	Indexers []arr.Indexer `json:"indexers"`
	Count    int           `json:"count"`
}

// IndexerSchemaList wraps indexer definition results. The listing is capped, so
// Count is the size of the page rather than the number of definitions Prowlarr
// ships; narrow the query to see fewer.
type IndexerSchemaList struct {
	Schemas []arr.IndexerSchema `json:"schemas"`
	Count   int                 `json:"count"`
}

// ReleaseList wraps indexer search results.
type ReleaseList struct {
	Releases []arr.SearchResult `json:"releases"`
	Count    int                `json:"count"`
}

// IndexerStatList wraps Prowlarr indexer statistics.
type IndexerStatList struct {
	Stats []arr.IndexerStat `json:"stats"`
}

// IndexerTestList wraps the outcome of testing every indexer.
type IndexerTestList struct {
	Results []arr.IndexerTestResult `json:"results"`
	Count   int                     `json:"count"`
	Failed  int                     `json:"failed" jsonschema:"how many indexers failed their test"`
}

// ApplicationList wraps the *arr instances Prowlarr syncs indexers to.
type ApplicationList struct {
	Applications []arr.Application `json:"applications"`
	Count        int               `json:"count"`
}

// AppProfileList wraps Prowlarr sync profile results.
type AppProfileList struct {
	Profiles []arr.AppProfile `json:"profiles"`
	Count    int              `json:"count"`
}

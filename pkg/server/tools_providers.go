package server

import "github.com/GauranshMathur/ARR_MCP/pkg/arr"

// --- provider tool inputs ---

// ProviderFlagArgs are the enable switches a provider may declare. Each is a
// pointer so an omitted argument leaves the stored setting alone, and each is
// only accepted by the kinds that have it: a flag the provider does not declare
// is an error rather than a silent no-op. A notification's onGrab, onDownload
// and other triggers are not settable here; they keep their current values.
type ProviderFlagArgs struct {
	Enable                  *bool `json:"enable,omitempty" jsonschema:"download clients only: whether the client is used"`
	EnableRSS               *bool `json:"enableRss,omitempty" jsonschema:"indexers only: use this indexer for RSS monitoring"`
	EnableAutomaticSearch   *bool `json:"enableAutomaticSearch,omitempty" jsonschema:"indexers only: use this indexer for automatic searches"`
	EnableInteractiveSearch *bool `json:"enableInteractiveSearch,omitempty" jsonschema:"indexers only: use this indexer for interactive searches"`
	EnableAutomaticAdd      *bool `json:"enableAutomaticAdd,omitempty" jsonschema:"import lists only: add what the list contains automatically"`
}

// flags converts the tool arguments into a client request.
func (a ProviderFlagArgs) flags() arr.ProviderFlags {
	return arr.ProviderFlags{
		Enable:                  a.Enable,
		EnableRSS:               a.EnableRSS,
		EnableAutomaticSearch:   a.EnableAutomaticSearch,
		EnableInteractiveSearch: a.EnableInteractiveSearch,
		EnableAutomaticAdd:      a.EnableAutomaticAdd,
	}
}

// ProviderSchemaArgs is the input for the provider schema listing.
type ProviderSchemaArgs struct {
	InstanceArg
	Kind  string `json:"kind" jsonschema:"provider family: indexer, downloadClient, notification or importList"`
	Query string `json:"query,omitempty" jsonschema:"case-insensitive substring of the implementation name, e.g. qbit or discord; omit to list from the start"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum implementations to return; defaults to 10"`
}

// ProviderIDArgs selects one configured provider.
type ProviderIDArgs struct {
	InstanceArg
	Kind string `json:"kind" jsonschema:"provider family: indexer, downloadClient, notification or importList"`
	ID   int    `json:"id" jsonschema:"provider id from the matching listing tool, e.g. sonarr_list_indexers"`
}

// AddProviderArgs is the input for the provider creation tool. The optional
// settings are pointers so an omitted argument keeps the implementation's own
// default rather than resetting it to a zero value.
type AddProviderArgs struct {
	InstanceArg
	ProviderFlagArgs
	Kind           string         `json:"kind" jsonschema:"provider family: indexer, downloadClient, notification or importList"`
	Implementation string         `json:"implementation" jsonschema:"implementation from the provider schemas tool, e.g. Nzbget or Discord"`
	Name           string         `json:"name" jsonschema:"display name for the new provider"`
	Priority       *int           `json:"priority,omitempty" jsonschema:"indexers and download clients only; 1 is highest"`
	Tags           []int          `json:"tags,omitempty" jsonschema:"tag ids from the service's tag listing"`
	Fields         map[string]any `json:"fields,omitempty" jsonschema:"connection settings by field name from the provider schemas tool, e.g. host, apiKey or webHookUrl"`
}

// UpdateProviderArgs is the input for the provider update tool. Every optional
// member is a pointer so an omitted argument leaves that setting alone.
type UpdateProviderArgs struct {
	InstanceArg
	ProviderFlagArgs
	Kind     string         `json:"kind" jsonschema:"provider family: indexer, downloadClient, notification or importList"`
	ID       int            `json:"id" jsonschema:"provider id from the matching listing tool"`
	Name     *string        `json:"name,omitempty"`
	Priority *int           `json:"priority,omitempty" jsonschema:"indexers and download clients only; 1 is highest"`
	Tags     []int          `json:"tags,omitempty" jsonschema:"tag ids; replaces the current tags"`
	Fields   map[string]any `json:"fields,omitempty" jsonschema:"connection settings to change, by field name; untouched fields keep their stored values, credentials included"`
}

// --- provider tool outputs ---

// ProviderSchemaList wraps the implementations one provider kind supports. The
// listing is capped, so Count is the size of the page rather than the number of
// implementations the service ships; narrow the query to see fewer.
type ProviderSchemaList struct {
	Schemas []arr.ProviderSchema `json:"schemas"`
	Count   int                  `json:"count"`
}

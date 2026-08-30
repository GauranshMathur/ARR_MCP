package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Indexer is the trimmed view of a Prowlarr indexer.
type Indexer struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	DefinitionName string `json:"definitionName,omitempty" jsonschema:"the definition this indexer was created from; pass it to prowlarr_add_indexer"`
	Implementation string `json:"implementation,omitempty" jsonschema:"Cardigann, Newznab, Torznab or a site-specific driver"`
	Protocol       string `json:"protocol,omitempty" jsonschema:"usenet or torrent"`
	Enable         bool   `json:"enable"`
	Priority       int    `json:"priority,omitempty"`
	AppProfileID   int    `json:"appProfileId,omitempty" jsonschema:"sync profile id from prowlarr_list_app_profiles"`
	Tags           []int  `json:"tags,omitempty"`
}

// IndexerField is one setting of an indexer or an indexer definition.
//
// Value is replaced with "***" whenever the upstream field declares a privacy
// other than "normal". Prowlarr marks credentials as apiKey, password or
// userName, and those are the indexer's own login details: nothing here should
// hand a model another service's credentials. The replacement is unconditional
// rather than value-dependent, so a new privacy value cannot leak by default.
type IndexerField struct {
	Name     string `json:"name"`
	Label    string `json:"label,omitempty"`
	Value    any    `json:"value,omitempty" jsonschema:"the configured value, or *** when the field holds a credential"`
	Type     string `json:"type,omitempty" jsonschema:"textbox, number, checkbox, select or info"`
	Privacy  string `json:"privacy,omitempty" jsonschema:"normal, apiKey, password or userName"`
	Advanced bool   `json:"advanced,omitempty"`
}

// IndexerDetail is one configured indexer with its settings.
type IndexerDetail struct {
	Indexer
	Privacy     string         `json:"privacy,omitempty" jsonschema:"public, semiPrivate or private"`
	Language    string         `json:"language,omitempty"`
	IndexerURLs []string       `json:"indexerUrls,omitempty"`
	Fields      []IndexerField `json:"fields,omitempty"`
}

// IndexerSchema is one indexer definition Prowlarr can create an indexer from.
// Fields are present only on the single-definition lookup: the raw list is 624
// definitions and 5.7 MB on a stock instance, almost all of it field metadata.
type IndexerSchema struct {
	Name           string         `json:"name"`
	DefinitionName string         `json:"definitionName,omitempty" jsonschema:"pass this to prowlarr_add_indexer"`
	Implementation string         `json:"implementation,omitempty"`
	Protocol       string         `json:"protocol,omitempty" jsonschema:"usenet or torrent"`
	Privacy        string         `json:"privacy,omitempty" jsonschema:"public, semiPrivate or private"`
	Language       string         `json:"language,omitempty"`
	Description    string         `json:"description,omitempty"`
	Fields         []IndexerField `json:"fields,omitempty"`
}

// IndexerTestResult reports whether an indexer answered a test request.
type IndexerTestResult struct {
	ID       int      `json:"id"`
	IsValid  bool     `json:"isValid"`
	Failures []string `json:"failures,omitempty" jsonschema:"why the test failed, one entry per validation error"`
}

// Application is an *arr instance Prowlarr syncs indexers to. Its own fields
// array holds that instance's API key and is deliberately not decoded.
type Application struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Implementation string `json:"implementation,omitempty" jsonschema:"Sonarr, Radarr, Lidarr and so on"`
	SyncLevel      string `json:"syncLevel,omitempty" jsonschema:"fullSync, addOnly or disabled"`
	Tags           []int  `json:"tags,omitempty"`
}

// AppProfile is a sync profile controlling how indexers behave in the apps
// Prowlarr pushes them to.
type AppProfile struct {
	ID                      int    `json:"id"`
	Name                    string `json:"name"`
	EnableRSS               bool   `json:"enableRss"`
	EnableAutomaticSearch   bool   `json:"enableAutomaticSearch"`
	EnableInteractiveSearch bool   `json:"enableInteractiveSearch"`
	MinimumSeeders          int    `json:"minimumSeeders,omitempty"`
}

// SearchResult is the trimmed view of a Prowlarr indexer search hit. GUID and
// IndexerID are what prowlarr_grab_release needs. downloadUrl is deliberately
// absent: most results carry one and it embeds the indexer's API key.
type SearchResult struct {
	Title       string `json:"title"`
	GUID        string `json:"guid,omitempty" jsonschema:"release identity; pass it with indexerId to prowlarr_grab_release"`
	IndexerID   int    `json:"indexerId,omitempty"`
	Indexer     string `json:"indexer,omitempty"`
	Size        int64  `json:"size,omitempty" jsonschema:"release size in bytes"`
	Seeders     int    `json:"seeders,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	PublishDate string `json:"publishDate,omitempty"`
}

// IndexerCreateRequest describes a new indexer. Optional settings are pointers
// so an omitted argument keeps the definition's own default instead of
// resetting it to a zero value.
type IndexerCreateRequest struct {
	// DefinitionName selects the schema entry to build from.
	DefinitionName string
	// Name is the display name for the new indexer.
	Name string
	// Enable, Priority and AppProfileID override the definition's defaults.
	Enable       *bool
	Priority     *int
	AppProfileID *int
	// Tags replaces the definition's tag list when non-nil.
	Tags []int
	// Fields sets indexer settings by field name, e.g. baseUrl or apiKey.
	Fields map[string]any
}

// IndexerUpdateRequest changes one existing indexer. Every optional member is a
// pointer so an omitted argument leaves the current setting alone.
type IndexerUpdateRequest struct {
	ID           int
	Name         *string
	Enable       *bool
	Priority     *int
	AppProfileID *int
	Tags         []int
	Fields       map[string]any
}

// rawIndexerField mirrors the upstream field entry before redaction.
type rawIndexerField struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Value    any    `json:"value"`
	Type     string `json:"type"`
	Privacy  string `json:"privacy"`
	Advanced bool   `json:"advanced"`
}

// rawIndexerDetail mirrors the upstream indexer resource. The capabilities
// block, which is most of the payload's 13 KB per indexer, has no member here
// and so cannot ride along.
type rawIndexerDetail struct {
	ID             int               `json:"id"`
	Name           string            `json:"name"`
	DefinitionName string            `json:"definitionName"`
	Implementation string            `json:"implementation"`
	Protocol       string            `json:"protocol"`
	Enable         bool              `json:"enable"`
	Priority       int               `json:"priority"`
	AppProfileID   int               `json:"appProfileId"`
	Tags           []int             `json:"tags"`
	Privacy        string            `json:"privacy"`
	Language       string            `json:"language"`
	IndexerURLs    []string          `json:"indexerUrls"`
	Fields         []rawIndexerField `json:"fields"`
}

// rawIndexerSchema mirrors one entry of /indexer/schema.
type rawIndexerSchema struct {
	Name           string            `json:"name"`
	DefinitionName string            `json:"definitionName"`
	Implementation string            `json:"implementation"`
	Protocol       string            `json:"protocol"`
	Privacy        string            `json:"privacy"`
	Language       string            `json:"language"`
	Description    string            `json:"description"`
	Fields         []rawIndexerField `json:"fields"`
}

// rawTestResult mirrors a provider test outcome.
type rawTestResult struct {
	ID                 int  `json:"id"`
	IsValid            bool `json:"isValid"`
	ValidationFailures []struct {
		PropertyName string `json:"propertyName"`
		ErrorMessage string `json:"errorMessage"`
	} `json:"validationFailures"`
}

// secretMask replaces the value of any field whose privacy is not "normal".
const secretMask = "***"

// trimFields projects upstream fields, masking every credential.
func trimFields(raw []rawIndexerField) []IndexerField {
	if len(raw) == 0 {
		return nil
	}
	out := make([]IndexerField, 0, len(raw))
	for _, f := range raw {
		value := f.Value
		if f.Privacy != "normal" {
			value = secretMask
		}
		out = append(out, IndexerField{
			Name: f.Name, Label: f.Label, Value: value,
			Type: f.Type, Privacy: f.Privacy, Advanced: f.Advanced,
		})
	}
	return out
}

// toIndexerDetail projects an upstream indexer, masking its credentials.
func (r rawIndexerDetail) toIndexerDetail() IndexerDetail {
	return IndexerDetail{
		Indexer: Indexer{
			ID: r.ID, Name: r.Name, DefinitionName: r.DefinitionName,
			Implementation: r.Implementation, Protocol: r.Protocol,
			Enable: r.Enable, Priority: r.Priority,
			AppProfileID: r.AppProfileID, Tags: r.Tags,
		},
		Privacy:     r.Privacy,
		Language:    r.Language,
		IndexerURLs: r.IndexerURLs,
		Fields:      trimFields(r.Fields),
	}
}

// toIndexerSchema projects a definition. withFields is false for listings,
// where the field metadata dwarfs everything a caller is choosing between.
func (r rawIndexerSchema) toIndexerSchema(withFields bool) IndexerSchema {
	out := IndexerSchema{
		Name: r.Name, DefinitionName: r.DefinitionName,
		Implementation: r.Implementation, Protocol: r.Protocol,
		Privacy: r.Privacy, Language: r.Language, Description: r.Description,
	}
	if withFields {
		out.Fields = trimFields(r.Fields)
	}
	return out
}

// ProwlarrListIndexers returns the configured indexers.
func ProwlarrListIndexers(ctx context.Context, c *Client) ([]Indexer, error) {
	return GetJSON[[]Indexer](ctx, c, "/indexer")
}

// ProwlarrGetIndexer returns one configured indexer with its settings, with
// every credential field masked.
func ProwlarrGetIndexer(ctx context.Context, c *Client, id int) (IndexerDetail, error) {
	raw, err := GetJSON[rawIndexerDetail](ctx, c, "/indexer/"+itoa(id))
	if err != nil {
		return IndexerDetail{}, err
	}
	return raw.toIndexerDetail(), nil
}

// defaultSchemaLimit caps a definition listing. Prowlarr answers /indexer/schema
// with every definition it ships -- 624 of them, 5.7 MB -- which no listing can
// return and no model needs to read.
const defaultSchemaLimit = 50

// ProwlarrListIndexerSchemas returns the indexer definitions whose name or
// definition name contains query, compared case-insensitively. An empty query
// matches everything. limit defaults to 50.
func ProwlarrListIndexerSchemas(ctx context.Context, c *Client, query string, limit int) ([]IndexerSchema, error) {
	raw, err := GetJSON[[]rawIndexerSchema](ctx, c, "/indexer/schema")
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultSchemaLimit
	}
	needle := strings.ToLower(query)
	out := make([]IndexerSchema, 0, limit)
	for _, r := range raw {
		if needle != "" &&
			!strings.Contains(strings.ToLower(r.Name), needle) &&
			!strings.Contains(strings.ToLower(r.DefinitionName), needle) {
			continue
		}
		out = append(out, r.toIndexerSchema(false))
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// ProwlarrGetIndexerSchema returns one indexer definition with its settable
// fields. Several definitions share a definition name -- every Newznab and
// Torznab preset does -- and the first match is returned as the template, so
// presets differing only in baseUrl must be set through that field.
func ProwlarrGetIndexerSchema(ctx context.Context, c *Client, definitionName string) (IndexerSchema, error) {
	raw, err := GetJSON[[]rawIndexerSchema](ctx, c, "/indexer/schema")
	if err != nil {
		return IndexerSchema{}, err
	}
	for _, r := range raw {
		if strings.EqualFold(r.DefinitionName, definitionName) {
			return r.toIndexerSchema(true), nil
		}
	}
	return IndexerSchema{}, fmt.Errorf(
		"unknown indexer definition %q; list definitions with prowlarr_list_indexer_schemas", definitionName)
}

// findSchemaResource returns the raw definition for definitionName. The entry
// is kept as a map so every key the definition declares survives into the
// created indexer; a typed round-trip would drop the ones this package has no
// member for and Prowlarr rejects the incomplete resource.
func findSchemaResource(ctx context.Context, c *Client, definitionName string) (map[string]any, error) {
	entries, err := GetJSON[[]map[string]any](ctx, c, "/indexer/schema")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name, _ := entry["definitionName"].(string)
		if strings.EqualFold(name, definitionName) {
			return entry, nil
		}
	}
	return nil, fmt.Errorf(
		"unknown indexer definition %q; list definitions with prowlarr_list_indexer_schemas", definitionName)
}

// patchFields sets values on a provider resource's fields array by field name.
// An unknown name is an error listing the valid ones, because a silently
// dropped setting leaves an indexer that cannot authenticate.
func patchFields(resource map[string]any, values map[string]any) error {
	if len(values) == 0 {
		return nil
	}
	entries, _ := resource["fields"].([]any)
	index := make(map[string]map[string]any, len(entries))
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		field, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		name, _ := field["name"].(string)
		if name == "" {
			continue
		}
		index[name] = field
		names = append(names, name)
	}
	sort.Strings(names)

	// Sorted so the rejection is the same whichever key Go's map iteration
	// reaches first.
	given := make([]string, 0, len(values))
	for name := range values {
		given = append(given, name)
	}
	sort.Strings(given)

	for _, name := range given {
		field, ok := index[name]
		if !ok {
			return fmt.Errorf("unknown indexer field %q; valid fields: %s", name, strings.Join(names, ", "))
		}
		field["value"] = values[name]
	}
	return nil
}

// ProwlarrAddIndexer creates an indexer from a definition. The definition's own
// resource is used as the request body so every key it declares is sent back
// intact, with only the caller's settings patched in.
func ProwlarrAddIndexer(ctx context.Context, c *Client, req IndexerCreateRequest) (IndexerDetail, error) {
	if req.DefinitionName == "" {
		return IndexerDetail{}, fmt.Errorf("definitionName is required")
	}
	resource, err := findSchemaResource(ctx, c, req.DefinitionName)
	if err != nil {
		return IndexerDetail{}, err
	}
	if req.Name != "" {
		resource["name"] = req.Name
	}
	if req.Enable != nil {
		resource["enable"] = *req.Enable
	}
	if req.Priority != nil {
		resource["priority"] = *req.Priority
	}
	if req.AppProfileID != nil {
		resource["appProfileId"] = *req.AppProfileID
	}
	if req.Tags != nil {
		resource["tags"] = req.Tags
	}
	if err := patchFields(resource, req.Fields); err != nil {
		return IndexerDetail{}, err
	}

	body, err := c.Post(ctx, "/indexer", resource)
	if err != nil {
		return IndexerDetail{}, err
	}
	var raw rawIndexerDetail
	if err := unmarshal(body, &raw); err != nil {
		return IndexerDetail{}, err
	}
	return raw.toIndexerDetail(), nil
}

// ProwlarrUpdateIndexer changes one indexer. The current resource is read as a
// map and written back as one, so settings this package has no member for --
// downloadClientId, configContract, the capabilities block -- survive the edit
// instead of being reset by a typed round-trip.
func ProwlarrUpdateIndexer(ctx context.Context, c *Client, req IndexerUpdateRequest) (IndexerDetail, error) {
	if req.ID <= 0 {
		return IndexerDetail{}, fmt.Errorf("indexer id is required")
	}
	path := "/indexer/" + itoa(req.ID)
	resource, err := GetJSON[map[string]any](ctx, c, path)
	if err != nil {
		return IndexerDetail{}, err
	}
	if req.Name != nil {
		resource["name"] = *req.Name
	}
	if req.Enable != nil {
		resource["enable"] = *req.Enable
	}
	if req.Priority != nil {
		resource["priority"] = *req.Priority
	}
	if req.AppProfileID != nil {
		resource["appProfileId"] = *req.AppProfileID
	}
	if req.Tags != nil {
		resource["tags"] = req.Tags
	}
	if err := patchFields(resource, req.Fields); err != nil {
		return IndexerDetail{}, err
	}

	body, err := c.Put(ctx, path, resource)
	if err != nil {
		return IndexerDetail{}, err
	}
	var raw rawIndexerDetail
	if err := unmarshal(body, &raw); err != nil {
		return IndexerDetail{}, err
	}
	return raw.toIndexerDetail(), nil
}

// ProwlarrDeleteIndexer removes an indexer and unsyncs it from every connected
// application. This cannot be undone.
func ProwlarrDeleteIndexer(ctx context.Context, c *Client, id int) error {
	_, err := c.Delete(ctx, "/indexer/"+itoa(id))
	return err
}

// rejectedMarker is how the shared client renders a 400. A rejected provider
// test is an answer rather than a transport failure, and the client reports
// status and body as one string, so the body is recovered from that text.
const rejectedMarker = " returned 400: "

// rejectedBody returns the response body of a 400, if err reports one.
func rejectedBody(err error) (string, bool) {
	message := err.Error()
	at := strings.Index(message, rejectedMarker)
	if at < 0 {
		return "", false
	}
	return strings.TrimSpace(message[at+len(rejectedMarker):]), true
}

// parseValidationFailures reads the upstream validation array, falling back to
// the raw body when it is not one.
func parseValidationFailures(body string) []string {
	var raw []struct {
		PropertyName string `json:"propertyName"`
		ErrorMessage string `json:"errorMessage"`
	}
	if err := json.Unmarshal([]byte(body), &raw); err != nil || len(raw) == 0 {
		return []string{body}
	}
	out := make([]string, 0, len(raw))
	for _, f := range raw {
		if f.PropertyName == "" {
			out = append(out, f.ErrorMessage)
			continue
		}
		out = append(out, f.PropertyName+": "+f.ErrorMessage)
	}
	return out
}

// ProwlarrTestIndexer asks Prowlarr to contact one indexer. A rejected test is
// reported as IsValid false with the reasons, not as an error: "this indexer is
// unreachable" is the answer the caller wanted.
func ProwlarrTestIndexer(ctx context.Context, c *Client, id int) (IndexerTestResult, error) {
	resource, err := GetJSON[map[string]any](ctx, c, "/indexer/"+itoa(id))
	if err != nil {
		return IndexerTestResult{}, err
	}
	if _, err := c.Post(ctx, "/indexer/test", resource); err != nil {
		body, rejected := rejectedBody(err)
		if !rejected {
			return IndexerTestResult{}, err
		}
		return IndexerTestResult{ID: id, Failures: parseValidationFailures(body)}, nil
	}
	return IndexerTestResult{ID: id, IsValid: true}, nil
}

// ProwlarrTestAllIndexers tests every configured indexer in one call.
func ProwlarrTestAllIndexers(ctx context.Context, c *Client) ([]IndexerTestResult, error) {
	body, err := c.Post(ctx, "/indexer/testall", nil)
	if err != nil {
		rejected, ok := rejectedBody(err)
		if !ok {
			return nil, err
		}
		body = []byte(rejected)
	}
	var raw []rawTestResult
	if err := unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]IndexerTestResult, 0, len(raw))
	for _, r := range raw {
		result := IndexerTestResult{ID: r.ID, IsValid: r.IsValid}
		for _, f := range r.ValidationFailures {
			if f.PropertyName == "" {
				result.Failures = append(result.Failures, f.ErrorMessage)
				continue
			}
			result.Failures = append(result.Failures, f.PropertyName+": "+f.ErrorMessage)
		}
		out = append(out, result)
	}
	return out, nil
}

// ProwlarrListApplications returns the *arr instances Prowlarr syncs to.
func ProwlarrListApplications(ctx context.Context, c *Client) ([]Application, error) {
	return GetJSON[[]Application](ctx, c, "/applications")
}

// ProwlarrListAppProfiles returns the sync profiles indexers can be assigned to.
func ProwlarrListAppProfiles(ctx context.Context, c *Client) ([]AppProfile, error) {
	return GetJSON[[]AppProfile](ctx, c, "/appprofile")
}

// ProwlarrSearch searches all configured indexers for query.
//
// Prowlarr ignores the limit query parameter -- a two-result request against
// the live instance answered with 398 -- so the cap is applied here.
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

// ProwlarrGrabRelease sends a release from prowlarr_search to the download
// client the indexer is configured with.
func ProwlarrGrabRelease(ctx context.Context, c *Client, guid string, indexerID int) error {
	if guid == "" {
		return fmt.Errorf("release guid is required; take it from prowlarr_search")
	}
	if indexerID <= 0 {
		return fmt.Errorf("indexerId is required; take it from prowlarr_search")
	}
	_, err := c.Post(ctx, "/search", map[string]any{"guid": guid, "indexerId": indexerID})
	return err
}

package arr

import (
	"context"
	"fmt"
	"strings"
)

// providerKind validates the provider family selector and returns it as a path
// segment. The four families share one resource shape but live at four
// lowercase, unseparated routes. The error names every valid value so a caller
// that guessed the plural or the upstream spelling can correct itself.
func providerKind(kind string) (string, error) {
	switch kind {
	case "indexer":
		return "indexer", nil
	case "downloadClient":
		return "downloadclient", nil
	case "notification":
		return "notification", nil
	case "importList":
		return "importlist", nil
	default:
		return "", fmt.Errorf(
			"unknown kind %q; valid kinds: indexer, downloadClient, notification, importList", kind)
	}
}

// ProviderDetail is one configured indexer, download client, notification or
// import list with its settings.
//
// Fields carries the provider's own connection settings, and every one whose
// upstream privacy is anything other than "normal" reports "***" instead of its
// value. Sonarr and Radarr mark credentials as apiKey, password or userName;
// the masking is default-deny, so a privacy value this package has never seen
// is masked too.
type ProviderDetail struct {
	Provider
	Kind           string         `json:"kind" jsonschema:"indexer, downloadClient, notification or importList"`
	ConfigContract string         `json:"configContract,omitempty" jsonschema:"the settings contract the implementation uses"`
	Fields         []IndexerField `json:"fields,omitempty" jsonschema:"connection settings by name; credential values report ***"`
}

// ProviderSchemaField names one setting an implementation accepts. There is no
// value member at all: the schema exists to name the settings, and a template
// value is at best a default and at worst an example credential.
type ProviderSchemaField struct {
	Name     string `json:"name" jsonschema:"pass this as a key of the fields argument"`
	Label    string `json:"label,omitempty"`
	Type     string `json:"type,omitempty" jsonschema:"textbox, password, number, checkbox, select, path, tag or info"`
	Privacy  string `json:"privacy,omitempty" jsonschema:"normal, apiKey, password or userName"`
	Advanced bool   `json:"advanced,omitempty"`
}

// ProviderSchema is one implementation a provider can be created from.
type ProviderSchema struct {
	Implementation     string                `json:"implementation" jsonschema:"pass this to the add tool"`
	ImplementationName string                `json:"implementationName,omitempty" jsonschema:"the name the web UI shows"`
	ConfigContract     string                `json:"configContract,omitempty"`
	Protocol           string                `json:"protocol,omitempty" jsonschema:"usenet or torrent, for indexers and download clients"`
	InfoLink           string                `json:"infoLink,omitempty"`
	Fields             []ProviderSchemaField `json:"fields,omitempty"`
}

// ProviderTestResult is the outcome of asking a service to contact one
// provider. It is the same shape as an indexer test: the provider's id, whether
// it answered, and the validation failures when it did not.
type ProviderTestResult = IndexerTestResult

// ProviderFlags are the enable switches a provider resource may declare. Each
// is a pointer so an omitted argument leaves the stored setting alone, and each
// is applied only when the resource actually has it: an indexer has the three
// search switches, a download client has enable, an import list has
// enableAutomaticAdd, and a notification has none of them -- its onGrab,
// onDownload and other triggers are preserved by the read-modify-write instead.
type ProviderFlags struct {
	// Enable is the download client switch.
	Enable *bool
	// EnableRSS, EnableAutomaticSearch and EnableInteractiveSearch are the
	// indexer switches.
	EnableRSS               *bool
	EnableAutomaticSearch   *bool
	EnableInteractiveSearch *bool
	// EnableAutomaticAdd is the import list switch.
	EnableAutomaticAdd *bool
}

// ProviderCreateRequest describes a new provider. Optional settings are
// pointers so an omitted argument keeps the implementation's own default
// instead of resetting it to a zero value.
type ProviderCreateRequest struct {
	// Kind selects the provider family; see providerKind for the valid values.
	Kind string
	// Implementation selects the schema entry to build from.
	Implementation string
	// Name is the display name for the new provider.
	Name string
	// Flags and Priority override the implementation's defaults.
	Flags    ProviderFlags
	Priority *int
	// Tags replaces the implementation's tag list when non-nil.
	Tags []int
	// Fields sets connection settings by field name, e.g. host or apiKey.
	Fields map[string]any
}

// ProviderUpdateRequest changes one existing provider. Every optional member is
// a pointer so an omitted argument leaves that setting alone.
type ProviderUpdateRequest struct {
	Kind     string
	ID       int
	Name     *string
	Flags    ProviderFlags
	Priority *int
	Tags     []int
	Fields   map[string]any
}

// rawProviderDetail mirrors the upstream provider resource. It reuses the
// listing projection for everything but the fields array, which the listing
// deliberately has no member for.
type rawProviderDetail struct {
	rawProvider
	ConfigContract string            `json:"configContract"`
	Fields         []rawIndexerField `json:"fields"`
}

// toProviderDetail projects an upstream provider, masking its credentials.
func (r rawProviderDetail) toProviderDetail(kind string) ProviderDetail {
	return ProviderDetail{
		Provider:       r.toProvider(),
		Kind:           kind,
		ConfigContract: r.ConfigContract,
		Fields:         trimFields(r.Fields),
	}
}

// rawProviderSchema mirrors one entry of a /{kind}/schema listing.
type rawProviderSchema struct {
	Implementation     string            `json:"implementation"`
	ImplementationName string            `json:"implementationName"`
	ConfigContract     string            `json:"configContract"`
	Protocol           string            `json:"protocol"`
	InfoLink           string            `json:"infoLink"`
	Fields             []rawIndexerField `json:"fields"`
}

// toProviderSchema projects one implementation, keeping the field metadata and
// dropping every value.
func (r rawProviderSchema) toProviderSchema() ProviderSchema {
	out := ProviderSchema{
		Implementation:     r.Implementation,
		ImplementationName: r.ImplementationName,
		ConfigContract:     r.ConfigContract,
		Protocol:           r.Protocol,
		InfoLink:           r.InfoLink,
	}
	for _, f := range r.Fields {
		out.Fields = append(out.Fields, ProviderSchemaField{
			Name: f.Name, Label: f.Label, Type: f.Type,
			Privacy: f.Privacy, Advanced: f.Advanced,
		})
	}
	return out
}

// defaultProviderSchemaLimit caps a schema listing. The live instances answer
// with every implementation they ship along with its field metadata -- 26
// notification implementations and 86 KB on stock Radarr -- so a listing is a
// page, narrowed with a query rather than read end to end.
const defaultProviderSchemaLimit = 10

// setIfDeclared writes key on a provider resource, refusing a setting the kind
// does not have. A silently dropped flag would report success while changing
// nothing at all.
func setIfDeclared(resource map[string]any, kind, key string, value any) error {
	if _, ok := resource[key]; !ok {
		return fmt.Errorf("%s is not a setting of a %s provider", key, kind)
	}
	resource[key] = value
	return nil
}

// apply writes the requested switches onto a provider resource. The order is
// fixed so a request setting two impossible flags is rejected the same way
// every time.
func (f ProviderFlags) apply(resource map[string]any, kind string) error {
	for _, flag := range []struct {
		key   string
		value *bool
	}{
		{"enable", f.Enable},
		{"enableRss", f.EnableRSS},
		{"enableAutomaticSearch", f.EnableAutomaticSearch},
		{"enableInteractiveSearch", f.EnableInteractiveSearch},
		{"enableAutomaticAdd", f.EnableAutomaticAdd},
	} {
		if flag.value == nil {
			continue
		}
		if err := setIfDeclared(resource, kind, flag.key, *flag.value); err != nil {
			return err
		}
	}
	return nil
}

// GetProvider returns one configured provider with its settings, with every
// credential field masked.
func GetProvider(ctx context.Context, c *Client, kind string, id int) (ProviderDetail, error) {
	segment, err := providerKind(kind)
	if err != nil {
		return ProviderDetail{}, err
	}
	raw, err := GetJSON[rawProviderDetail](ctx, c, "/"+segment+"/"+itoa(id))
	if err != nil {
		return ProviderDetail{}, err
	}
	return raw.toProviderDetail(kind), nil
}

// ListProviderSchemas returns the implementations of one provider kind whose
// implementation or display name contains query, compared case-insensitively.
// An empty query matches everything. limit defaults to 10.
func ListProviderSchemas(ctx context.Context, c *Client, kind, query string, limit int) ([]ProviderSchema, error) {
	segment, err := providerKind(kind)
	if err != nil {
		return nil, err
	}
	raw, err := GetJSON[[]rawProviderSchema](ctx, c, "/"+segment+"/schema")
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultProviderSchemaLimit
	}
	needle := strings.ToLower(query)
	out := make([]ProviderSchema, 0, limit)
	for _, r := range raw {
		if needle != "" &&
			!strings.Contains(strings.ToLower(r.Implementation), needle) &&
			!strings.Contains(strings.ToLower(r.ImplementationName), needle) {
			continue
		}
		out = append(out, r.toProviderSchema())
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// findProviderSchema returns the raw schema entry for implementation. The entry
// is kept as a map so every key it declares survives into the created provider;
// a typed round-trip would drop the ones this package has no member for and the
// service rejects the incomplete resource.
func findProviderSchema(ctx context.Context, c *Client, segment, implementation string) (map[string]any, error) {
	entries, err := GetJSON[[]map[string]any](ctx, c, "/"+segment+"/schema")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name, _ := entry["implementation"].(string)
		if strings.EqualFold(name, implementation) {
			return entry, nil
		}
	}
	return nil, fmt.Errorf(
		"unknown implementation %q; list the implementations of this kind with the provider schemas tool",
		implementation)
}

// AddProvider creates a provider from one of the service's implementations. The
// schema entry is used as the request body so every key it declares is sent
// back intact, with only the caller's settings patched in.
func AddProvider(ctx context.Context, c *Client, req ProviderCreateRequest) (ProviderDetail, error) {
	segment, err := providerKind(req.Kind)
	if err != nil {
		return ProviderDetail{}, err
	}
	if req.Implementation == "" {
		return ProviderDetail{}, fmt.Errorf("implementation is required")
	}
	if req.Name == "" {
		return ProviderDetail{}, fmt.Errorf("name is required")
	}
	resource, err := findProviderSchema(ctx, c, segment, req.Implementation)
	if err != nil {
		return ProviderDetail{}, err
	}
	resource["name"] = req.Name
	if err := applyProviderEdits(resource, req.Kind, req.Flags, req.Priority, req.Tags, req.Fields); err != nil {
		return ProviderDetail{}, err
	}

	body, err := c.Post(ctx, "/"+segment, resource)
	if err != nil {
		return ProviderDetail{}, err
	}
	var raw rawProviderDetail
	if err := unmarshal(body, &raw); err != nil {
		return ProviderDetail{}, err
	}
	return raw.toProviderDetail(req.Kind), nil
}

// applyProviderEdits patches the settings shared by creation and update onto a
// provider resource.
func applyProviderEdits(
	resource map[string]any, kind string, flags ProviderFlags,
	priority *int, tags []int, fields map[string]any,
) error {
	if err := flags.apply(resource, kind); err != nil {
		return err
	}
	if priority != nil {
		if err := setIfDeclared(resource, kind, "priority", *priority); err != nil {
			return err
		}
	}
	if tags != nil {
		resource["tags"] = tags
	}
	return patchFields(resource, fields)
}

// UpdateProvider changes one provider. The current resource is read as a map
// and written back as one, so settings this package has no member for -- a
// notification's onGrab triggers, an indexer's downloadClientId, the config
// contract -- survive the edit instead of being reset by a typed round-trip.
//
// It also means an untouched credential is sent back at its stored value. The
// mask a read applies is never written: doing so would replace the provider's
// password with three asterisks.
func UpdateProvider(ctx context.Context, c *Client, req ProviderUpdateRequest) (ProviderDetail, error) {
	segment, err := providerKind(req.Kind)
	if err != nil {
		return ProviderDetail{}, err
	}
	if req.ID <= 0 {
		return ProviderDetail{}, fmt.Errorf("provider id is required")
	}
	path := "/" + segment + "/" + itoa(req.ID)
	resource, err := GetJSON[map[string]any](ctx, c, path)
	if err != nil {
		return ProviderDetail{}, err
	}
	if req.Name != nil {
		resource["name"] = *req.Name
	}
	if err := applyProviderEdits(resource, req.Kind, req.Flags, req.Priority, req.Tags, req.Fields); err != nil {
		return ProviderDetail{}, err
	}

	body, err := c.Put(ctx, path, resource)
	if err != nil {
		return ProviderDetail{}, err
	}
	var raw rawProviderDetail
	if err := unmarshal(body, &raw); err != nil {
		return ProviderDetail{}, err
	}
	return raw.toProviderDetail(req.Kind), nil
}

// TestProvider asks the service to contact one provider. A rejected test is
// reported as IsValid false with the reasons, not as an error: "this provider
// is unreachable" is the answer the caller wanted.
func TestProvider(ctx context.Context, c *Client, kind string, id int) (ProviderTestResult, error) {
	segment, err := providerKind(kind)
	if err != nil {
		return ProviderTestResult{}, err
	}
	resource, err := GetJSON[map[string]any](ctx, c, "/"+segment+"/"+itoa(id))
	if err != nil {
		return ProviderTestResult{}, err
	}
	if _, err := c.Post(ctx, "/"+segment+"/test", resource); err != nil {
		body, rejected := rejectedBody(err)
		if !rejected {
			return ProviderTestResult{}, err
		}
		return ProviderTestResult{ID: id, Failures: parseValidationFailures(body)}, nil
	}
	return ProviderTestResult{ID: id, IsValid: true}, nil
}

// DeleteProvider removes one provider. Its configuration, including whatever
// credentials it held, is not recoverable.
func DeleteProvider(ctx context.Context, c *Client, kind string, id int) error {
	segment, err := providerKind(kind)
	if err != nil {
		return err
	}
	_, err = c.Delete(ctx, "/"+segment+"/"+itoa(id))
	return err
}

package arr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// prowlarrRoute is one canned response keyed by "METHOD /path".
type prowlarrRoute struct {
	status int
	body   string
}

// recordedRequest is one request the fake Prowlarr received.
type recordedRequest struct {
	method string
	path   string
	query  string
	body   string
}

// prowlarrFake serves a different canned response per route, which the indexer
// tools need because adding and testing an indexer read a resource before
// writing it back. Requests are recorded in order.
func prowlarrFake(t *testing.T, routes map[string]prowlarrRoute) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	var seen []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent, _ := io.ReadAll(r.Body)
		key := r.Method + " " + r.URL.Path
		seen = append(seen, recordedRequest{
			method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: string(sent),
		})
		route, ok := routes[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"no route for ` + key + `"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(route.status)
		_, _ = w.Write([]byte(route.body))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

// prowlarrClient builds a client against the fake.
func prowlarrClient(url string) *Client {
	return NewClient(url, ProwlarrSpec, Credentials{APIKey: "k"})
}

// indexerWithSecrets is one indexer whose fields cover every privacy value the
// live instance's /indexer/schema reports: normal, apiKey, password, userName.
const indexerWithSecrets = `{
  "id":15,"name":"altHUB","definitionName":"Newznab","implementation":"Newznab",
  "appProfileId":1,"tags":[3],"enable":true,"priority":24,"protocol":"usenet",
  "privacy":"private","language":"en-US","indexerUrls":["https://example.invalid"],
  "fields":[
    {"name":"baseUrl","label":"Url","type":"textbox","privacy":"normal","value":"https://example.invalid"},
    {"name":"apiKey","label":"API Key","type":"textbox","privacy":"apiKey","value":"indexer-api-key-value"},
    {"name":"password","label":"Password","type":"password","privacy":"password","value":"indexer-password-value"},
    {"name":"username","label":"Username","type":"textbox","privacy":"userName","advanced":true,"value":"indexer-user-value"}
  ]
}`

// An indexer's own credentials live in its fields array. Anything whose privacy
// is not "normal" must be replaced before the value can reach a tool result.
func TestProwlarrGetIndexerRedactsSecretFieldsByPrivacy(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v1/indexer/15": {200, indexerWithSecrets},
	})

	detail, err := ProwlarrGetIndexer(context.Background(), prowlarrClient(srv.URL), 15)
	if err != nil {
		t.Fatalf("ProwlarrGetIndexer returned error: %v", err)
	}
	if (*seen)[0].path != "/api/v1/indexer/15" {
		t.Errorf("path = %q, want /api/v1/indexer/15", (*seen)[0].path)
	}
	if detail.DefinitionName != "Newznab" || detail.AppProfileID != 1 || len(detail.Tags) != 1 {
		t.Errorf("detail = %+v, want definitionName/appProfileId/tags populated", detail)
	}
	if detail.Privacy != "private" || detail.Language != "en-US" || len(detail.IndexerURLs) != 1 {
		t.Errorf("detail = %+v, want privacy/language/indexerUrls populated", detail)
	}

	values := map[string]any{}
	for _, f := range detail.Fields {
		values[f.Name] = f.Value
	}
	if values["baseUrl"] != "https://example.invalid" {
		t.Errorf("non-secret field was altered: %v", values["baseUrl"])
	}
	for _, secret := range []string{"apiKey", "password", "username"} {
		if values[secret] != "***" {
			t.Errorf("field %q value = %v, want ***", secret, values[secret])
		}
	}

	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshalling detail: %v", err)
	}
	for _, secret := range []string{"indexer-api-key-value", "indexer-password-value", "indexer-user-value"} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("serialised detail leaks %q: %s", secret, encoded)
		}
	}
}

// schemaList is a trimmed stand-in for /indexer/schema, which on the live
// instance is 624 definitions and 5.7 MB.
const schemaList = `[
  {"id":null,"name":"abNZB","definitionName":"Newznab","implementation":"Newznab","protocol":"usenet",
   "privacy":"private","language":"en-US","description":"Newznab is an API search specification for Usenet",
   "fields":[
     {"name":"baseUrl","label":"Url","type":"textbox","privacy":"normal","value":"https://abnzb.invalid"},
     {"name":"apiKey","label":"API Key","type":"textbox","privacy":"apiKey","value":null}
   ]},
  {"id":null,"name":"1337x","definitionName":"1337x","implementation":"Cardigann","protocol":"torrent",
   "privacy":"public","language":"en-US","description":"1337x is a Public torrent site",
   "fields":[{"name":"definitionFile","label":null,"type":"textbox","privacy":"normal","value":"1337x"}]},
  {"id":null,"name":"Knaben","definitionName":"Knaben","implementation":"Knaben","protocol":"torrent",
   "privacy":"public","language":"en-US","description":"Knaben is a Public torrent meta-search",
   "fields":[{"name":"baseUrl","label":"Url","type":"textbox","privacy":"normal","value":"https://knaben.invalid"}]}
]`

// The raw schema list is hundreds of definitions each carrying its own fields,
// so the list form filters, caps and drops fields; only the detail form has them.
func TestProwlarrListIndexerSchemasFiltersAndDropsFields(t *testing.T) {
	srv, _ := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v1/indexer/schema": {200, schemaList},
	})

	schemas, err := ProwlarrListIndexerSchemas(context.Background(), prowlarrClient(srv.URL), "NEWZ", 0)
	if err != nil {
		t.Fatalf("ProwlarrListIndexerSchemas returned error: %v", err)
	}
	if len(schemas) != 1 {
		t.Fatalf("schemas = %d, want 1 matching \"NEWZ\" case-insensitively: %+v", len(schemas), schemas)
	}
	if schemas[0].DefinitionName != "Newznab" || schemas[0].Protocol != "usenet" {
		t.Errorf("schema = %+v, want the Newznab definition", schemas[0])
	}
	if schemas[0].Fields != nil {
		t.Errorf("list form must not carry fields: %+v", schemas[0].Fields)
	}

	all, err := ProwlarrListIndexerSchemas(context.Background(), prowlarrClient(srv.URL), "", 2)
	if err != nil {
		t.Fatalf("ProwlarrListIndexerSchemas returned error: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("limit 2 returned %d schemas", len(all))
	}
}

func TestProwlarrGetIndexerSchemaRedactsFields(t *testing.T) {
	srv, _ := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v1/indexer/schema": {200, schemaList},
	})

	schema, err := ProwlarrGetIndexerSchema(context.Background(), prowlarrClient(srv.URL), "newznab")
	if err != nil {
		t.Fatalf("ProwlarrGetIndexerSchema returned error: %v", err)
	}
	if len(schema.Fields) != 2 {
		t.Fatalf("fields = %d, want 2", len(schema.Fields))
	}
	for _, f := range schema.Fields {
		if f.Privacy != "normal" && f.Value != "***" {
			t.Errorf("field %q value = %v, want *** for privacy %q", f.Name, f.Value, f.Privacy)
		}
	}
}

func TestProwlarrGetIndexerSchemaRejectsUnknownDefinition(t *testing.T) {
	srv, _ := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v1/indexer/schema": {200, schemaList},
	})

	_, err := ProwlarrGetIndexerSchema(context.Background(), prowlarrClient(srv.URL), "nosuchthing")
	if err == nil {
		t.Fatal("expected an error for an unknown definition name")
	}
	if !strings.Contains(err.Error(), "nosuchthing") {
		t.Errorf("error %q does not name the definition the caller asked for", err)
	}
}

// Adding an indexer starts from the schema entry, because the upstream resource
// needs every field the definition declares, not just the ones the caller set.
func TestProwlarrAddIndexerPatchesSchemaFieldsAndPosts(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v1/indexer/schema": {200, schemaList},
		"POST /api/v1/indexer":       {201, indexerWithSecrets},
	})

	enable := true
	priority := 30
	detail, err := ProwlarrAddIndexer(context.Background(), prowlarrClient(srv.URL), IndexerCreateRequest{
		DefinitionName: "Newznab",
		Name:           "My Newznab",
		Enable:         &enable,
		Priority:       &priority,
		Tags:           []int{3},
		Fields:         map[string]any{"apiKey": "brand-new-key", "baseUrl": "https://mine.invalid"},
	})
	if err != nil {
		t.Fatalf("ProwlarrAddIndexer returned error: %v", err)
	}
	if len(*seen) != 2 {
		t.Fatalf("requests = %v, want a schema GET then an indexer POST", *seen)
	}
	if (*seen)[1].method != http.MethodPost || (*seen)[1].path != "/api/v1/indexer" {
		t.Errorf("second request = %s %s, want POST /api/v1/indexer", (*seen)[1].method, (*seen)[1].path)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte((*seen)[1].body), &sent); err != nil {
		t.Fatalf("decoding posted body: %v", err)
	}
	if sent["name"] != "My Newznab" {
		t.Errorf("posted name = %v, want My Newznab", sent["name"])
	}
	if sent["enable"] != true || sent["priority"] != float64(30) {
		t.Errorf("posted enable/priority = %v/%v, want true/30", sent["enable"], sent["priority"])
	}
	posted := map[string]any{}
	for _, entry := range sent["fields"].([]any) {
		field := entry.(map[string]any)
		posted[field["name"].(string)] = field["value"]
	}
	if posted["apiKey"] != "brand-new-key" {
		t.Errorf("posted apiKey = %v, want brand-new-key", posted["apiKey"])
	}
	if posted["baseUrl"] != "https://mine.invalid" {
		t.Errorf("posted baseUrl = %v, want the caller's value", posted["baseUrl"])
	}
	// The result comes back through the same redaction as a plain read.
	for _, f := range detail.Fields {
		if f.Name == "apiKey" && f.Value != "***" {
			t.Errorf("created indexer returned its apiKey: %v", f.Value)
		}
	}
}

// A misspelled field name would otherwise be silently dropped, leaving an
// indexer that cannot authenticate. The error has to be self-correctable.
func TestProwlarrAddIndexerRejectsUnknownFieldNameWithValidNames(t *testing.T) {
	srv, _ := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v1/indexer/schema": {200, schemaList},
	})

	_, err := ProwlarrAddIndexer(context.Background(), prowlarrClient(srv.URL), IndexerCreateRequest{
		DefinitionName: "Newznab",
		Name:           "Typo",
		Fields:         map[string]any{"api_key": "x"},
	})
	if err == nil {
		t.Fatal("expected an error for an unknown field name")
	}
	if !strings.Contains(err.Error(), "api_key") {
		t.Errorf("error %q does not name the offending field", err)
	}
	for _, want := range []string{"apiKey", "baseUrl"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not list valid field %q", err, want)
		}
	}
}

// The upstream resource carries settings this package has no member for. The
// update reads it as a map and writes the same map back so those keys survive;
// decoding into a typed struct would silently reset them.
func TestProwlarrUpdateIndexerRoundTripsUnknownKeys(t *testing.T) {
	current := `{"id":15,"name":"altHUB","definitionName":"Newznab","implementation":"Newznab",
	  "enable":true,"priority":24,"appProfileId":1,"tags":[],"protocol":"usenet",
	  "downloadClientId":7,"supportsRedirect":true,"configContract":"NewznabSettings",
	  "fields":[{"name":"apiKey","label":"API Key","privacy":"apiKey","value":"old-key"}]}`
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v1/indexer/15": {200, current},
		"PUT /api/v1/indexer/15": {200, indexerWithSecrets},
	})

	enable := false
	if _, err := ProwlarrUpdateIndexer(context.Background(), prowlarrClient(srv.URL), IndexerUpdateRequest{
		ID:     15,
		Enable: &enable,
		Fields: map[string]any{"apiKey": "rotated-key"},
	}); err != nil {
		t.Fatalf("ProwlarrUpdateIndexer returned error: %v", err)
	}
	if len(*seen) != 2 || (*seen)[1].method != http.MethodPut {
		t.Fatalf("requests = %v, want a GET then a PUT", *seen)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte((*seen)[1].body), &sent); err != nil {
		t.Fatalf("decoding the PUT body: %v", err)
	}
	for key, want := range map[string]any{
		"downloadClientId": float64(7),
		"supportsRedirect": true,
		"configContract":   "NewznabSettings",
	} {
		if sent[key] != want {
			t.Errorf("PUT dropped unknown key %q: got %v, want %v", key, sent[key], want)
		}
	}
	if sent["enable"] != false {
		t.Errorf("PUT enable = %v, want false", sent["enable"])
	}
	if sent["name"] != "altHUB" {
		t.Errorf("PUT name = %v, want the unchanged name", sent["name"])
	}
	field := sent["fields"].([]any)[0].(map[string]any)
	if field["value"] != "rotated-key" {
		t.Errorf("PUT apiKey = %v, want rotated-key", field["value"])
	}
}

// A rejected test is an answer, not a transport failure: the caller wants to
// know why the indexer is unhealthy, not that a request returned 400.
func TestProwlarrTestIndexerReportsValidationFailures(t *testing.T) {
	srv, _ := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v1/indexer/15":    {200, indexerWithSecrets},
		"POST /api/v1/indexer/test": {400, `[{"propertyName":"apiKey","errorMessage":"Unable to connect to indexer"}]`},
	})

	result, err := ProwlarrTestIndexer(context.Background(), prowlarrClient(srv.URL), 15)
	if err != nil {
		t.Fatalf("a failed indexer test must not be a tool error: %v", err)
	}
	if result.IsValid {
		t.Error("result.IsValid = true for a 400 response")
	}
	if len(result.Failures) != 1 || !strings.Contains(result.Failures[0], "Unable to connect") {
		t.Errorf("failures = %v, want the upstream validation message", result.Failures)
	}
	if result.ID != 15 {
		t.Errorf("result.ID = %d, want 15", result.ID)
	}
}

func TestProwlarrTestIndexerReportsSuccess(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v1/indexer/15":    {200, indexerWithSecrets},
		"POST /api/v1/indexer/test": {200, ``},
	})

	result, err := ProwlarrTestIndexer(context.Background(), prowlarrClient(srv.URL), 15)
	if err != nil {
		t.Fatalf("ProwlarrTestIndexer returned error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("result = %+v, want IsValid for a 200 response", result)
	}
	if len(*seen) != 2 || (*seen)[1].path != "/api/v1/indexer/test" {
		t.Errorf("requests = %v, want the resource read then POST /api/v1/indexer/test", *seen)
	}
}

func TestProwlarrTestAllIndexersReportsEachIndexer(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"POST /api/v1/indexer/testall": {200, `[
		  {"id":15,"isValid":true,"validationFailures":[]},
		  {"id":20,"isValid":false,"validationFailures":[{"propertyName":"baseUrl","errorMessage":"timed out"}]}
		]`},
	})

	results, err := ProwlarrTestAllIndexers(context.Background(), prowlarrClient(srv.URL))
	if err != nil {
		t.Fatalf("ProwlarrTestAllIndexers returned error: %v", err)
	}
	if (*seen)[0].path != "/api/v1/indexer/testall" {
		t.Errorf("path = %q, want /api/v1/indexer/testall", (*seen)[0].path)
	}
	if len(results) != 2 || !results[0].IsValid || results[1].IsValid {
		t.Fatalf("results = %+v, want one valid and one invalid", results)
	}
	if len(results[1].Failures) != 1 || !strings.Contains(results[1].Failures[0], "timed out") {
		t.Errorf("failures = %v, want the upstream message", results[1].Failures)
	}
}

// Grabbing sends the release back by identity. downloadUrl is deliberately not
// part of it: on the live instance 553 of 724 search results carry one and at
// least one embeds an apikey query parameter.
func TestProwlarrGrabReleasePostsGUIDAndIndexerID(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"POST /api/v1/search": {200, `{}`},
	})

	err := ProwlarrGrabRelease(context.Background(), prowlarrClient(srv.URL), "magnet:?xt=urn:btih:abc", 4)
	if err != nil {
		t.Fatalf("ProwlarrGrabRelease returned error: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte((*seen)[0].body), &sent); err != nil {
		t.Fatalf("decoding posted body: %v", err)
	}
	if sent["guid"] != "magnet:?xt=urn:btih:abc" || sent["indexerId"] != float64(4) {
		t.Errorf("posted body = %v, want guid and indexerId", sent)
	}
}

// A search result without its guid and indexerId cannot be grabbed afterwards.
func TestProwlarrSearchCarriesGrabIdentity(t *testing.T) {
	srv, _ := fakeService(t, 200, `[
	  {"guid":"magnet:?xt=urn:btih:abc","indexerId":4,"indexer":"The Pirate Bay","title":"Ubuntu 22.04",
	   "size":3654957056,"seeders":37,"protocol":"torrent","publishDate":"2022-05-18T12:33:51Z",
	   "downloadUrl":"https://example.invalid/dl?apikey=leaked-key"}
	]`)

	results, err := ProwlarrSearch(context.Background(), prowlarrClient(srv.URL), "ubuntu", nil, 10)
	if err != nil {
		t.Fatalf("ProwlarrSearch returned error: %v", err)
	}
	if len(results) != 1 || results[0].GUID == "" || results[0].IndexerID != 4 {
		t.Fatalf("results = %+v, want guid and indexerId populated", results)
	}
	encoded, _ := json.Marshal(results)
	if strings.Contains(string(encoded), "leaked-key") {
		t.Errorf("search result carries the download url's api key: %s", encoded)
	}
}

func TestProwlarrListApplicationsAndAppProfiles(t *testing.T) {
	srv, _ := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v1/applications": {200, `[{"id":1,"name":"Radarr","implementation":"Radarr",
		  "syncLevel":"fullSync","tags":[],"fields":[{"name":"apiKey","value":"radarr-key-leak"}]}]`},
		"GET /api/v1/appprofile": {200, `[{"id":1,"name":"Standard","enableRss":true,
		  "enableAutomaticSearch":true,"enableInteractiveSearch":true,"minimumSeeders":100}]`},
	})
	c := prowlarrClient(srv.URL)

	apps, err := ProwlarrListApplications(context.Background(), c)
	if err != nil {
		t.Fatalf("ProwlarrListApplications returned error: %v", err)
	}
	if len(apps) != 1 || apps[0].SyncLevel != "fullSync" {
		t.Fatalf("apps = %+v, want one fullSync application", apps)
	}
	encoded, _ := json.Marshal(apps)
	if strings.Contains(string(encoded), "radarr-key-leak") {
		t.Errorf("application listing leaks the connected app's api key: %s", encoded)
	}

	profiles, err := ProwlarrListAppProfiles(context.Background(), c)
	if err != nil {
		t.Fatalf("ProwlarrListAppProfiles returned error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].MinimumSeeders != 100 || !profiles[0].EnableRSS {
		t.Errorf("profiles = %+v, want the Standard profile", profiles)
	}
}

func TestProwlarrDeleteIndexerTargetsTheIndexer(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"DELETE /api/v1/indexer/15": {200, ``},
	})

	if err := ProwlarrDeleteIndexer(context.Background(), prowlarrClient(srv.URL), 15); err != nil {
		t.Fatalf("ProwlarrDeleteIndexer returned error: %v", err)
	}
	if (*seen)[0].method != http.MethodDelete || (*seen)[0].path != "/api/v1/indexer/15" {
		t.Errorf("request = %s %s, want DELETE /api/v1/indexer/15", (*seen)[0].method, (*seen)[0].path)
	}
}

// The mask exists only in the projection returned to the model. An update
// that does not mention a credential must send the real value back, because
// the request is built from the raw upstream resource. Projecting the trimmed
// struct into the PUT instead would write the literal "***" over every stored
// password on the next unrelated edit, which upstream would accept silently.
func TestProwlarrUpdateIndexerNeverWritesTheMaskOverUntouchedSecrets(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v1/indexer/15": {200, indexerWithSecrets},
		"PUT /api/v1/indexer/15": {200, indexerWithSecrets},
	})

	priority := 30
	if _, err := ProwlarrUpdateIndexer(context.Background(), prowlarrClient(srv.URL), IndexerUpdateRequest{
		ID:       15,
		Priority: &priority,
	}); err != nil {
		t.Fatalf("ProwlarrUpdateIndexer returned error: %v", err)
	}

	body := (*seen)[1].body
	if strings.Contains(body, secretMask) {
		t.Errorf("PUT body carries the redaction mask, which would erase the stored credential: %s", body)
	}
	for _, secret := range []string{"indexer-api-key-value", "indexer-password-value", "indexer-user-value"} {
		if !strings.Contains(body, secret) {
			t.Errorf("PUT body dropped the untouched credential %q: %s", secret, body)
		}
	}
}

package arr

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// providerClient builds a client for the provider tests. Sonarr and Radarr
// serve these routes identically under /api/v3, so one spec pins both.
func providerClient(url string) *Client {
	return NewClient(url, SonarrSpec, Credentials{APIKey: "k"})
}

// providerKindPaths is the kind enum and the path segment each maps to on the
// live instances, whose routes are all lowercase and unseparated.
var providerKindPaths = map[string]string{
	"indexer":        "indexer",
	"downloadClient": "downloadclient",
	"notification":   "notification",
	"importList":     "importlist",
}

// providerWithSecrets is one stored provider whose fields cover every privacy
// value the live instances report -- normal, apiKey, password and userName --
// plus one this package has never seen, which must be masked all the same.
const providerWithSecrets = `{
  "id":7,"name":"NZBGet","implementation":"Nzbget","configContract":"NzbgetSettings",
  "protocol":"usenet","enable":true,"priority":1,"tags":[2],
  "fields":[
    {"name":"host","label":"Host","type":"textbox","privacy":"normal","value":"nzbget.invalid"},
    {"name":"apiKey","label":"API Key","type":"textbox","privacy":"apiKey","value":"provider-api-key-value"},
    {"name":"password","label":"Password","type":"password","privacy":"password","value":"provider-password-value"},
    {"name":"username","label":"Username","type":"textbox","privacy":"userName","value":"provider-user-value"},
    {"name":"bearerToken","label":"Token","type":"textbox","privacy":"bearerToken","value":"provider-token-value"}
  ]
}`

// providerSecretValues are the credential values that must never leave the
// package, whatever the field is called.
var providerSecretValues = []string{
	"provider-api-key-value", "provider-password-value",
	"provider-user-value", "provider-token-value",
}

// A provider's own credentials live in its fields array. Anything whose privacy
// is not "normal" must be replaced before the value can reach a tool result,
// and the rule is default-deny so a privacy value nobody has seen yet -- here
// bearerToken -- cannot leak by being unrecognised.
func TestGetProviderMasksEveryNonNormalPrivacyForEveryKind(t *testing.T) {
	for kind, segment := range providerKindPaths {
		t.Run(kind, func(t *testing.T) {
			srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
				"GET /api/v3/" + segment + "/7": {200, providerWithSecrets},
			})

			detail, err := GetProvider(context.Background(), providerClient(srv.URL), kind, 7)
			if err != nil {
				t.Fatalf("GetProvider returned error: %v", err)
			}
			if (*seen)[0].path != "/api/v3/"+segment+"/7" {
				t.Errorf("path = %q, want /api/v3/%s/7", (*seen)[0].path, segment)
			}
			if detail.ID != 7 || detail.Name != "NZBGet" || detail.ConfigContract != "NzbgetSettings" {
				t.Errorf("detail = %+v, want the identity fields populated", detail)
			}
			if detail.Kind != kind {
				t.Errorf("detail.Kind = %q, want %q", detail.Kind, kind)
			}

			values := map[string]any{}
			for _, f := range detail.Fields {
				values[f.Name] = f.Value
			}
			if values["host"] != "nzbget.invalid" {
				t.Errorf("non-secret field was altered: %v", values["host"])
			}
			for _, secret := range []string{"apiKey", "password", "username", "bearerToken"} {
				if values[secret] != "***" {
					t.Errorf("field %q value = %v, want ***", secret, values[secret])
				}
			}

			encoded, err := json.Marshal(detail)
			if err != nil {
				t.Fatalf("marshalling detail: %v", err)
			}
			for _, secret := range providerSecretValues {
				if strings.Contains(string(encoded), secret) {
					t.Errorf("serialised detail leaks %q: %s", secret, encoded)
				}
			}
		})
	}
}

// The kind is an enum, and a caller that guessed the plural or the upstream
// path spelling has to be able to correct itself from the error alone.
func TestProviderKindRejectsUnknownValueNamingTheValidOnes(t *testing.T) {
	srv, _ := prowlarrFake(t, map[string]prowlarrRoute{})

	_, err := GetProvider(context.Background(), providerClient(srv.URL), "indexers", 7)
	if err == nil {
		t.Fatal("expected an error for an unknown kind")
	}
	if !strings.Contains(err.Error(), "indexers") {
		t.Errorf("error %q does not name the kind the caller asked for", err)
	}
	for _, want := range []string{"indexer", "downloadClient", "notification", "importList"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not list valid kind %q", err, want)
		}
	}
}

// downloadClientSchemas is a trimmed stand-in for /downloadclient/schema, which
// on the live Sonarr instance is 18 implementations and 64 KB.
const downloadClientSchemas = `[
  {"enable":true,"protocol":"usenet","priority":1,"removeCompletedDownloads":true,"name":"",
   "implementation":"Nzbget","implementationName":"NZBGet","configContract":"NzbgetSettings",
   "infoLink":"https://wiki.invalid/nzbget","tags":[],
   "fields":[
     {"order":0,"name":"host","label":"Host","type":"textbox","privacy":"normal","value":"localhost"},
     {"order":5,"name":"password","label":"Password","type":"password","privacy":"password","value":"schema-default-password"}
   ]},
  {"enable":true,"protocol":"torrent","priority":1,"name":"",
   "implementation":"QBittorrent","implementationName":"qBittorrent","configContract":"QBittorrentSettings",
   "fields":[{"order":0,"name":"host","label":"Host","type":"textbox","privacy":"normal","value":"localhost"}]}
]`

// The schema listing exists to name the settings an implementation takes. A
// template value is at best a default and at worst an example credential, so
// none is returned at all.
func TestListProviderSchemasFiltersByQueryAndNeverReturnsValues(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v3/downloadclient/schema": {200, downloadClientSchemas},
	})

	schemas, err := ListProviderSchemas(context.Background(), providerClient(srv.URL), "downloadClient", "QBIT", 0)
	if err != nil {
		t.Fatalf("ListProviderSchemas returned error: %v", err)
	}
	if (*seen)[0].path != "/api/v3/downloadclient/schema" {
		t.Errorf("path = %q, want /api/v3/downloadclient/schema", (*seen)[0].path)
	}
	if len(schemas) != 1 {
		t.Fatalf("schemas = %d, want 1 matching \"QBIT\" case-insensitively: %+v", len(schemas), schemas)
	}
	if schemas[0].Implementation != "QBittorrent" || schemas[0].ImplementationName != "qBittorrent" {
		t.Errorf("schema = %+v, want the qBittorrent implementation", schemas[0])
	}

	all, err := ListProviderSchemas(context.Background(), providerClient(srv.URL), "downloadClient", "", 0)
	if err != nil {
		t.Fatalf("ListProviderSchemas returned error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("empty query returned %d schemas, want 2", len(all))
	}
	encoded, err := json.Marshal(all)
	if err != nil {
		t.Fatalf("marshalling schemas: %v", err)
	}
	if strings.Contains(string(encoded), "schema-default-password") {
		t.Errorf("schema listing carries a field value: %s", encoded)
	}
	if strings.Contains(string(encoded), `"value"`) {
		t.Errorf("schema listing has a value member at all: %s", encoded)
	}
	if !strings.Contains(string(encoded), "password") || !strings.Contains(string(encoded), "Host") {
		t.Errorf("schema listing dropped the field names and labels: %s", encoded)
	}

	capped, err := ListProviderSchemas(context.Background(), providerClient(srv.URL), "downloadClient", "", 1)
	if err != nil {
		t.Fatalf("ListProviderSchemas returned error: %v", err)
	}
	if len(capped) != 1 {
		t.Errorf("limit 1 returned %d schemas", len(capped))
	}
}

// Adding starts from the schema entry, because the upstream resource needs
// every key the implementation declares, not just the ones the caller set.
func TestAddProviderPatchesTheSchemaTemplateAndPosts(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v3/downloadclient/schema": {200, downloadClientSchemas},
		"POST /api/v3/downloadclient":       {201, providerWithSecrets},
	})

	enable := false
	priority := 3
	detail, err := AddProvider(context.Background(), providerClient(srv.URL), ProviderCreateRequest{
		Kind:           "downloadClient",
		Implementation: "nzbget",
		Name:           "My NZBGet",
		Flags:          ProviderFlags{Enable: &enable},
		Priority:       &priority,
		Tags:           []int{2},
		Fields:         map[string]any{"host": "nzb.invalid", "password": "brand-new-password"},
	})
	if err != nil {
		t.Fatalf("AddProvider returned error: %v", err)
	}
	if len(*seen) != 2 {
		t.Fatalf("requests = %v, want a schema GET then a POST", *seen)
	}
	if (*seen)[1].method != http.MethodPost || (*seen)[1].path != "/api/v3/downloadclient" {
		t.Errorf("second request = %s %s, want POST /api/v3/downloadclient",
			(*seen)[1].method, (*seen)[1].path)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte((*seen)[1].body), &sent); err != nil {
		t.Fatalf("decoding the posted body: %v", err)
	}
	if sent["name"] != "My NZBGet" {
		t.Errorf("posted name = %v, want My NZBGet", sent["name"])
	}
	if sent["enable"] != false || sent["priority"] != float64(3) {
		t.Errorf("posted enable/priority = %v/%v, want false/3", sent["enable"], sent["priority"])
	}
	if sent["configContract"] != "NzbgetSettings" {
		t.Errorf("posted body dropped the template's configContract: %v", sent["configContract"])
	}
	posted := map[string]any{}
	for _, entry := range sent["fields"].([]any) {
		field := entry.(map[string]any)
		posted[field["name"].(string)] = field["value"]
	}
	if posted["host"] != "nzb.invalid" || posted["password"] != "brand-new-password" {
		t.Errorf("posted fields = %v, want the caller's values", posted)
	}
	// The created provider comes back through the same redaction as a read.
	for _, f := range detail.Fields {
		if f.Name == "password" && f.Value != "***" {
			t.Errorf("created provider returned its password: %v", f.Value)
		}
	}
}

// A misspelled field name would otherwise be silently dropped, leaving a
// provider that cannot authenticate. The error has to be self-correctable.
func TestAddProviderRejectsUnknownFieldNameListingValidNames(t *testing.T) {
	srv, _ := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v3/downloadclient/schema": {200, downloadClientSchemas},
	})

	_, err := AddProvider(context.Background(), providerClient(srv.URL), ProviderCreateRequest{
		Kind:           "downloadClient",
		Implementation: "Nzbget",
		Name:           "Typo",
		Fields:         map[string]any{"hostname": "x"},
	})
	if err == nil {
		t.Fatal("expected an error for an unknown field name")
	}
	if !strings.Contains(err.Error(), "hostname") {
		t.Errorf("error %q does not name the offending field", err)
	}
	for _, want := range []string{"host", "password"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not list valid field %q", err, want)
		}
	}
}

func TestAddProviderRejectsUnknownImplementation(t *testing.T) {
	srv, _ := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v3/downloadclient/schema": {200, downloadClientSchemas},
	})

	_, err := AddProvider(context.Background(), providerClient(srv.URL), ProviderCreateRequest{
		Kind:           "downloadClient",
		Implementation: "Nosuchclient",
		Name:           "Nope",
	})
	if err == nil {
		t.Fatal("expected an error for an unknown implementation")
	}
	if !strings.Contains(err.Error(), "Nosuchclient") {
		t.Errorf("error %q does not name the implementation the caller asked for", err)
	}
}

// The most dangerous regression this package can have: the mask a read applies
// must never be written back, because that would replace the stored credential
// with three asterisks and silently break the provider.
func TestUpdateProviderNeverWritesTheMaskOverUntouchedSecrets(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v3/downloadclient/7": {200, providerWithSecrets},
		"PUT /api/v3/downloadclient/7": {200, providerWithSecrets},
	})

	priority := 5
	if _, err := UpdateProvider(context.Background(), providerClient(srv.URL), ProviderUpdateRequest{
		Kind:     "downloadClient",
		ID:       7,
		Priority: &priority,
	}); err != nil {
		t.Fatalf("UpdateProvider returned error: %v", err)
	}

	body := (*seen)[1].body
	if strings.Contains(body, secretMask) {
		t.Errorf("PUT body carries the redaction mask, which would erase the stored credential: %s", body)
	}
	for _, secret := range providerSecretValues {
		if !strings.Contains(body, secret) {
			t.Errorf("PUT body dropped the untouched credential %q: %s", secret, body)
		}
	}
}

// The upstream resource carries settings this package has no member for. The
// update reads it as a map and writes the same map back so those keys survive;
// decoding into a typed struct would silently reset them.
func TestUpdateProviderRoundTripsUnknownUpstreamKeys(t *testing.T) {
	current := `{"id":4,"name":"Webhook","implementation":"Webhook","configContract":"WebhookSettings",
	  "onGrab":false,"onDownload":true,"onHealthIssue":true,"includeHealthWarnings":false,
	  "supportsOnGrab":true,"tags":[],
	  "fields":[{"name":"url","label":"Webhook URL","privacy":"normal","value":"https://hook.invalid"}]}`
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v3/notification/4": {200, current},
		"PUT /api/v3/notification/4": {200, current},
	})

	name := "Renamed"
	if _, err := UpdateProvider(context.Background(), providerClient(srv.URL), ProviderUpdateRequest{
		Kind:   "notification",
		ID:     4,
		Name:   &name,
		Tags:   []int{9},
		Fields: map[string]any{"url": "https://other.invalid"},
	}); err != nil {
		t.Fatalf("UpdateProvider returned error: %v", err)
	}
	if len(*seen) != 2 || (*seen)[1].method != http.MethodPut {
		t.Fatalf("requests = %v, want a GET then a PUT", *seen)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte((*seen)[1].body), &sent); err != nil {
		t.Fatalf("decoding the PUT body: %v", err)
	}
	for key, want := range map[string]any{
		"onDownload":     true,
		"onHealthIssue":  true,
		"supportsOnGrab": true,
		"configContract": "WebhookSettings",
	} {
		if sent[key] != want {
			t.Errorf("PUT dropped upstream key %q: got %v, want %v", key, sent[key], want)
		}
	}
	if sent["name"] != "Renamed" {
		t.Errorf("PUT name = %v, want Renamed", sent["name"])
	}
	field := sent["fields"].([]any)[0].(map[string]any)
	if field["value"] != "https://other.invalid" {
		t.Errorf("PUT url field = %v, want the caller's value", field["value"])
	}
}

// The enable flags differ per kind: a download client has enable, an indexer
// has three search switches, a notification has none of them. Setting one the
// resource does not declare would report success while changing nothing.
func TestUpdateProviderRejectsAFlagTheKindDoesNotDeclare(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v3/downloadclient/7": {200, providerWithSecrets},
	})

	on := true
	_, err := UpdateProvider(context.Background(), providerClient(srv.URL), ProviderUpdateRequest{
		Kind:  "downloadClient",
		ID:    7,
		Flags: ProviderFlags{EnableRSS: &on},
	})
	if err == nil {
		t.Fatal("expected an error for a flag the download client does not have")
	}
	if !strings.Contains(err.Error(), "enableRss") || !strings.Contains(err.Error(), "downloadClient") {
		t.Errorf("error %q does not say which flag is wrong for which kind", err)
	}
	if len(*seen) != 1 {
		t.Errorf("requests = %v, want the read only; nothing may be written", *seen)
	}
}

// A rejected test is an answer, not a transport failure: the caller wants to
// know why the provider is unhealthy, not that a request returned 400.
func TestTestProviderReportsValidationFailuresInsteadOfAnError(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v3/notification/4":     {200, providerWithSecrets},
		"POST /api/v3/notification/test": {400, `[{"propertyName":"url","errorMessage":"Unable to connect"}]`},
	})

	result, err := TestProvider(context.Background(), providerClient(srv.URL), "notification", 4)
	if err != nil {
		t.Fatalf("a failed provider test must not be a tool error: %v", err)
	}
	if result.IsValid {
		t.Error("result.IsValid = true for a 400 response")
	}
	if len(result.Failures) != 1 || !strings.Contains(result.Failures[0], "Unable to connect") {
		t.Errorf("failures = %v, want the upstream validation message", result.Failures)
	}
	if result.ID != 4 {
		t.Errorf("result.ID = %d, want 4", result.ID)
	}
	if len(*seen) != 2 || (*seen)[1].path != "/api/v3/notification/test" {
		t.Errorf("requests = %v, want the resource read then POST /api/v3/notification/test", *seen)
	}
}

func TestTestProviderReportsSuccess(t *testing.T) {
	srv, _ := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v3/indexer/7":     {200, providerWithSecrets},
		"POST /api/v3/indexer/test": {200, ``},
	})

	result, err := TestProvider(context.Background(), providerClient(srv.URL), "indexer", 7)
	if err != nil {
		t.Fatalf("TestProvider returned error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("result = %+v, want IsValid for a 200 response", result)
	}
}

// The stored resource is what gets tested, so the credentials it holds must go
// back out unmasked or every test of a private provider would fail.
func TestTestProviderSendsTheStoredCredentialsNotTheMask(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"GET /api/v3/indexer/7":     {200, providerWithSecrets},
		"POST /api/v3/indexer/test": {200, ``},
	})

	if _, err := TestProvider(context.Background(), providerClient(srv.URL), "indexer", 7); err != nil {
		t.Fatalf("TestProvider returned error: %v", err)
	}
	if body := (*seen)[1].body; strings.Contains(body, secretMask) {
		t.Errorf("test body carries the redaction mask instead of the stored value: %s", body)
	}
}

func TestDeleteProviderHitsTheKindPath(t *testing.T) {
	srv, seen := prowlarrFake(t, map[string]prowlarrRoute{
		"DELETE /api/v3/importlist/12": {200, ``},
	})

	if err := DeleteProvider(context.Background(), providerClient(srv.URL), "importList", 12); err != nil {
		t.Fatalf("DeleteProvider returned error: %v", err)
	}
	if (*seen)[0].method != http.MethodDelete || (*seen)[0].path != "/api/v3/importlist/12" {
		t.Errorf("request = %s %s, want DELETE /api/v3/importlist/12", (*seen)[0].method, (*seen)[0].path)
	}
}

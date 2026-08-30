package server

import (
	"context"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// providerReadTools are the provider tools a readonly deployment must still
// advertise, as name suffixes shared by both media services.
var providerReadTools = []string{"_provider_schemas", "_get_provider"}

// providerWriteTools create or modify provider configuration. Testing a
// provider counts: it makes the service contact the remote and records the
// outcome against the stored provider.
var providerWriteTools = []string{"_add_provider", "_update_provider", "_test_provider"}

// providerDestructiveTools remove configuration that cannot be recovered.
var providerDestructiveTools = []string{"_delete_provider"}

// One implementation serves both services, so neither can quietly fall behind
// the other.
func TestProviderToolsAreRegisteredForBothMediaServices(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	names := toolNames(t, connect(t, mediaCfg(srv.URL)))

	var want []string
	want = append(want, providerReadTools...)
	want = append(want, providerWriteTools...)
	want = append(want, providerDestructiveTools...)
	for _, suffix := range want {
		for _, svc := range []string{"sonarr", "radarr"} {
			if !has(names, svc+suffix) {
				t.Errorf("tool %q not advertised", svc+suffix)
			}
		}
	}
}

// Deleting a provider throws away credentials that were only ever stored there,
// and adding or editing one can point a library at an attacker's indexer: a
// readonly deployment must offer none of it.
func TestProviderMutatingToolsAreHiddenInReadOnlyMode(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {{Name: "main", URL: srv.URL, APIKey: "k", Default: true}},
	}, config.Permissions{Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny}))
	names := toolNames(t, cs)

	for _, suffix := range append(append([]string{}, providerWriteTools...), providerDestructiveTools...) {
		if has(names, "sonarr"+suffix) {
			t.Errorf("readonly mode must not expose %q", "sonarr"+suffix)
		}
	}
	for _, suffix := range providerReadTools {
		if !has(names, "sonarr"+suffix) {
			t.Errorf("readonly mode must still expose the read tool %q", "sonarr"+suffix)
		}
	}
}

// The tool result is the last place a projection mistake shows up: a download
// client's password must not reach the model even though the field name does.
func TestGetProviderToolReturnsFieldNamesWithoutSecretValues(t *testing.T) {
	srv, _ := fakeArr(t, `{"id":7,"name":"NZBGet","implementation":"Nzbget","enable":true,
	  "protocol":"usenet","priority":1,
	  "fields":[
	    {"name":"host","label":"Host","privacy":"normal","value":"nzbget.invalid"},
	    {"name":"password","label":"Password","privacy":"password","value":"leaked-client-password"}
	  ]}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_get_provider",
		Arguments: map[string]any{"kind": "downloadClient", "id": 7},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	body := contentText(res)
	if !strings.Contains(body, "password") {
		t.Errorf("result does not name the password field: %s", body)
	}
	if strings.Contains(body, "leaked-client-password") {
		t.Fatalf("download client credentials reached the client: %s", body)
	}
	if !strings.Contains(body, "nzbget.invalid") {
		t.Errorf("non-secret field value was dropped: %s", body)
	}
}

// The kind is the one argument a model has to guess, so a wrong guess must come
// back naming the values that would have worked.
func TestGetProviderToolRejectsAnUnknownKind(t *testing.T) {
	srv, _ := fakeArr(t, `{}`)
	cs := connect(t, mediaCfg(srv.URL))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "radarr_get_provider",
		Arguments: map[string]any{"kind": "downloadclients", "id": 7},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("an unknown kind must be an error")
	}
	if !strings.Contains(contentText(res), "importList") {
		t.Errorf("error does not list the valid kinds: %s", contentText(res))
	}
}

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/GauranshMathur/ARR_MCP/pkg/logger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeArr serves a canned JSON body and records which instance was hit.
func fakeArr(t *testing.T, body string) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// connect wires an in-memory MCP client to the server under test.
func connect(t *testing.T, cfg *config.Config) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	s := New(cfg, logger.New("error", "test"))
	ct, st := mcp.NewInMemoryTransports()
	if _, err := s.MCP().Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// toolNames lists the tools the server advertises.
func toolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := make([]string, 0, len(res.Tools))
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func has(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func cfgWith(services map[string][]config.Instance, perms config.Permissions) *config.Config {
	return &config.Config{Permissions: perms, Services: services}
}

var permsFull = config.Permissions{Mode: config.ModeFull, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny}

// The whole point of the rewrite: a real MCP client can complete the handshake.
func TestServerCompletesMCPHandshake(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {{Name: "main", URL: srv.URL, APIKey: "k", Default: true}},
	}, permsFull))

	if names := toolNames(t, cs); len(names) == 0 {
		t.Fatal("server advertised no tools after a successful handshake")
	}
}

func TestOnlyConfiguredServicesExposeTools(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {{Name: "main", URL: srv.URL, APIKey: "k", Default: true}},
	}, permsFull))

	names := toolNames(t, cs)
	if !has(names, "sonarr_list_series") {
		t.Errorf("sonarr tools missing from %v", names)
	}
	for _, unwanted := range []string{"radarr_list_movies", "prowlarr_list_indexers"} {
		if has(names, unwanted) {
			t.Errorf("tool %q exposed for an unconfigured service", unwanted)
		}
	}
}

// The model can only pick an instance if tools/list tells it the valid names.
func TestInstanceArgumentAdvertisesConfiguredNames(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {
			{Name: "main", URL: srv.URL, APIKey: "k", Default: true},
			{Name: "anime", URL: srv.URL, APIKey: "k"},
		},
	}, permsFull))

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var schema string
	for _, tool := range res.Tools {
		if tool.Name == "sonarr_list_series" {
			raw, _ := json.Marshal(tool.InputSchema)
			schema = string(raw)
		}
	}
	if schema == "" {
		t.Fatal("sonarr_list_series not advertised")
	}
	for _, want := range []string{"main", "anime"} {
		if !strings.Contains(schema, want) {
			t.Errorf("input schema does not offer instance %q: %s", want, schema)
		}
	}
}

// Multi-instance is the headline feature: the instance argument must actually
// change which upstream server is contacted.
func TestInstanceArgumentRoutesToSelectedUpstream(t *testing.T) {
	main, mainHits := fakeArr(t, `[{"id":1,"title":"Main Show"}]`)
	anime, animeHits := fakeArr(t, `[{"id":2,"title":"Anime Show"}]`)

	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {
			{Name: "main", URL: main.URL, APIKey: "k", Default: true},
			{Name: "anime", URL: anime.URL, APIKey: "k"},
		},
	}, permsFull))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_list_series",
		Arguments: map[string]any{"instance": "anime"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %+v", res.Content)
	}
	if *animeHits != 1 {
		t.Errorf("anime instance hits = %d, want 1", *animeHits)
	}
	if *mainHits != 0 {
		t.Errorf("main instance hits = %d, want 0", *mainHits)
	}
}

func TestOmittedInstanceUsesConfiguredDefault(t *testing.T) {
	main, mainHits := fakeArr(t, `[]`)
	anime, animeHits := fakeArr(t, `[]`)

	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {
			{Name: "main", URL: main.URL, APIKey: "k"},
			{Name: "anime", URL: anime.URL, APIKey: "k", Default: true},
		},
	}, permsFull))

	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "sonarr_list_series",
	}); err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if *animeHits != 1 || *mainHits != 0 {
		t.Errorf("hits main=%d anime=%d, want main=0 anime=1", *mainHits, *animeHits)
	}
}

// The instance enum makes an invalid name unrepresentable: the SDK rejects it
// during schema validation, before any handler runs, and names the valid set.
func TestUnknownInstanceIsRejectedWithValidNames(t *testing.T) {
	srv, hits := fakeArr(t, `[]`)
	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {{Name: "main", URL: srv.URL, APIKey: "k", Default: true}},
	}, permsFull))

	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_list_series",
		Arguments: map[string]any{"instance": "typo"},
	})
	if err == nil {
		t.Fatal("expected an error for an unknown instance name")
	}
	if !strings.Contains(err.Error(), "main") {
		t.Errorf("error %q does not tell the model the valid instance names", err)
	}
	if *hits != 0 {
		t.Errorf("upstream contacted %d times for an invalid instance", *hits)
	}
}

// Ambiguity is not schema-catchable: the argument is simply absent, so the
// handler must explain which instances it could have meant.
func TestAmbiguousInstanceReturnsCorrectableToolError(t *testing.T) {
	a, _ := fakeArr(t, `[]`)
	b, _ := fakeArr(t, `[]`)
	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {
			{Name: "main", URL: a.URL, APIKey: "k"},
			{Name: "anime", URL: b.URL, APIKey: "k"},
		},
	}, permsFull))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "sonarr_list_series",
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected a tool error when no default instance is configured")
	}
	text := contentText(res)
	for _, want := range []string{"main", "anime"} {
		if !strings.Contains(text, want) {
			t.Errorf("error %q does not mention instance %q", text, want)
		}
	}
}

func TestReadOnlyModeHidesMutatingTools(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {{Name: "main", URL: srv.URL, APIKey: "k", Default: true}},
	}, config.Permissions{Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny}))

	names := toolNames(t, cs)
	if !has(names, "sonarr_list_series") {
		t.Error("readonly mode must still expose read tools")
	}
	if has(names, "sonarr_add_series") {
		t.Error("readonly mode must not expose sonarr_add_series")
	}
}

// A client without elicitation support must not get silent write access.
func TestConfirmModeDeniesWriteWhenClientCannotPrompt(t *testing.T) {
	srv, hits := fakeArr(t, `{}`)
	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {{Name: "main", URL: srv.URL, APIKey: "k", Default: true}},
	}, config.Permissions{Mode: config.ModeConfirm, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny}))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "sonarr_add_series",
		Arguments: map[string]any{"tvdbId": 1, "qualityProfileId": 1, "rootFolderPath": "/tv"},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected the write to be denied without elicitation support")
	}
	if *hits != 0 {
		t.Errorf("upstream was contacted %d times despite denial", *hits)
	}
}

func TestReadToolsAreAnnotatedReadOnly(t *testing.T) {
	srv, _ := fakeArr(t, `[]`)
	cs := connect(t, cfgWith(map[string][]config.Instance{
		"sonarr": {{Name: "main", URL: srv.URL, APIKey: "k", Default: true}},
	}, permsFull))

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range res.Tools {
		if tool.Name != "sonarr_list_series" {
			continue
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("sonarr_list_series is missing ReadOnlyHint: %+v", tool.Annotations)
		}
		return
	}
	t.Fatal("sonarr_list_series not advertised")
}

// contentText flattens a tool result's content for assertions.
func contentText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

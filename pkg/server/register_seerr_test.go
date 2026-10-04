package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// seerrReadTools are the tools a readonly deployment must still advertise.
var seerrReadTools = []string{
	"seerr_system_status",
	"seerr_request_counts",
	"seerr_list_requests",
	"seerr_get_request",
	"seerr_search",
	"seerr_discover",
	"seerr_get_media",
	"seerr_list_users",
	"seerr_list_issues",
	"seerr_get_issue",
}

// seerrWriteTools create or change requests and issues.
var seerrWriteTools = []string{
	"seerr_create_request",
	"seerr_approve_request",
	"seerr_decline_request",
	"seerr_retry_request",
	"seerr_comment_issue",
	"seerr_set_issue_status",
}

// seerrDestructiveTools remove a request.
var seerrDestructiveTools = []string{"seerr_delete_request"}

// seerrMutatingArgs gives each mutating tool arguments that pass its own
// validation, so a denial can only come from the permission gate.
var seerrMutatingArgs = map[string]map[string]any{
	"seerr_create_request":   {"mediaType": "movie", "tmdbId": 550},
	"seerr_approve_request":  {"requestId": 1},
	"seerr_decline_request":  {"requestId": 1},
	"seerr_retry_request":    {"requestId": 1},
	"seerr_comment_issue":    {"issueId": 1, "message": "hi"},
	"seerr_set_issue_status": {"issueId": 1, "status": "resolved"},
	"seerr_delete_request":   {"requestId": 1},
}

func seerrCfg(url string, perms config.Permissions) *config.Config {
	return cfgWith(map[string][]config.Instance{
		"seerr": {{Name: "main", URL: url, APIKey: "seerr-key", Default: true}},
	}, perms)
}

func TestSeerrToolsAreAllAdvertised(t *testing.T) {
	srv, _ := fakeArr(t, `{}`)
	names := toolNames(t, connect(t, seerrCfg(srv.URL, permsFull)))

	var want []string
	want = append(want, seerrReadTools...)
	want = append(want, seerrWriteTools...)
	want = append(want, seerrDestructiveTools...)
	for _, tool := range want {
		if !has(names, tool) {
			t.Errorf("tool %q not advertised", tool)
		}
	}
	seerr := 0
	for _, n := range names {
		if strings.HasPrefix(n, "seerr_") {
			seerr++
		}
	}
	if seerr != 17 || len(want) != 17 {
		t.Errorf("seerr tools = %d (expected set %d), want 17: %v", seerr, len(want), names)
	}
}

func TestSeerrMutatingToolsAreHiddenInReadOnlyMode(t *testing.T) {
	srv, _ := fakeArr(t, `{}`)
	names := toolNames(t, connect(t, seerrCfg(srv.URL, config.Permissions{
		Mode: config.ModeReadOnly, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny,
	})))

	for _, tool := range append(append([]string{}, seerrWriteTools...), seerrDestructiveTools...) {
		if has(names, tool) {
			t.Errorf("readonly mode must not expose %q", tool)
		}
	}
	for _, tool := range seerrReadTools {
		if !has(names, tool) {
			t.Errorf("readonly mode must still expose %q", tool)
		}
	}
}

// Under confirm mode with a client that cannot be asked, every mutating tool
// must refuse before it reaches Seerr.
func TestSeerrMutatingToolsAreDeniedWithoutConfirmation(t *testing.T) {
	srv, hits := fakeArr(t, `{}`)
	cs := connect(t, seerrCfg(srv.URL, config.Permissions{
		Mode: config.ModeConfirm, ConfirmScope: config.ScopeWrite, Fallback: config.FallbackDeny,
	}))

	for _, tool := range append(append([]string{}, seerrWriteTools...), seerrDestructiveTools...) {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: seerrMutatingArgs[tool]})
		if err != nil {
			t.Fatalf("%s: transport error: %v", tool, err)
		}
		if !res.IsError {
			t.Errorf("%s ran without confirmation: %s", tool, contentText(res))
		}
	}
	if *hits != 0 {
		t.Errorf("upstream was contacted %d times despite denial", *hits)
	}
}

func TestSeerrDeleteRequestIsAnnotatedDestructive(t *testing.T) {
	srv, _ := fakeArr(t, `{}`)
	cs := connect(t, seerrCfg(srv.URL, permsFull))

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	seen := false
	for _, tool := range res.Tools {
		if tool.Name != "seerr_delete_request" {
			continue
		}
		seen = true
		if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
			t.Errorf("annotations = %+v, want destructive", tool.Annotations)
		}
	}
	if !seen {
		t.Error("seerr_delete_request not advertised")
	}
}

// seerrRecorder answers every request with body and keeps what it saw.
func seerrRecorder(t *testing.T, status int, body string) (*httptest.Server, *[]*http.Request, *[]string) {
	t.Helper()
	var reqs []*http.Request
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent, _ := io.ReadAll(r.Body)
		reqs = append(reqs, r)
		bodies = append(bodies, string(sent))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs, &bodies
}

func callSeerr(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: transport error: %v", tool, err)
	}
	if res.IsError {
		t.Fatalf("%s returned an error: %s", tool, contentText(res))
	}
	return contentText(res)
}

// The listing tool must authenticate, send its filters upstream, and wrap the
// trimmed requests with a count.
func TestSeerrListRequestsToolEndToEnd(t *testing.T) {
	srv, reqs, _ := seerrRecorder(t, 200, `{"pageInfo":{"pages":1,"pageSize":20,"results":1,"page":1},"results":[
	  {"id":9,"status":1,"type":"movie","media":{"tmdbId":550,"status":2},"requestedBy":{"id":13,"email":"s@example.com","displayName":"soumil"}}]}`)
	cs := connect(t, seerrCfg(srv.URL, permsFull))

	out := callSeerr(t, cs, "seerr_list_requests", map[string]any{"filter": "pending", "requestedBy": 13})

	r := (*reqs)[0]
	if r.URL.Path != "/api/v1/request" || r.URL.Query().Get("filter") != "pending" || r.URL.Query().Get("requestedBy") != "13" {
		t.Errorf("upstream request = %s", r.URL.String())
	}
	if r.Header.Get("X-Api-Key") != "seerr-key" {
		t.Errorf("X-Api-Key = %q", r.Header.Get("X-Api-Key"))
	}
	if !strings.Contains(out, `"status":"PENDING"`) || !strings.Contains(out, `"count":1`) || !strings.Contains(out, `"tmdbId":550`) {
		t.Errorf("result = %s", out)
	}
	if strings.Contains(out, "example.com") {
		t.Errorf("email reached the client: %s", out)
	}
}

func TestSeerrCreateRequestToolSendsTMDBIDAsMediaID(t *testing.T) {
	srv, reqs, bodies := seerrRecorder(t, 201, `{"id":12,"status":2,"type":"tv","media":{"tmdbId":1398,"status":3}}`)
	cs := connect(t, seerrCfg(srv.URL, permsFull))

	out := callSeerr(t, cs, "seerr_create_request", map[string]any{
		"mediaType": "tv", "tmdbId": 1398, "seasons": []int{1, 2}, "userId": 13,
	})

	if (*reqs)[0].Method != "POST" || (*reqs)[0].URL.Path != "/api/v1/request" {
		t.Errorf("upstream request = %s %s", (*reqs)[0].Method, (*reqs)[0].URL.Path)
	}
	if got := (*bodies)[0]; got != `{"mediaId":1398,"mediaType":"tv","seasons":[1,2],"userId":13}` {
		t.Errorf("body = %s", got)
	}
	if !strings.Contains(out, `"status":"APPROVED"`) {
		t.Errorf("result = %s", out)
	}
}

func TestSeerrApproveAndDeclineHitTheirOwnPaths(t *testing.T) {
	srv, reqs, _ := seerrRecorder(t, 200, `{"id":4,"status":2,"type":"movie"}`)
	cs := connect(t, seerrCfg(srv.URL, permsFull))

	callSeerr(t, cs, "seerr_approve_request", map[string]any{"requestId": 4})
	callSeerr(t, cs, "seerr_decline_request", map[string]any{"requestId": 4})
	callSeerr(t, cs, "seerr_retry_request", map[string]any{"requestId": 4})

	var got []string
	for _, r := range *reqs {
		got = append(got, r.Method+" "+r.URL.Path)
	}
	want := "POST /api/v1/request/4/approve,POST /api/v1/request/4/decline,POST /api/v1/request/4/retry"
	if strings.Join(got, ",") != want {
		t.Errorf("upstream calls = %v, want %s", got, want)
	}
}

func TestSeerrDeleteRequestToolReportsDeletion(t *testing.T) {
	srv, reqs, _ := seerrRecorder(t, 204, ``)
	cs := connect(t, seerrCfg(srv.URL, permsFull))

	out := callSeerr(t, cs, "seerr_delete_request", map[string]any{"requestId": 7})

	if (*reqs)[0].Method != "DELETE" || (*reqs)[0].URL.Path != "/api/v1/request/7" {
		t.Errorf("upstream request = %s %s", (*reqs)[0].Method, (*reqs)[0].URL.Path)
	}
	if !strings.Contains(out, `"id":7`) || !strings.Contains(out, `"deleted":true`) {
		t.Errorf("result = %s", out)
	}
}

// A failed upstream call must come back as a tool error naming the instance,
// not as a success with an empty body.
func TestSeerrToolErrorsNameTheInstance(t *testing.T) {
	srv, _, _ := seerrRecorder(t, 403, `{"message":"You do not have permission to access this endpoint"}`)
	cs := connect(t, seerrCfg(srv.URL, permsFull))

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "seerr_get_request", Arguments: map[string]any{"requestId": 1}})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	text := contentText(res)
	if !res.IsError || !strings.Contains(text, `seerr instance "main"`) || !strings.Contains(text, "403") {
		t.Errorf("result = %v %s", res.IsError, text)
	}
}

func TestSeerrSearchAndMediaTools(t *testing.T) {
	srv, reqs, _ := seerrRecorder(t, 200, `{"page":1,"totalPages":1,"totalResults":1,"results":[{"id":550,"mediaType":"movie","title":"Fight Club"}],"id":550,"title":"Fight Club"}`)
	cs := connect(t, seerrCfg(srv.URL, permsFull))

	callSeerr(t, cs, "seerr_search", map[string]any{"query": "fight club"})
	callSeerr(t, cs, "seerr_discover", map[string]any{"kind": "trending"})
	callSeerr(t, cs, "seerr_get_media", map[string]any{"mediaType": "movie", "tmdbId": 550})
	callSeerr(t, cs, "seerr_list_users", map[string]any{})
	callSeerr(t, cs, "seerr_list_issues", map[string]any{"filter": "all"})
	callSeerr(t, cs, "seerr_get_issue", map[string]any{"issueId": 1})
	callSeerr(t, cs, "seerr_comment_issue", map[string]any{"issueId": 1, "message": "hi"})
	callSeerr(t, cs, "seerr_set_issue_status", map[string]any{"issueId": 1, "status": "open"})

	var got []string
	for _, r := range *reqs {
		got = append(got, r.Method+" "+r.URL.Path)
	}
	want := []string{
		"GET /api/v1/search", "GET /api/v1/discover/trending", "GET /api/v1/movie/550", "GET /api/v1/user",
		"GET /api/v1/issue", "GET /api/v1/issue/1", "POST /api/v1/issue/1/comment", "POST /api/v1/issue/1/open",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("upstream calls = %v\nwant %v", got, want)
	}
}

func TestSeerrSystemStatusAndCountsTools(t *testing.T) {
	srv, reqs, _ := seerrRecorder(t, 200, `{"version":"3.4.1","totalRequests":290,"total":290,"pending":0}`)
	cs := connect(t, seerrCfg(srv.URL, permsFull))

	out := callSeerr(t, cs, "seerr_system_status", map[string]any{})
	if !strings.Contains(out, `"version":"3.4.1"`) || !strings.Contains(out, `"totalRequests":290`) {
		t.Errorf("status = %s", out)
	}
	out = callSeerr(t, cs, "seerr_request_counts", map[string]any{})
	if !strings.Contains(out, `"total":290`) {
		t.Errorf("counts = %s", out)
	}
	last := (*reqs)[len(*reqs)-1]
	if last.URL.Path != "/api/v1/request/count" {
		t.Errorf("last upstream path = %s", last.URL.Path)
	}
}

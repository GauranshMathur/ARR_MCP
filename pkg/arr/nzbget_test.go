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

// nzbCreds are throwaway credentials for the fake NZBGet.
var nzbCreds = Credentials{Username: "nzb", Password: "pw"}

// fakeNZBGet serves a JSON-RPC endpoint that answers each method with the
// canned result in results, wrapped in NZBGet's envelope. Methods not in the
// map get the "Invalid procedure" error the real server returns. It records
// the decoded request envelopes in order.
func fakeNZBGet(t *testing.T, results map[string]string) (*httptest.Server, *[]rpcRequest) {
	t.Helper()
	var calls []rpcRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent, _ := io.ReadAll(r.Body)
		var req rpcRequest
		if err := json.Unmarshal(sent, &req); err != nil {
			t.Errorf("request body is not a JSON-RPC envelope: %v: %s", err, sent)
		}
		calls = append(calls, req)
		w.Header().Set("Content-Type", "application/json")
		result, ok := results[req.Method]
		if !ok {
			_, _ = w.Write([]byte(`{"version":"1.1","id":1,"error":{"name":"JSONRPCError","code":1,"message":"Invalid procedure"},"result":null}`))
			return
		}
		_, _ = w.Write([]byte(`{"version":"1.1","id":1,"result":` + result + `}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestNZBCallSendsJSONRPCEnvelopeWithBasicAuth(t *testing.T) {
	srv, got := fakeService(t, 200, `{"version":"1.1","id":1,"result":"26.3"}`)
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	version, err := NZBGetVersion(context.Background(), c)
	if err != nil {
		t.Fatalf("NZBGetVersion returned error: %v", err)
	}
	if version != "26.3" {
		t.Errorf("version = %q, want 26.3", version)
	}
	if got.method != http.MethodPost || got.path != "/jsonrpc" {
		t.Errorf("request = %s %s, want POST /jsonrpc", got.method, got.path)
	}
	// NZBGet rejects a null params, so the envelope must carry an empty array.
	var env map[string]json.RawMessage
	if err := json.Unmarshal([]byte(got.body), &env); err != nil {
		t.Fatalf("body is not JSON: %v: %s", err, got.body)
	}
	if string(env["method"]) != `"version"` {
		t.Errorf("method = %s, want \"version\"", env["method"])
	}
	if string(env["params"]) != `[]` {
		t.Errorf("params = %s, want []", env["params"])
	}
	if _, ok := env["id"]; !ok {
		t.Errorf("envelope has no id: %s", got.body)
	}
	user, pass, ok := parseBasic(got.header.Get("Authorization"))
	if !ok || user != "nzb" || pass != "pw" {
		t.Errorf("basic auth = (%q,%q,%v), want (nzb,pw,true)", user, pass, ok)
	}
}

func TestNZBCallReturnsRPCErrorMessage(t *testing.T) {
	srv, _ := fakeNZBGet(t, nil)
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	_, err := NZBGetVersion(context.Background(), c)
	if err == nil {
		t.Fatal("expected an error from a JSON-RPC error object, got nil")
	}
	for _, want := range []string{"version", "Invalid procedure", "code 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// status reports rates in bytes per second while rate() takes KiB/s, so the
// projection has to convert or the model will set limits a thousand times off.
func TestNZBGetStatusProjectsTypedResult(t *testing.T) {
	srv, calls := fakeNZBGet(t, map[string]string{
		"version": `"26.3"`,
		"status": `{"RemainingSizeMB":1668,"DownloadedSizeMB":160755,"FreeDiskSpaceMB":10848998,
		  "DownloadRateLo":2097152,"DownloadRateHi":0,"DownloadLimit":1048576,
		  "ThreadCount":10,"PostJobCount":1,"UpTimeSec":119711,"DownloadTimeSec":4946,
		  "ServerStandBy":true,"DownloadPaused":true,"PostPaused":false,"ScanPaused":false}`,
	})
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	st, err := NZBGetStatus(context.Background(), c)
	if err != nil {
		t.Fatalf("NZBGetStatus returned error: %v", err)
	}
	want := NZBStatus{
		Version: "26.3", DownloadRateKB: 2048, DownloadLimitKB: 1024,
		RemainingSizeMB: 1668, DownloadedSizeMB: 160755, FreeDiskSpaceMB: 10848998,
		DownloadPaused: true, ServerStandBy: true,
		ThreadCount: 10, PostJobCount: 1, UpTimeSec: 119711, DownloadTimeSec: 4946,
	}
	if st != want {
		t.Errorf("status = %+v, want %+v", st, want)
	}
	if len(*calls) != 2 {
		t.Errorf("calls = %d, want status and version", len(*calls))
	}
}

func TestNZBGetListGroupsProjectsHealthAndPriority(t *testing.T) {
	srv, _ := fakeNZBGet(t, map[string]string{
		"listgroups": `[{"NZBID":15803,"NZBName":"Some.Movie.2011","Status":"PAUSED","Category":"Movies",
		  "FileSizeMB":34309,"RemainingSizeMB":1668,"PausedSizeMB":1668,"DownloadedSizeMB":32641,
		  "Health":950,"CriticalHealth":948,"ActiveDownloads":0,"MaxPriority":50,"MinPriority":50,
		  "DestDir":"/downloads/intermediate/Some.Movie.2011.#15803","FileCount":139}]`,
	})
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	groups, err := NZBGetListGroups(context.Background(), c)
	if err != nil {
		t.Fatalf("NZBGetListGroups returned error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	want := NZBGroup{
		ID: 15803, Name: "Some.Movie.2011", Status: "PAUSED", Category: "Movies",
		SizeMB: 34309, RemainingMB: 1668, PausedMB: 1668, DownloadedMB: 32641,
		HealthPercent: 95, ActiveDownloads: 0, Priority: 50,
		DestDir: "/downloads/intermediate/Some.Movie.2011.#15803",
	}
	if groups[0] != want {
		t.Errorf("group = %+v, want %+v", groups[0], want)
	}
}

func TestNZBGetHistoryPassesHiddenAndTruncatesToLimit(t *testing.T) {
	srv, calls := fakeNZBGet(t, map[string]string{
		"history": `[
		  {"NZBID":3,"Name":"c","Kind":"NZB","Status":"SUCCESS/UNPACK","Category":"Series","FileSizeMB":3270,
		   "HistoryTime":1787880087,"DestDir":"/done/c","FinalDir":"","Health":1000,"DupeKey":"","DeleteStatus":"NONE","MarkStatus":"NONE"},
		  {"NZBID":2,"Name":"b","Kind":"NZB","Status":"FAILURE/PAR","Health":600,"DeleteStatus":"HEALTH","MarkStatus":"BAD"},
		  {"NZBID":1,"Name":"a","Kind":"DUP","Status":"DELETED/DUPE"}]`,
	})
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	items, err := NZBGetHistory(context.Background(), c, true, 2)
	if err != nil {
		t.Fatalf("NZBGetHistory returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2 (truncated)", len(items))
	}
	want := NZBHistoryItem{
		ID: 3, Name: "c", Kind: "NZB", Status: "SUCCESS/UNPACK", Category: "Series", SizeMB: 3270,
		Time: "2026-08-28T01:21:27Z", DestDir: "/done/c", HealthPercent: 100,
		DeleteStatus: "NONE", MarkStatus: "NONE",
	}
	if items[0] != want {
		t.Errorf("item = %+v, want %+v", items[0], want)
	}
	if items[1].HealthPercent != 60 || items[1].MarkStatus != "BAD" {
		t.Errorf("second item = %+v, want health 60 marked BAD", items[1])
	}
	if got := (*calls)[0].Params; len(got) != 1 || got[0] != true {
		t.Errorf("history params = %v, want [true]", got)
	}
}

func TestNZBGetHistoryWithoutLimitReturnsEverything(t *testing.T) {
	srv, _ := fakeNZBGet(t, map[string]string{"history": `[{"NZBID":1},{"NZBID":2},{"NZBID":3}]`})
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	items, err := NZBGetHistory(context.Background(), c, false, 0)
	if err != nil {
		t.Fatalf("NZBGetHistory returned error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("items = %d, want 3", len(items))
	}
}

// append is positional: the URL or base64 content goes in the second slot and
// PPParameters must be an empty array, not omitted.
func TestNZBGetAppendSendsPositionalParams(t *testing.T) {
	srv, calls := fakeNZBGet(t, map[string]string{"append": `42`})
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	id, err := NZBGetAppend(context.Background(), c, AppendNZBRequest{
		URL: "https://indexer.example/get/1", Category: "Movies", Priority: 50,
		AddToTop: true, AddPaused: false, DupeKey: "tt0001", DupeScore: 10,
	})
	if err != nil {
		t.Fatalf("NZBGetAppend returned error: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
	if (*calls)[0].Method != "append" {
		t.Fatalf("method = %q, want append", (*calls)[0].Method)
	}
	got := (*calls)[0].Params
	want := []any{"", "https://indexer.example/get/1", "Movies", float64(50), true, false, "tt0001", float64(10), "SCORE", []any{}}
	if len(got) != len(want) {
		t.Fatalf("params = %v (%d), want %v (%d)", got, len(got), want, len(want))
	}
	for i := range want {
		if i == len(want)-1 {
			if arr, ok := got[i].([]any); !ok || len(arr) != 0 {
				t.Errorf("params[%d] = %v, want empty PPParameters array", i, got[i])
			}
			continue
		}
		if got[i] != want[i] {
			t.Errorf("params[%d] = %v (%T), want %v (%T)", i, got[i], got[i], want[i], want[i])
		}
	}
}

func TestNZBGetAppendPrefersContentAndFilename(t *testing.T) {
	srv, calls := fakeNZBGet(t, map[string]string{"append": `7`})
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	if _, err := NZBGetAppend(context.Background(), c, AppendNZBRequest{
		Filename: "show.nzb", Content: "PD94bWw=", DupeMode: "ALL",
	}); err != nil {
		t.Fatalf("NZBGetAppend returned error: %v", err)
	}
	got := (*calls)[0].Params
	if got[0] != "show.nzb" || got[1] != "PD94bWw=" || got[8] != "ALL" {
		t.Errorf("params = %v, want filename, content and dupe mode passed through", got)
	}
}

func TestNZBGetAppendRejectsZeroResult(t *testing.T) {
	srv, _ := fakeNZBGet(t, map[string]string{"append": `0`})
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	if _, err := NZBGetAppend(context.Background(), c, AppendNZBRequest{URL: "https://x/1"}); err == nil {
		t.Fatal("expected an error when append returns 0, got nil")
	}
}

func TestNZBGetAppendRequiresExactlyOneSource(t *testing.T) {
	srv, calls := fakeNZBGet(t, map[string]string{"append": `1`})
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	for _, req := range []AppendNZBRequest{{}, {URL: "https://x/1", Content: "abc"}} {
		if _, err := NZBGetAppend(context.Background(), c, req); err == nil {
			t.Errorf("request %+v: expected an error, got nil", req)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("upstream contacted %d times for invalid requests", len(*calls))
	}
}

func TestNZBGetEditQueueSendsCommandParamAndIDs(t *testing.T) {
	srv, calls := fakeNZBGet(t, map[string]string{"editqueue": `true`})
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	if err := NZBGetEditQueue(context.Background(), c, "GroupSetPriority", "50", []int{7, 9}); err != nil {
		t.Fatalf("NZBGetEditQueue returned error: %v", err)
	}
	got := (*calls)[0]
	if got.Method != "editqueue" || len(got.Params) != 3 {
		t.Fatalf("call = %+v, want editqueue with 3 params", got)
	}
	if got.Params[0] != "GroupSetPriority" || got.Params[1] != "50" {
		t.Errorf("params = %v, want command and param first", got.Params)
	}
	ids, _ := got.Params[2].([]any)
	if len(ids) != 2 || ids[0] != float64(7) || ids[1] != float64(9) {
		t.Errorf("ids = %v, want [7 9]", got.Params[2])
	}
}

func TestNZBGetEditQueueFalseIsAnError(t *testing.T) {
	srv, _ := fakeNZBGet(t, map[string]string{"editqueue": `false`})
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	err := NZBGetEditQueue(context.Background(), c, "GroupPause", "", []int{7})
	if err == nil {
		t.Fatal("expected an error when editqueue returns false, got nil")
	}
	for _, want := range []string{"GroupPause", "7"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestNZBGetSetPausedPicksMethodByScope(t *testing.T) {
	cases := []struct {
		scope  string
		paused bool
		want   string
	}{
		{"download", true, "pausedownload"},
		{"download", false, "resumedownload"},
		{"post", true, "pausepost"},
		{"post", false, "resumepost"},
		{"scan", true, "pausescan"},
		{"scan", false, "resumescan"},
	}
	for _, tc := range cases {
		srv, calls := fakeNZBGet(t, map[string]string{tc.want: `true`})
		c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

		if err := NZBGetSetPaused(context.Background(), c, tc.scope, tc.paused); err != nil {
			t.Errorf("%s paused=%v: %v", tc.scope, tc.paused, err)
			continue
		}
		if got := (*calls)[0].Method; got != tc.want {
			t.Errorf("%s paused=%v: method = %q, want %q", tc.scope, tc.paused, got, tc.want)
		}
	}
}

func TestNZBGetSetPausedRejectsUnknownScope(t *testing.T) {
	srv, calls := fakeNZBGet(t, nil)
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	err := NZBGetSetPaused(context.Background(), c, "everything", true)
	if err == nil {
		t.Fatal("expected an error for an unknown scope, got nil")
	}
	if !strings.Contains(err.Error(), "download") {
		t.Errorf("error %q does not list the valid scopes", err)
	}
	if len(*calls) != 0 {
		t.Errorf("upstream contacted %d times for an invalid scope", len(*calls))
	}
}

func TestNZBGetRateSendsLimitAndRejectsFalse(t *testing.T) {
	srv, calls := fakeNZBGet(t, map[string]string{"rate": `true`})
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	if err := NZBGetRate(context.Background(), c, 2048); err != nil {
		t.Fatalf("NZBGetRate returned error: %v", err)
	}
	if got := (*calls)[0]; got.Method != "rate" || len(got.Params) != 1 || got.Params[0] != float64(2048) {
		t.Errorf("call = %+v, want rate [2048]", got)
	}

	failing, _ := fakeNZBGet(t, map[string]string{"rate": `false`})
	if err := NZBGetRate(context.Background(), NewClient(failing.URL, NZBGetSpec, nzbCreds), 5); err == nil {
		t.Error("expected an error when rate returns false, got nil")
	}
	if err := NZBGetRate(context.Background(), c, -1); err == nil {
		t.Error("expected an error for a negative limit, got nil")
	}
}

func TestNZBGetScanCallsScan(t *testing.T) {
	srv, calls := fakeNZBGet(t, map[string]string{"scan": `true`})
	c := NewClient(srv.URL, NZBGetSpec, nzbCreds)

	if err := NZBGetScan(context.Background(), c); err != nil {
		t.Fatalf("NZBGetScan returned error: %v", err)
	}
	if got := (*calls)[0].Method; got != "scan" {
		t.Errorf("method = %q, want scan", got)
	}
}

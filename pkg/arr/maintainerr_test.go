package arr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// maintainerrRoutes serves a fixed body per "METHOD /path" and records every
// request, for calls that need more than one upstream response.
func maintainerrRoutes(t *testing.T, routes map[string]string) (*Client, *[]string, *[]string) {
	t.Helper()
	var paths, bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		paths = append(paths, key)
		sent, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(sent))
		body, ok := routes[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, MaintainerrSpec, Credentials{}), &paths, &bodies
}

// Maintainerr has no authentication, so nothing that looks like a credential
// should go on the wire.
func TestMaintainerrSendsNoCredentials(t *testing.T) {
	srv, got := fakeService(t, 200, `{"status":1,"version":"3.27.0"}`)
	c := NewClient(srv.URL, MaintainerrSpec, Credentials{})

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if got.path != "/api/app/status" {
		t.Errorf("status path = %q, want /api/app/status", got.path)
	}
	for _, h := range []string{"X-Api-Key", "Authorization"} {
		if v := got.header.Get(h); v != "" {
			t.Errorf("%s = %q, want unset", h, v)
		}
	}
}

func TestMaintainerrStatusDecodes(t *testing.T) {
	srv, _ := fakeService(t, 200,
		`{"status":1,"version":"3.27.0","commitTag":"latest-a26fee9","updateAvailable":true}`)
	c := NewClient(srv.URL, MaintainerrSpec, Credentials{})

	st, err := MaintainerrGetStatus(context.Background(), c)
	if err != nil {
		t.Fatalf("MaintainerrGetStatus: %v", err)
	}
	if st.Version != "3.27.0" || !st.UpdateAvailable || st.CommitTag != "latest-a26fee9" {
		t.Errorf("status = %+v", st)
	}
}

// arrAction is a bare enum index upstream; a model reading "0" cannot tell that
// it means files are deleted from disk.
func TestMaintainerrCollectionsNameTheirArrAction(t *testing.T) {
	c, paths, _ := maintainerrRoutes(t, map[string]string{
		"GET /api/collections": `[
		  {"id":1,"title":"Movies","type":"movie","libraryId":"lib1","isActive":true,
		   "arrAction":0,"deleteAfterDays":14,"mediaCount":3,"handledMediaAmount":11,
		   "handledMediaSizeBytes":610453712291,"overlayEnabled":true,"media":[{"id":9}]},
		  {"id":2,"title":"Shows","type":"show","arrAction":3,"deleteAfterDays":null}]`,
	})

	cols, err := MaintainerrListCollections(context.Background(), c)
	if err != nil {
		t.Fatalf("MaintainerrListCollections: %v", err)
	}
	if (*paths)[0] != "GET /api/collections" {
		t.Errorf("request = %v", *paths)
	}
	if len(cols) != 2 {
		t.Fatalf("collections = %d, want 2", len(cols))
	}
	if cols[0].ArrAction != "DELETE" || cols[1].ArrAction != "UNMONITOR" {
		t.Errorf("arrAction = %q, %q; want DELETE, UNMONITOR", cols[0].ArrAction, cols[1].ArrAction)
	}
	if cols[0].DeleteAfterDays == nil || *cols[0].DeleteAfterDays != 14 || cols[1].DeleteAfterDays != nil {
		t.Errorf("deleteAfterDays = %v, %v", cols[0].DeleteAfterDays, cols[1].DeleteAfterDays)
	}
	if cols[0].MediaCount != 3 || cols[0].HandledMediaAmount != 11 {
		t.Errorf("collection = %+v", cols[0])
	}
}

// The media page does not carry the collection's grace period, so the call
// fetches the collection too; without it no one can tell when an item goes.
func TestMaintainerrCollectionMediaCarriesTheGracePeriod(t *testing.T) {
	c, paths, _ := maintainerrRoutes(t, map[string]string{
		"GET /api/collections/collection/1": `{"id":1,"title":"Movies","deleteAfterDays":14}`,
		"GET /api/collections/media/1/content/2": `{"totalSize":30,"items":[
		  {"id":5,"collectionId":1,"mediaServerId":"abc","tmdbId":603,"tvdbId":null,
		   "addDate":"2026-09-20T08:00:00.000Z","isManual":false,"sizeBytes":1000,
		   "mediaData":{"id":"abc","title":"The Matrix","type":"movie","year":1999,
		     "summary":"A very long overview","actors":[{"name":"Keanu"}],
		     "providerIds":{"tmdb":["603"]}}}]}`,
	})

	page, err := MaintainerrCollectionMedia(context.Background(), c, 1, 2, 10)
	if err != nil {
		t.Fatalf("MaintainerrCollectionMedia: %v", err)
	}
	if page.DeleteAfterDays == nil || *page.DeleteAfterDays != 14 || page.Total != 30 || page.CollectionTitle != "Movies" {
		t.Errorf("page = %+v", page)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(page.Items))
	}
	item := page.Items[0]
	if item.MediaServerID != "abc" || item.Title != "The Matrix" || item.Year != 1999 ||
		item.TMDBID != 603 || item.AddDate != "2026-09-20T08:00:00.000Z" {
		t.Errorf("item = %+v", item)
	}
	encoded, _ := json.Marshal(page)
	if strings.Contains(string(encoded), "overview") || strings.Contains(string(encoded), "Keanu") {
		t.Errorf("untrimmed metadata leaked into the page: %s", encoded)
	}
	if got := strings.Join(*paths, ","); !strings.Contains(got, "/content/2") {
		t.Errorf("requests = %v, want page 2 requested", *paths)
	}
}

// Rule groups embed notification agents whose options hold webhook URLs with
// their tokens. Nothing from them may reach a tool result.
func TestMaintainerrRulesDropNotificationSecrets(t *testing.T) {
	rule := `{"id":1,"name":"Movies","description":"","libraryId":"lib1","collectionId":1,
	  "isActive":true,"dataType":"movie","useRules":true,"ruleHandlerCronSchedule":null,
	  "rules":[{"id":96,"ruleJson":"{\"action\":4}","section":0,"isActive":true}],
	  "notifications":[{"id":1,"name":"Discord","agent":"discord",
	    "options":{"webhookUrl":"https://discord.com/api/webhooks/1/leaked-token"}}],
	  "collection":{"id":1,"title":"Movies","arrAction":0,"deleteAfterDays":14}}`
	c, _, _ := maintainerrRoutes(t, map[string]string{
		"GET /api/rules":              "[" + rule + "]",
		"GET /api/rules/1":            rule,
		"POST /api/rules/yaml/encode": `{"code":1,"result":"rules: []"}`,
	})

	list, err := MaintainerrListRules(context.Background(), c)
	if err != nil {
		t.Fatalf("MaintainerrListRules: %v", err)
	}
	detail, err := MaintainerrGetRule(context.Background(), c, 1)
	if err != nil {
		t.Fatalf("MaintainerrGetRule: %v", err)
	}
	for name, v := range map[string]any{"list": list, "detail": detail} {
		encoded, _ := json.Marshal(v)
		if strings.Contains(string(encoded), "leaked-token") || strings.Contains(string(encoded), "webhook") {
			t.Errorf("%s leaked notification options: %s", name, encoded)
		}
	}
	if len(list) != 1 || list[0].RuleCount != 1 || list[0].CollectionID != 1 {
		t.Errorf("list = %+v", list)
	}
	if len(detail.Rules) != 1 || detail.Rules[0].RuleJSON != `{"action":4}` || detail.ArrAction != "DELETE" {
		t.Errorf("detail = %+v", detail)
	}
}

// Omitting collectionId is how Maintainerr expresses a global exclusion, so a
// zero must not be sent as collection 0.
func TestMaintainerrAddExclusionOmitsCollectionForGlobal(t *testing.T) {
	c, paths, bodies := maintainerrRoutes(t, map[string]string{
		"POST /api/rules/exclusion": `{"code":1,"result":"Success"}`,
	})

	if err := MaintainerrAddExclusion(context.Background(), c, "abc", 0); err != nil {
		t.Fatalf("MaintainerrAddExclusion: %v", err)
	}
	if (*paths)[0] != "POST /api/rules/exclusion" {
		t.Errorf("request = %v", *paths)
	}
	if (*bodies)[0] != `{"mediaId":"abc"}` {
		t.Errorf("body = %s, want only mediaId", (*bodies)[0])
	}

	if err := MaintainerrAddExclusion(context.Background(), c, "abc", 2); err != nil {
		t.Fatalf("MaintainerrAddExclusion: %v", err)
	}
	if (*bodies)[1] != `{"mediaId":"abc","collectionId":2}` {
		t.Errorf("body = %s, want mediaId and collectionId", (*bodies)[1])
	}
}

// Maintainerr answers a failed exclusion with 201 and code 0. Trusting the
// status would tell the user an item is protected when it is not.
func TestMaintainerrExclusionFailureInBodyIsAnError(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, map[string]string{
		"POST /api/rules/exclusion":     `{"code":0,"result":"Failed - no metadata"}`,
		"DELETE /api/rules/exclusion/7": `{"code":0,"result":"Failed"}`,
	})

	err := MaintainerrAddExclusion(context.Background(), c, "missing", 0)
	if err == nil || !strings.Contains(err.Error(), "Failed - no metadata") {
		t.Errorf("add error = %v, want the upstream failure", err)
	}
	if err := MaintainerrRemoveExclusion(context.Background(), c, 7); err == nil {
		t.Error("remove with code 0 returned no error")
	}
}

func TestMaintainerrExecuteRulesTargetsOneOrAll(t *testing.T) {
	c, paths, _ := maintainerrRoutes(t, map[string]string{
		"POST /api/rules/execute":   ``,
		"POST /api/rules/3/execute": ``,
	})

	if err := MaintainerrExecuteRules(context.Background(), c, 0); err != nil {
		t.Fatalf("execute all: %v", err)
	}
	if err := MaintainerrExecuteRules(context.Background(), c, 3); err != nil {
		t.Fatalf("execute one: %v", err)
	}
	if got := strings.Join(*paths, ","); got != "POST /api/rules/execute,POST /api/rules/3/execute" {
		t.Errorf("requests = %s", got)
	}
}

// Omitting days resets the full grace window; sending 0 would be rejected.
func TestMaintainerrPostponeOmitsDaysToReset(t *testing.T) {
	c, _, bodies := maintainerrRoutes(t, map[string]string{
		"POST /api/collections/media/postpone": `{"collectionId":1,"mediaServerId":"abc",
		  "addDate":"2026-09-28T00:00:00.000Z","deleteAfterDays":14,
		  "deletionDate":"2026-10-12T00:00:00.000Z"}`,
	})

	res, err := MaintainerrPostpone(context.Background(), c, 1, "abc", 0)
	if err != nil {
		t.Fatalf("MaintainerrPostpone: %v", err)
	}
	if (*bodies)[0] != `{"collectionId":1,"mediaId":"abc"}` {
		t.Errorf("body = %s, want no days", (*bodies)[0])
	}
	if res.DeletionDate != "2026-10-12T00:00:00.000Z" {
		t.Errorf("result = %+v", res)
	}

	if _, err := MaintainerrPostpone(context.Background(), c, 1, "abc", 30); err != nil {
		t.Fatalf("MaintainerrPostpone: %v", err)
	}
	if (*bodies)[1] != `{"collectionId":1,"mediaId":"abc","days":30}` {
		t.Errorf("body = %s, want days 30", (*bodies)[1])
	}
}

func TestMaintainerrReadsRuleAndOverlayStatus(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, map[string]string{
		"GET /api/rules/execute/status": `{"processingQueue":true,"executingRuleGroupId":2,
		  "pendingRuleGroupIds":[3],"queue":[]}`,
		"GET /api/overlays/status":     `{"status":"idle","lastRun":null,"lastResult":null}`,
		"POST /api/overlays/process/1": `{"processed":4,"reverted":0,"skipped":1,"errors":0}`,
		"GET /api/media-server/meta/abc/maintainerr-status": `{"excludedFrom":[{"label":"Global"}],
		  "manuallyAddedTo":[]}`,
		"GET /api/rules/exclusion": `[{"id":7,"mediaServerId":"abc","ruleGroupId":null,
		  "parent":null,"type":"movie"}]`,
	})
	ctx := context.Background()

	rs, err := MaintainerrRuleExecutionStatus(ctx, c)
	if err != nil || !rs.ProcessingQueue || rs.ExecutingRuleGroupID == nil || *rs.ExecutingRuleGroupID != 2 {
		t.Errorf("rule status = %+v, %v", rs, err)
	}
	ov, err := MaintainerrOverlayStatus(ctx, c)
	if err != nil || ov.Status != "idle" || ov.LastRun != nil {
		t.Errorf("overlay status = %+v, %v", ov, err)
	}
	run, err := MaintainerrProcessOverlays(ctx, c, 1)
	if err != nil || run.Processed != 4 || run.Skipped != 1 {
		t.Errorf("overlay run = %+v, %v", run, err)
	}
	ms, err := MaintainerrMediaStatus(ctx, c, "abc")
	if err != nil || len(ms.ExcludedFrom) != 1 || ms.ExcludedFrom[0].Label != "Global" {
		t.Errorf("media status = %+v, %v", ms, err)
	}
	ex, err := MaintainerrListExclusions(ctx, c, 0, "")
	if err != nil || len(ex) != 1 || ex[0].ID != 7 || ex[0].RuleGroupID != nil {
		t.Errorf("exclusions = %+v, %v", ex, err)
	}
}

// Maintainerr answers an unknown id with 200 and an empty body. A decode error
// would hide that; the caller needs to hear the id does not exist.
func TestMaintainerrUnknownIDIsNamedNotFound(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, map[string]string{
		"GET /api/rules/99":                  ``,
		"GET /api/collections/collection/99": ``,
	})

	if _, err := MaintainerrGetRule(context.Background(), c, 99); err == nil ||
		!strings.Contains(err.Error(), "no rule group with id 99") {
		t.Errorf("rule error = %v, want not found", err)
	}
	if _, err := MaintainerrCollectionMedia(context.Background(), c, 99, 1, 25); err == nil ||
		!strings.Contains(err.Error(), "no collection with id 99") {
		t.Errorf("collection error = %v, want not found", err)
	}
}

// The id is a path segment; one that walks to /api/settings would return every
// credential Maintainerr holds.
func TestMaintainerrMediaStatusRejectsPathTraversal(t *testing.T) {
	c, paths, _ := maintainerrRoutes(t, map[string]string{})

	for _, id := range []string{"../../settings", "a/b", "", ".."} {
		if _, err := MaintainerrMediaStatus(context.Background(), c, id); err == nil {
			t.Errorf("id %q accepted", id)
		}
	}
	if len(*paths) != 0 {
		t.Errorf("requests sent for invalid ids: %v", *paths)
	}
}

// Excluding and postponing wait up to 30s for Maintainerr's execution lock
// before doing their own work, and an overlay run renders every poster before
// answering. On the default timeout the tool would report failure while the
// change still lands, so each builds its own long-timeout client. A base
// client with an impossibly short timeout still succeeds when it does.
func TestMaintainerrLockedCallsOverrideTheClientTimeout(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, map[string]string{
		"GET /api/app/status":                  `{}`,
		"POST /api/rules/exclusion":            `{"code":1}`,
		"POST /api/collections/media/postpone": `{"collectionId":1}`,
		"POST /api/overlays/process/1":         `{"processed":0}`,
	})
	impatient := c.WithTimeout(time.Nanosecond)
	ctx := context.Background()

	if _, err := impatient.Get(ctx, "/app/status"); err == nil {
		t.Fatal("a one-nanosecond timeout completed a request; the test proves nothing")
	}
	if err := MaintainerrAddExclusion(ctx, impatient, "abc", 0); err != nil {
		t.Errorf("add exclusion used the caller's short timeout: %v", err)
	}
	if _, err := MaintainerrPostpone(ctx, impatient, 1, "abc", 0); err != nil {
		t.Errorf("postpone used the caller's short timeout: %v", err)
	}
	if _, err := MaintainerrProcessOverlays(ctx, impatient, 1); err != nil {
		t.Errorf("overlay processing used the caller's short timeout: %v", err)
	}
}

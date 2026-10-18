package arr

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	isEpisode = "00cb39385fc82b4b4e9db35434770392"
	isSegment = "01a14e5f37457e479775608f97681260"
	isSeries  = "9993907cc9859aa5e0e31db2d60ce47a"
	isSeason  = "5d6c3e1a9b8f4c2d8e7f6a5b4c3d2e1f"
)

// introSkipperRoutes serves a status and body per "METHOD /path" and records
// each request line with its body, for calls that make more than one request.
func introSkipperRoutes(t *testing.T, routes map[string]struct {
	status int
	body   string
}) (*Client, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = append(seen, strings.TrimSpace(r.Method+" "+r.URL.RequestURI()+" "+string(b)))
		route, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(route.status)
		_, _ = w.Write([]byte(route.body))
	}))
	t.Cleanup(srv.Close)
	return jellyfinClient(srv.URL), &seen
}

func TestIntroSkipperListSegmentsProjects(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"Id":"01a14e5f37457e479775608f97681260","Type":"Introduction","Start":242.3544067,
	  "End":255.7558632,"Source":"Chromaprint","Suppressed":false},
	  {"Id":"02a14e5f37457e479775608f97681260","Type":"Credits","Start":1370.9,"End":1378.432,"Source":"Chapter","Suppressed":true}]`)

	segs, err := IntroSkipperListSegments(context.Background(), jellyfinClient(srv.URL), isEpisode, true)
	if err != nil {
		t.Fatalf("IntroSkipperListSegments: %v", err)
	}
	if got.method != "GET" || got.path != "/Episode/"+isEpisode+"/Segments" || got.query != "includeSuppressed=true" {
		t.Errorf("request = %s %s ?%s", got.method, got.path, got.query)
	}
	wantJellyfinAuth(t, got)
	want := []IntroSkipperSegment{
		{ID: isSegment, Type: "Introduction", StartSeconds: 242.3544067, EndSeconds: 255.7558632, Source: "Chromaprint"},
		{ID: "02a14e5f37457e479775608f97681260", Type: "Credits", StartSeconds: 1370.9, EndSeconds: 1378.432, Source: "Chapter", Suppressed: true},
	}
	if len(segs) != 2 || segs[0] != want[0] || segs[1] != want[1] {
		t.Errorf("segments = %+v, want %+v", segs, want)
	}
}

func TestIntroSkipperListSegmentsLeavesTombstonesOutByDefault(t *testing.T) {
	srv, got := fakeService(t, 200, `[]`)
	segs, err := IntroSkipperListSegments(context.Background(), jellyfinClient(srv.URL), isEpisode, false)
	if err != nil || segs == nil || len(segs) != 0 {
		t.Errorf("segments = %#v, err = %v; want an empty, non-nil list", segs, err)
	}
	if got.query != "" {
		t.Errorf("query = %q, want none", got.query)
	}
}

// A 201 carries the segment itself: Jellyfin's copy, which playback reads, is
// already updated.
func TestIntroSkipperCreateSegmentPostsTypeAndSeconds(t *testing.T) {
	srv, got := fakeService(t, 201, `{"Id":"01a14e5f37457e479775608f97681260","Type":"Credits","Start":1232.2,"End":1270.1,
	  "Source":"User","Suppressed":false}`)

	ch, err := IntroSkipperCreateSegment(context.Background(), jellyfinClient(srv.URL), isEpisode, "Credits", 1232.2, 1270.1)
	if err != nil {
		t.Fatalf("IntroSkipperCreateSegment: %v", err)
	}
	if got.method != "POST" || got.path != "/Episode/"+isEpisode+"/Segments" ||
		got.body != `{"Type":"Credits","Start":1232.2,"End":1270.1}` {
		t.Errorf("request = %s %s %s", got.method, got.path, got.body)
	}
	wantJellyfinAuth(t, got)
	if ch.Projection != "Applied" || len(ch.Segments) != 1 || ch.Segments[0].Source != "User" || ch.Segments[0].EndSeconds != 1270.1 {
		t.Errorf("change = %+v", ch)
	}
}

// A 202 means Intro Skipper saved the change but Jellyfin's copy has not caught
// up; that must not read as applied.
func TestIntroSkipperUpdateSegmentReportsAPendingProjection(t *testing.T) {
	srv, got := fakeService(t, 202, `{"ChangeStatus":"Accepted","Projection":"Pending","Segments":[
	  {"Id":"01a14e5f37457e479775608f97681260","Type":"Introduction","Start":96.054,"End":108.1,"Source":"User","Suppressed":false}]}`)

	ch, err := IntroSkipperUpdateSegment(context.Background(), jellyfinClient(srv.URL), isEpisode, isSegment, 96.054, 108.1)
	if err != nil {
		t.Fatalf("IntroSkipperUpdateSegment: %v", err)
	}
	if got.method != "PUT" || got.path != "/Episode/"+isEpisode+"/Segments/"+isSegment || got.body != `{"Start":96.054,"End":108.1}` {
		t.Errorf("request = %s %s %s", got.method, got.path, got.body)
	}
	if ch.Projection != "Pending" || len(ch.Segments) != 1 || ch.Segments[0].StartSeconds != 96.054 {
		t.Errorf("change = %+v", ch)
	}
}

func TestIntroSkipperDeleteSegmentAnswersNoContent(t *testing.T) {
	srv, got := fakeService(t, 204, ``)
	ch, err := IntroSkipperDeleteSegment(context.Background(), jellyfinClient(srv.URL), isEpisode, isSegment)
	if err != nil {
		t.Fatalf("IntroSkipperDeleteSegment: %v", err)
	}
	if got.method != "DELETE" || got.path != "/Episode/"+isEpisode+"/Segments/"+isSegment {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if ch.Projection != "Applied" || ch.Segments == nil || len(ch.Segments) != 0 {
		t.Errorf("change = %#v, want applied with an empty segment list", ch)
	}
}

func TestIntroSkipperRestoreSegmentPostsRestore(t *testing.T) {
	srv, got := fakeService(t, 200, `{"Id":"01a14e5f37457e479775608f97681260","Type":"Credits","Start":1.5,"End":9,
	  "Source":"Chapter","Suppressed":false}`)
	ch, err := IntroSkipperRestoreSegment(context.Background(), jellyfinClient(srv.URL), isEpisode, isSegment)
	if err != nil {
		t.Fatalf("IntroSkipperRestoreSegment: %v", err)
	}
	if got.method != "POST" || got.path != "/Episode/"+isEpisode+"/Segments/"+isSegment+"/Restore" || got.body != "" {
		t.Errorf("request = %s %s %q", got.method, got.path, got.body)
	}
	if ch.Projection != "Applied" || len(ch.Segments) != 1 || ch.Segments[0].Suppressed {
		t.Errorf("change = %+v", ch)
	}
}

// Intro Skipper names segment types Introduction and Credits where Jellyfin
// says Intro and Outro; the Jellyfin name must be refused with the right one,
// before any request, as must boundaries the plugin would reject.
func TestIntroSkipperCreateSegmentRejectsBadInput(t *testing.T) {
	srv, got := fakeService(t, 201, `{}`)
	c := jellyfinClient(srv.URL)
	ctx := context.Background()
	_, err := IntroSkipperCreateSegment(ctx, c, isEpisode, "Intro", 1, 2)
	if err == nil || !strings.Contains(err.Error(), "Introduction") {
		t.Errorf("type Intro: err = %v, want one naming Introduction", err)
	}
	for _, tc := range []struct{ start, end float64 }{{-1, 5}, {5, 5}, {9, 5}} {
		if _, err := IntroSkipperCreateSegment(ctx, c, isEpisode, "Credits", tc.start, tc.end); err == nil {
			t.Errorf("start %v end %v accepted", tc.start, tc.end)
		}
		if _, err := IntroSkipperUpdateSegment(ctx, c, isEpisode, isSegment, tc.start, tc.end); err == nil {
			t.Errorf("update start %v end %v accepted", tc.start, tc.end)
		}
	}
	if len(got.paths) != 0 {
		t.Errorf("requests reached the server: %v", got.paths)
	}
}

func TestIntroSkipperAnalyzersReadsTheSeason(t *testing.T) {
	srv, got := fakeService(t, 200, `{"Introduction":"Chromaprint","Credits":"Default","Preview":"Default","Recap":"Default","Commercial":"Default"}`)
	actions, err := IntroSkipperAnalyzers(context.Background(), jellyfinClient(srv.URL), isSeason)
	if err != nil {
		t.Fatalf("IntroSkipperAnalyzers: %v", err)
	}
	if got.method != "GET" || got.path != "/Intros/AnalyzerActions/"+isSeason {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if len(actions) != 5 || actions["Introduction"] != "Chromaprint" || actions["Credits"] != "Default" {
		t.Errorf("actions = %v", actions)
	}
}

// The plugin's UpdateSeason accepts any id with a 204, so the season is read
// first: an unknown id must fail rather than store actions for nothing. Only the
// named modes are sent, the id goes in its dashed form, and the result is the
// season read back after the write.
func TestIntroSkipperSetAnalyzersChecksTheSeasonFirst(t *testing.T) {
	c, seen := introSkipperRoutes(t, map[string]struct {
		status int
		body   string
	}{
		"GET /Intros/AnalyzerActions/" + isSeason:   {200, `{"Introduction":"Chromaprint","Credits":"Default"}`},
		"POST /Intros/AnalyzerActions/UpdateSeason": {204, ``},
	})

	actions, err := IntroSkipperSetAnalyzers(context.Background(), c, isSeason, map[string]string{"Credits": "Chapter"})
	if err != nil {
		t.Fatalf("IntroSkipperSetAnalyzers: %v", err)
	}
	want := []string{
		"GET /Intros/AnalyzerActions/" + isSeason,
		`POST /Intros/AnalyzerActions/UpdateSeason {"Id":"5d6c3e1a-9b8f-4c2d-8e7f-6a5b4c3d2e1f","AnalyzerActions":{"Credits":"Chapter"}}`,
		"GET /Intros/AnalyzerActions/" + isSeason,
	}
	if len(*seen) != 3 || (*seen)[0] != want[0] || (*seen)[1] != want[1] || (*seen)[2] != want[2] {
		t.Errorf("requests = %q, want %q", *seen, want)
	}
	if actions["Introduction"] != "Chromaprint" {
		t.Errorf("actions = %v, want the season as read back", actions)
	}
}

func TestIntroSkipperSetAnalyzersRefusesAnUnknownSeason(t *testing.T) {
	c, seen := introSkipperRoutes(t, map[string]struct {
		status int
		body   string
	}{"POST /Intros/AnalyzerActions/UpdateSeason": {204, ``}})
	_, err := IntroSkipperSetAnalyzers(context.Background(), c, isSeason, map[string]string{"Credits": "Chapter"})
	var se *StatusError
	if !errors.As(err, &se) || se.Status != 404 {
		t.Errorf("err = %v, want the season lookup's 404", err)
	}
	for _, line := range *seen {
		if strings.HasPrefix(line, "POST") {
			t.Errorf("an unknown season was written to: %v", *seen)
		}
	}
}

func TestIntroSkipperSetAnalyzersRejectsBadNames(t *testing.T) {
	srv, got := fakeService(t, 200, `{}`)
	c := jellyfinClient(srv.URL)
	for name, actions := range map[string]map[string]string{
		"empty":           {},
		"jellyfin mode":   {"Intro": "Chromaprint"},
		"unknown action":  {"Credits": "Fingerprint"},
		"one of two bad":  {"Credits": "Chapter", "Outro": "Chapter"},
		"lowercase value": {"Credits": "chapter"},
	} {
		if _, err := IntroSkipperSetAnalyzers(context.Background(), c, isSeason, actions); err == nil {
			t.Errorf("%s: %v accepted", name, actions)
		}
	}
	if len(got.paths) != 0 {
		t.Errorf("requests reached the server: %v", got.paths)
	}
}

func TestIntroSkipperScanSeasonPostsBothIDs(t *testing.T) {
	srv, got := fakeService(t, 202, ``)
	if err := IntroSkipperScanSeason(context.Background(), jellyfinClient(srv.URL), isSeries, isSeason); err != nil {
		t.Fatalf("IntroSkipperScanSeason: %v", err)
	}
	if got.method != "POST" || got.path != "/Intros/ScanSeason/"+isSeries+"/"+isSeason || got.body != "" {
		t.Errorf("request = %s %s %q", got.method, got.path, got.body)
	}
	wantJellyfinAuth(t, got)
}

// 12.0.4.0 runs one scan at a time and answers 409 rather than queueing.
func TestIntroSkipperScanSeasonExplainsAConflict(t *testing.T) {
	srv, _ := fakeService(t, 409, `{"message":"A scan is already in progress."}`)
	err := IntroSkipperScanSeason(context.Background(), jellyfinClient(srv.URL), isSeries, isSeason)
	var se *StatusError
	if !errors.As(err, &se) || se.Status != 409 || !strings.Contains(err.Error(), "jellyfin_get_introskipper_scan_status") {
		t.Errorf("err = %v, want the 409 kept and a pointer to the status tool", err)
	}
}

// 12.0.5.0 reports per season; 12.0.4.0 has only a server-wide route, which
// the per-season one is a 404 on.
func TestIntroSkipperScanStatusPerSeason(t *testing.T) {
	srv, got := fakeService(t, 200, `{"isRunning":false,"isQueued":true,"failed":false}`)
	st, err := IntroSkipperScanStatus(context.Background(), jellyfinClient(srv.URL), isSeason)
	if err != nil {
		t.Fatalf("IntroSkipperScanStatus: %v", err)
	}
	if got.path != "/Intros/ScanStatus/"+isSeason {
		t.Errorf("path = %s", got.path)
	}
	if st.Scope != "season" || st.IsRunning || st.IsQueued == nil || !*st.IsQueued || st.Failed == nil || *st.Failed {
		t.Errorf("status = %+v", st)
	}
}

func TestIntroSkipperScanStatusFallsBackOnOlderBuilds(t *testing.T) {
	c, seen := introSkipperRoutes(t, map[string]struct {
		status int
		body   string
	}{"GET /Intros/ScanStatus": {200, `{"isRunning":true}`}})
	st, err := IntroSkipperScanStatus(context.Background(), c, isSeason)
	if err != nil {
		t.Fatalf("IntroSkipperScanStatus: %v", err)
	}
	if len(*seen) != 2 || (*seen)[0] != "GET /Intros/ScanStatus/"+isSeason || (*seen)[1] != "GET /Intros/ScanStatus" {
		t.Errorf("requests = %q", *seen)
	}
	if st.Scope != "server" || !st.IsRunning || st.IsQueued != nil || st.Failed != nil {
		t.Errorf("status = %+v, want a server-wide isRunning only", st)
	}
}

func TestIntroSkipperUpstreamErrorsCarryTheStatus(t *testing.T) {
	srv, _ := fakeService(t, 404, `{"title":"Not Found"}`)
	c := jellyfinClient(srv.URL)
	ctx := context.Background()
	checks := map[string]func() error{
		"list":      func() error { _, err := IntroSkipperListSegments(ctx, c, isEpisode, false); return err },
		"create":    func() error { _, err := IntroSkipperCreateSegment(ctx, c, isEpisode, "Credits", 1, 2); return err },
		"update":    func() error { _, err := IntroSkipperUpdateSegment(ctx, c, isEpisode, isSegment, 1, 2); return err },
		"delete":    func() error { _, err := IntroSkipperDeleteSegment(ctx, c, isEpisode, isSegment); return err },
		"restore":   func() error { _, err := IntroSkipperRestoreSegment(ctx, c, isEpisode, isSegment); return err },
		"analyzers": func() error { _, err := IntroSkipperAnalyzers(ctx, c, isSeason); return err },
		"set": func() error {
			_, err := IntroSkipperSetAnalyzers(ctx, c, isSeason, map[string]string{"Credits": "Chapter"})
			return err
		},
		"scan":   func() error { return IntroSkipperScanSeason(ctx, c, isSeries, isSeason) },
		"status": func() error { _, err := IntroSkipperScanStatus(ctx, c, isSeason); return err },
	}
	for name, call := range checks {
		var se *StatusError
		if err := call(); !errors.As(err, &se) || se.Status != 404 {
			t.Errorf("%s: err = %v, want a 404 StatusError", name, err)
		}
	}
}

// Every id is spliced into a path or a body; anything that is not a Jellyfin id
// must be refused before a request leaves.
func TestIntroSkipperRejectsEveryNonIDBeforeAnyRequest(t *testing.T) {
	srv, got := fakeService(t, 200, `{}`)
	c := jellyfinClient(srv.URL)
	ctx := context.Background()
	set := map[string]string{"Credits": "Chapter"}
	for _, bad := range []string{"../../System/Shutdown", "", "7489", isEpisode + "/Restore", "%2e%2e"} {
		calls := map[string]error{
			"list":         func() error { _, err := IntroSkipperListSegments(ctx, c, bad, false); return err }(),
			"create":       func() error { _, err := IntroSkipperCreateSegment(ctx, c, bad, "Credits", 1, 2); return err }(),
			"update item":  func() error { _, err := IntroSkipperUpdateSegment(ctx, c, bad, isSegment, 1, 2); return err }(),
			"update seg":   func() error { _, err := IntroSkipperUpdateSegment(ctx, c, isEpisode, bad, 1, 2); return err }(),
			"delete item":  func() error { _, err := IntroSkipperDeleteSegment(ctx, c, bad, isSegment); return err }(),
			"delete seg":   func() error { _, err := IntroSkipperDeleteSegment(ctx, c, isEpisode, bad); return err }(),
			"restore item": func() error { _, err := IntroSkipperRestoreSegment(ctx, c, bad, isSegment); return err }(),
			"restore seg":  func() error { _, err := IntroSkipperRestoreSegment(ctx, c, isEpisode, bad); return err }(),
			"analyzers":    func() error { _, err := IntroSkipperAnalyzers(ctx, c, bad); return err }(),
			"set":          func() error { _, err := IntroSkipperSetAnalyzers(ctx, c, bad, set); return err }(),
			"scan series":  IntroSkipperScanSeason(ctx, c, bad, isSeason),
			"scan season":  IntroSkipperScanSeason(ctx, c, isSeries, bad),
			"status":       func() error { _, err := IntroSkipperScanStatus(ctx, c, bad); return err }(),
		}
		for name, err := range calls {
			if err == nil {
				t.Errorf("%s accepted %q", name, bad)
			}
		}
	}
	if len(got.paths) != 0 {
		t.Errorf("requests reached the server: %v", got.paths)
	}
}

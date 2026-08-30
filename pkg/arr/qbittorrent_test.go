package arr

import (
	"context"
	"strings"
	"testing"
)

// tail returns the requests recorded after the login round-trip, which every
// test here triggers and none of them is about.
func tail(f *qbitFake) []string {
	var out []string
	for _, p := range f.paths {
		if p != "POST /api/v2/auth/login" {
			out = append(out, p)
		}
	}
	return out
}

func TestQBittorrentListTorrentsSendsFilterCategoryTagAndHashes(t *testing.T) {
	f := fakeQBit(t)
	f.responses = map[string]string{"/api/v2/torrents/info": `[]`}
	category := "tv"
	tag := "sonarr"

	_, err := QBittorrentListTorrents(context.Background(), qbitClient(f), TorrentFilter{
		Filter: "downloading", Category: &category, Tag: &tag, Hashes: []string{"aaa", "bbb"}, Limit: 5,
	})
	if err != nil {
		t.Fatalf("ListTorrents returned error: %v", err)
	}
	if got := tail(f); len(got) != 1 || got[0] != "GET /api/v2/torrents/info" {
		t.Fatalf("requests = %v, want one GET /api/v2/torrents/info", got)
	}
	for k, want := range map[string]string{
		"filter": "downloading", "category": "tv", "tag": "sonarr", "hashes": "aaa|bbb", "limit": "5",
	} {
		if got := f.lastQuery.Get(k); got != want {
			t.Errorf("query %s = %q, want %q", k, got, want)
		}
	}
}

// A nil category means "no filter"; an empty string is qBittorrent's spelling
// of "uncategorised", so the two must stay distinguishable on the wire.
func TestQBittorrentListTorrentsOmitsUnsetFilters(t *testing.T) {
	f := fakeQBit(t)
	f.responses = map[string]string{"/api/v2/torrents/info": `[]`}

	if _, err := QBittorrentListTorrents(context.Background(), qbitClient(f), TorrentFilter{}); err != nil {
		t.Fatalf("ListTorrents returned error: %v", err)
	}
	for _, k := range []string{"filter", "category", "tag", "hashes"} {
		if _, present := f.lastQuery[k]; present {
			t.Errorf("query carries %s=%q for an unset filter", k, f.lastQuery.Get(k))
		}
	}
}

func TestQBittorrentListTorrentsProjectsSnakeCaseAndSplitsTags(t *testing.T) {
	f := fakeQBit(t)
	f.responses = map[string]string{"/api/v2/torrents/info": `[{
		"hash":"abc","name":"debian.iso","state":"downloading","progress":0.25,
		"size":1000,"downloaded":250,"uploaded":50,"dlspeed":123,"upspeed":45,"eta":87,
		"ratio":0.2,"category":"linux","tags":"iso, sonarr,,keep","save_path":"/downloads",
		"added_on":1700000000,"completion_on":0,"num_seeds":54,"num_leechs":2,"priority":1,
		"ratio_limit":-2,"seeding_time_limit":-2,"dl_limit":0,"up_limit":1024,
		"magnet_uri":"magnet:?xt=urn:btih:abc","content_path":"/downloads/debian.iso"
	}]`}

	torrents, err := QBittorrentListTorrents(context.Background(), qbitClient(f), TorrentFilter{})
	if err != nil {
		t.Fatalf("ListTorrents returned error: %v", err)
	}
	if len(torrents) != 1 {
		t.Fatalf("got %d torrents, want 1", len(torrents))
	}
	got := torrents[0]
	if got.Hash != "abc" || got.Name != "debian.iso" || got.State != "downloading" {
		t.Errorf("identity fields = %+v", got)
	}
	if got.DownloadSpeed != 123 || got.UploadSpeed != 45 || got.SavePath != "/downloads" {
		t.Errorf("snake_case fields not projected: %+v", got)
	}
	if got.Seeds != 54 || got.Leechers != 2 || got.UploadLimit != 1024 || got.RatioLimit != -2 {
		t.Errorf("peer and limit fields not projected: %+v", got)
	}
	if strings.Join(got.Tags, ",") != "iso,sonarr,keep" {
		t.Errorf("tags = %v, want [iso sonarr keep]", got.Tags)
	}
	if got.AddedOn != "2023-11-14T22:13:20Z" {
		t.Errorf("addedOn = %q, want RFC3339 UTC", got.AddedOn)
	}
	if got.CompletionOn != "" {
		t.Errorf("completionOn = %q, want empty for an unfinished torrent", got.CompletionOn)
	}
}

func TestQBittorrentTorrentFilesQueriesByHash(t *testing.T) {
	f := fakeQBit(t)
	f.responses = map[string]string{"/api/v2/torrents/files": `[{"index":0,"name":"a/b.mkv","size":10,"progress":1,"priority":1,"is_seed":true,"piece_range":[0,3]}]`}

	files, err := QBittorrentTorrentFiles(context.Background(), qbitClient(f), "abc")
	if err != nil {
		t.Fatalf("TorrentFiles returned error: %v", err)
	}
	if f.lastQuery.Get("hash") != "abc" {
		t.Errorf("hash query = %q, want abc", f.lastQuery.Get("hash"))
	}
	if len(files) != 1 || files[0].Name != "a/b.mkv" || files[0].Size != 10 || files[0].Priority != 1 {
		t.Errorf("files = %+v", files)
	}
}

func TestQBittorrentSystemStatusReadsBothVersionEndpoints(t *testing.T) {
	f := fakeQBit(t)
	f.responses = map[string]string{
		"/api/v2/app/version":       "v5.1.2\n",
		"/api/v2/app/webapiVersion": "2.11.4",
	}

	v, err := QBittorrentSystemStatus(context.Background(), qbitClient(f))
	if err != nil {
		t.Fatalf("SystemStatus returned error: %v", err)
	}
	if v.Version != "v5.1.2" || v.APIVersion != "2.11.4" {
		t.Errorf("version = %+v, want v5.1.2 / 2.11.4", v)
	}
}

func TestQBittorrentTransferInfoCombinesInfoAndSpeedLimitsMode(t *testing.T) {
	f := fakeQBit(t)
	f.responses = map[string]string{
		"/api/v2/transfer/info":            `{"connection_status":"connected","dht_nodes":386,"dl_info_speed":10,"up_info_speed":20,"dl_rate_limit":0,"up_rate_limit":1048576}`,
		"/api/v2/transfer/speedLimitsMode": "1",
	}

	info, err := QBittorrentTransferInfo(context.Background(), qbitClient(f))
	if err != nil {
		t.Fatalf("TransferInfo returned error: %v", err)
	}
	if info.DownloadSpeed != 10 || info.UploadSpeed != 20 || info.UploadLimit != 1048576 || info.DHTNodes != 386 {
		t.Errorf("info = %+v", info)
	}
	if info.ConnectionStatus != "connected" || !info.AlternativeSpeedLimits {
		t.Errorf("status = %+v, want connected with alternative limits on", info)
	}
}

func TestQBittorrentListCategoriesFlattensTheMap(t *testing.T) {
	f := fakeQBit(t)
	f.responses = map[string]string{"/api/v2/torrents/categories": `{"tv":{"name":"tv","savePath":"/downloads/tv"},"movies":{"name":"movies","savePath":""}}`}

	cats, err := QBittorrentListCategories(context.Background(), qbitClient(f))
	if err != nil {
		t.Fatalf("ListCategories returned error: %v", err)
	}
	if len(cats) != 2 || cats[0].Name != "movies" || cats[1].Name != "tv" || cats[1].SavePath != "/downloads/tv" {
		t.Errorf("categories = %+v, want movies then tv", cats)
	}
}

func TestQBittorrentListTagsDecodesTheArray(t *testing.T) {
	f := fakeQBit(t)
	f.responses = map[string]string{"/api/v2/torrents/tags": `["keep","sonarr"]`}

	tags, err := QBittorrentListTags(context.Background(), qbitClient(f))
	if err != nil {
		t.Fatalf("ListTags returned error: %v", err)
	}
	if strings.Join(tags, ",") != "keep,sonarr" {
		t.Errorf("tags = %v", tags)
	}
}

func TestQBittorrentAddTorrentJoinsURLsAndSendsStopped(t *testing.T) {
	f := fakeQBit(t)
	f.responses = map[string]string{"/api/v2/torrents/add": "Ok."}
	dl := int64(2048)
	ratio := 1.5
	auto := false

	err := QBittorrentAddTorrent(context.Background(), qbitClient(f), AddTorrentRequest{
		URLs:          []string{"magnet:?xt=urn:btih:abc", "https://example.test/a.torrent"},
		SavePath:      "/downloads/tv",
		Category:      "tv",
		Tags:          []string{"sonarr", "keep"},
		Stopped:       true,
		Rename:        "renamed",
		DownloadLimit: &dl,
		RatioLimit:    &ratio,
		AutoTMM:       &auto,
	})
	if err != nil {
		t.Fatalf("AddTorrent returned error: %v", err)
	}
	if got := tail(f); len(got) != 1 || got[0] != "POST /api/v2/torrents/add" {
		t.Fatalf("requests = %v, want one POST /api/v2/torrents/add", got)
	}
	for k, want := range map[string]string{
		"urls":     "magnet%3A%3Fxt%3Durn%3Abtih%3Aabc%0Ahttps%3A%2F%2Fexample.test%2Fa.torrent",
		"savepath": "%2Fdownloads%2Ftv", "category": "tv", "tags": "sonarr%2Ckeep", "stopped": "true",
		"rename": "renamed", "dlLimit": "2048", "ratioLimit": "1.5", "autoTMM": "false",
	} {
		if !strings.Contains(f.lastBody, k+"="+want) {
			t.Errorf("body %q lacks %s=%s", f.lastBody, k, want)
		}
	}
	for _, absent := range []string{"upLimit", "seedingTimeLimit"} {
		if strings.Contains(f.lastBody, absent+"=") {
			t.Errorf("body %q carries %s, which was not set", f.lastBody, absent)
		}
	}
}

func TestQBittorrentAddTorrentOmitsStoppedWhenFalse(t *testing.T) {
	f := fakeQBit(t)
	f.responses = map[string]string{"/api/v2/torrents/add": "Ok."}

	err := QBittorrentAddTorrent(context.Background(), qbitClient(f), AddTorrentRequest{URLs: []string{"magnet:?xt=urn:btih:abc"}})
	if err != nil {
		t.Fatalf("AddTorrent returned error: %v", err)
	}
	if strings.Contains(f.lastBody, "stopped=") {
		t.Errorf("body %q sends stopped for a torrent that should start", f.lastBody)
	}
}

// qBittorrent answers 200 with "Fails." for a URL it cannot add, so the body
// is the only signal that anything went wrong.
func TestQBittorrentAddTorrentRejectsNonOkBody(t *testing.T) {
	f := fakeQBit(t)
	f.responses = map[string]string{"/api/v2/torrents/add": "Fails."}

	err := QBittorrentAddTorrent(context.Background(), qbitClient(f), AddTorrentRequest{URLs: []string{"magnet:?xt=urn:btih:abc"}})
	if err == nil {
		t.Fatal("expected an error for a Fails. body, got nil")
	}
	if !strings.Contains(err.Error(), "Fails.") {
		t.Errorf("error %q does not quote the response body", err)
	}
}

func TestQBittorrentAddTorrentRejectsUnsupportedURLsBeforeSending(t *testing.T) {
	f := fakeQBit(t)

	err := QBittorrentAddTorrent(context.Background(), qbitClient(f), AddTorrentRequest{URLs: []string{"ftp://example.test/a.torrent"}})
	if err == nil {
		t.Fatal("expected an error for an ftp url, got nil")
	}
	if len(f.paths) != 0 {
		t.Errorf("requests = %v, want none", f.paths)
	}

	if err := QBittorrentAddTorrent(context.Background(), qbitClient(f), AddTorrentRequest{}); err == nil {
		t.Fatal("expected an error for no urls, got nil")
	}
}

func TestQBittorrentStopTorrentsJoinsHashesWithPipe(t *testing.T) {
	f := fakeQBit(t)

	if err := QBittorrentStopTorrents(context.Background(), qbitClient(f), []string{"aaa", "bbb"}); err != nil {
		t.Fatalf("StopTorrents returned error: %v", err)
	}
	if got := tail(f); len(got) != 1 || got[0] != "POST /api/v2/torrents/stop" {
		t.Fatalf("requests = %v, want one POST /api/v2/torrents/stop", got)
	}
	if f.lastBody != "hashes=aaa%7Cbbb" {
		t.Errorf("body = %q, want hashes=aaa%%7Cbbb", f.lastBody)
	}
}

func TestQBittorrentStartAndRecheckUseV5Paths(t *testing.T) {
	f := fakeQBit(t)
	c := qbitClient(f)

	if err := QBittorrentStartTorrents(context.Background(), c, []string{"aaa"}); err != nil {
		t.Fatalf("StartTorrents returned error: %v", err)
	}
	if err := QBittorrentRecheckTorrents(context.Background(), c, []string{"aaa"}); err != nil {
		t.Fatalf("RecheckTorrents returned error: %v", err)
	}
	want := "POST /api/v2/torrents/start,POST /api/v2/torrents/recheck"
	if got := strings.Join(tail(f), ","); got != want {
		t.Errorf("requests = %s, want %s", got, want)
	}
}

func TestQBittorrentAllSentinelIsSentVerbatim(t *testing.T) {
	f := fakeQBit(t)

	if err := QBittorrentStartTorrents(context.Background(), qbitClient(f), []string{"all"}); err != nil {
		t.Fatalf("StartTorrents returned error: %v", err)
	}
	if f.lastBody != "hashes=all" {
		t.Errorf("body = %q, want hashes=all", f.lastBody)
	}
}

func TestQBittorrentEmptyHashesFailBeforeAnyRequest(t *testing.T) {
	f := fakeQBit(t)

	err := QBittorrentStopTorrents(context.Background(), qbitClient(f), nil)
	if err == nil {
		t.Fatal("expected an error for empty hashes, got nil")
	}
	if !strings.Contains(err.Error(), "all") {
		t.Errorf("error %q does not mention the all sentinel", err)
	}
	if len(f.paths) != 0 {
		t.Errorf("requests = %v, want none (not even a login)", f.paths)
	}
}

func TestQBittorrentDeleteTorrentsSendsDeleteFiles(t *testing.T) {
	f := fakeQBit(t)

	if err := QBittorrentDeleteTorrents(context.Background(), qbitClient(f), []string{"aaa"}, true); err != nil {
		t.Fatalf("DeleteTorrents returned error: %v", err)
	}
	if got := tail(f); len(got) != 1 || got[0] != "POST /api/v2/torrents/delete" {
		t.Fatalf("requests = %v, want one POST /api/v2/torrents/delete", got)
	}
	if f.lastBody != "deleteFiles=true&hashes=aaa" {
		t.Errorf("body = %q, want deleteFiles=true&hashes=aaa", f.lastBody)
	}
}

func TestQBittorrentDeleteTorrentsKeepsFilesByDefault(t *testing.T) {
	f := fakeQBit(t)

	if err := QBittorrentDeleteTorrents(context.Background(), qbitClient(f), []string{"aaa"}, false); err != nil {
		t.Fatalf("DeleteTorrents returned error: %v", err)
	}
	if f.lastBody != "deleteFiles=false&hashes=aaa" {
		t.Errorf("body = %q, want deleteFiles=false&hashes=aaa", f.lastBody)
	}
}

func TestQBittorrentCategoryCallsSendTheirForms(t *testing.T) {
	f := fakeQBit(t)
	c := qbitClient(f)
	ctx := context.Background()

	if err := QBittorrentSetCategory(ctx, c, []string{"aaa"}, "tv"); err != nil {
		t.Fatalf("SetCategory: %v", err)
	}
	if f.lastBody != "category=tv&hashes=aaa" {
		t.Errorf("setCategory body = %q", f.lastBody)
	}
	if err := QBittorrentCreateCategory(ctx, c, "tv", "/downloads/tv"); err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if f.lastBody != "category=tv&savePath=%2Fdownloads%2Ftv" {
		t.Errorf("createCategory body = %q", f.lastBody)
	}
	if err := QBittorrentEditCategory(ctx, c, "tv", "/mnt/tv"); err != nil {
		t.Fatalf("EditCategory: %v", err)
	}
	if f.lastBody != "category=tv&savePath=%2Fmnt%2Ftv" {
		t.Errorf("editCategory body = %q", f.lastBody)
	}
	if err := QBittorrentRemoveCategories(ctx, c, []string{"tv", "old"}); err != nil {
		t.Fatalf("RemoveCategories: %v", err)
	}
	if f.lastBody != "categories=tv%0Aold" {
		t.Errorf("removeCategories body = %q, want newline-joined names", f.lastBody)
	}
	want := "POST /api/v2/torrents/setCategory,POST /api/v2/torrents/createCategory,POST /api/v2/torrents/editCategory,POST /api/v2/torrents/removeCategories"
	if got := strings.Join(tail(f), ","); got != want {
		t.Errorf("requests = %s, want %s", got, want)
	}
}

func TestQBittorrentTagCallsJoinTagsWithComma(t *testing.T) {
	f := fakeQBit(t)
	c := qbitClient(f)
	ctx := context.Background()

	if err := QBittorrentAddTags(ctx, c, []string{"aaa", "bbb"}, []string{"keep", "seen"}); err != nil {
		t.Fatalf("AddTags: %v", err)
	}
	if f.lastBody != "hashes=aaa%7Cbbb&tags=keep%2Cseen" {
		t.Errorf("addTags body = %q", f.lastBody)
	}
	// Removing with no tags named strips every tag, which is what qBittorrent
	// does with an empty tags field.
	if err := QBittorrentRemoveTags(ctx, c, []string{"aaa"}, nil); err != nil {
		t.Fatalf("RemoveTags: %v", err)
	}
	if f.lastBody != "hashes=aaa&tags=" {
		t.Errorf("removeTags body = %q", f.lastBody)
	}
	want := "POST /api/v2/torrents/addTags,POST /api/v2/torrents/removeTags"
	if got := strings.Join(tail(f), ","); got != want {
		t.Errorf("requests = %s, want %s", got, want)
	}
}

func TestQBittorrentSetLocationAndRename(t *testing.T) {
	f := fakeQBit(t)
	c := qbitClient(f)
	ctx := context.Background()

	if err := QBittorrentSetLocation(ctx, c, []string{"aaa"}, "/mnt/new"); err != nil {
		t.Fatalf("SetLocation: %v", err)
	}
	if f.lastBody != "hashes=aaa&location=%2Fmnt%2Fnew" {
		t.Errorf("setLocation body = %q", f.lastBody)
	}
	if err := QBittorrentRenameTorrent(ctx, c, "aaa", "New Name"); err != nil {
		t.Fatalf("RenameTorrent: %v", err)
	}
	if f.lastBody != "hash=aaa&name=New+Name" {
		t.Errorf("rename body = %q", f.lastBody)
	}
	want := "POST /api/v2/torrents/setLocation,POST /api/v2/torrents/rename"
	if got := strings.Join(tail(f), ","); got != want {
		t.Errorf("requests = %s, want %s", got, want)
	}
}

func TestQBittorrentSetPriorityMapsPositionsToEndpoints(t *testing.T) {
	f := fakeQBit(t)
	c := qbitClient(f)
	ctx := context.Background()

	for _, pos := range []string{"top", "bottom", "up", "down"} {
		if err := QBittorrentSetPriority(ctx, c, []string{"aaa"}, pos); err != nil {
			t.Fatalf("SetPriority(%s): %v", pos, err)
		}
	}
	want := "POST /api/v2/torrents/topPrio,POST /api/v2/torrents/bottomPrio,POST /api/v2/torrents/increasePrio,POST /api/v2/torrents/decreasePrio"
	if got := strings.Join(tail(f), ","); got != want {
		t.Errorf("requests = %s, want %s", got, want)
	}

	before := len(f.paths)
	err := QBittorrentSetPriority(ctx, c, []string{"aaa"}, "middle")
	if err == nil {
		t.Fatal("expected an error for an unknown position, got nil")
	}
	if !strings.Contains(err.Error(), "top") {
		t.Errorf("error %q does not list the valid positions", err)
	}
	if len(f.paths) != before {
		t.Errorf("an invalid position still reached the server: %v", f.paths[before:])
	}
}

func TestQBittorrentSetTorrentLimitsOnlyCallsTheEndpointsGiven(t *testing.T) {
	f := fakeQBit(t)
	up := int64(512)

	err := QBittorrentSetTorrentLimits(context.Background(), qbitClient(f), []string{"aaa"}, nil, &up, nil)
	if err != nil {
		t.Fatalf("SetTorrentLimits returned error: %v", err)
	}
	if got := tail(f); len(got) != 1 || got[0] != "POST /api/v2/torrents/setUploadLimit" {
		t.Errorf("requests = %v, want only setUploadLimit", got)
	}
	if f.lastBody != "hashes=aaa&limit=512" {
		t.Errorf("body = %q", f.lastBody)
	}
}

// setShareLimits requires all three limits on every call, so the ones the
// caller did not mention are sent as -2, qBittorrent's "use the global
// default" value, rather than being silently reset to unlimited.
func TestQBittorrentSetTorrentLimitsFillsOmittedShareLimitsWithGlobalDefault(t *testing.T) {
	f := fakeQBit(t)
	ratio := 2.0
	dl := int64(0)

	err := QBittorrentSetTorrentLimits(context.Background(), qbitClient(f), []string{"aaa"}, &dl, nil, &ShareLimits{RatioLimit: &ratio})
	if err != nil {
		t.Fatalf("SetTorrentLimits returned error: %v", err)
	}
	want := "POST /api/v2/torrents/setDownloadLimit,POST /api/v2/torrents/setShareLimits"
	if got := strings.Join(tail(f), ","); got != want {
		t.Errorf("requests = %s, want %s", got, want)
	}
	if f.lastBody != "hashes=aaa&inactiveSeedingTimeLimit=-2&ratioLimit=2&seedingTimeLimit=-2" {
		t.Errorf("setShareLimits body = %q", f.lastBody)
	}
}

func TestQBittorrentSetGlobalLimitsTogglesAlternativeModeOnlyOnMismatch(t *testing.T) {
	f := fakeQBit(t)
	f.responses = map[string]string{
		"/api/v2/transfer/info":            `{"dl_info_speed":0,"up_info_speed":0,"dl_rate_limit":4096,"up_rate_limit":0,"dht_nodes":1,"connection_status":"connected"}`,
		"/api/v2/transfer/speedLimitsMode": "0",
	}
	dl := int64(4096)
	on := true

	info, err := QBittorrentSetGlobalLimits(context.Background(), qbitClient(f), &dl, nil, &on)
	if err != nil {
		t.Fatalf("SetGlobalLimits returned error: %v", err)
	}
	want := "POST /api/v2/transfer/setDownloadLimit,GET /api/v2/transfer/speedLimitsMode,POST /api/v2/transfer/toggleSpeedLimitsMode,GET /api/v2/transfer/info,GET /api/v2/transfer/speedLimitsMode"
	if got := strings.Join(tail(f), ","); got != want {
		t.Errorf("requests = %s, want %s", got, want)
	}
	if info.DownloadLimit != 4096 {
		t.Errorf("returned info = %+v, want the refreshed limits", info)
	}

	// Asking for the mode it is already in must not toggle it off again.
	f.paths = nil
	off := false
	if _, err := QBittorrentSetGlobalLimits(context.Background(), qbitClient(f), nil, nil, &off); err != nil {
		t.Fatalf("SetGlobalLimits returned error: %v", err)
	}
	for _, p := range f.paths {
		if strings.Contains(p, "toggleSpeedLimitsMode") {
			t.Errorf("toggled alternative mode although it already matched: %v", f.paths)
		}
	}
}

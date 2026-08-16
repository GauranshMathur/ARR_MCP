package arr

import (
	"context"
	"encoding/json"
	"testing"
)

// A single custom format carries its whole specification tree; the live Sonarr
// returns 283 KB for /customformat. Only the name and rule count are useful in
// a listing, and returning the rest would swamp the model's context.
func TestListCustomFormatsDropsSpecificationBodies(t *testing.T) {
	srv, got := fakeService(t, 200, `[{
	  "id":1,"name":"DV HDR10+","includeCustomFormatWhenRenaming":false,
	  "specifications":[
	    {"name":"a","implementation":"ReleaseTitleSpecification","fields":[{"name":"value","value":"^(?=.*x)"}]},
	    {"name":"b","implementation":"LanguageSpecification","fields":[]}
	  ]
	}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	formats, err := ListCustomFormats(context.Background(), c)
	if err != nil {
		t.Fatalf("ListCustomFormats returned error: %v", err)
	}
	if got.path != "/api/v3/customformat" {
		t.Errorf("path = %q, want /api/v3/customformat", got.path)
	}
	if len(formats) != 1 || formats[0].Name != "DV HDR10+" {
		t.Fatalf("formats = %+v, want one named DV HDR10+", formats)
	}
	if formats[0].SpecificationCount != 2 {
		t.Errorf("specificationCount = %d, want 2", formats[0].SpecificationCount)
	}
	if encoded, _ := json.Marshal(formats); contains(string(encoded), "ReleaseTitleSpecification") {
		t.Errorf("specification bodies leaked into the result: %s", encoded)
	}
}

func TestListDelayProfilesReturnsProtocolDelays(t *testing.T) {
	srv, got := fakeService(t, 200, `[{
	  "enableUsenet":true,"enableTorrent":true,"preferredProtocol":"usenet",
	  "usenetDelay":0,"torrentDelay":30,"bypassIfHighestQuality":true,
	  "bypassIfAboveCustomFormatScore":false,"minimumCustomFormatScore":0,
	  "order":2147483647,"tags":[1,2],"id":1
	}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	profiles, err := ListDelayProfiles(context.Background(), c)
	if err != nil {
		t.Fatalf("ListDelayProfiles returned error: %v", err)
	}
	if got.path != "/api/v3/delayprofile" {
		t.Errorf("path = %q, want /api/v3/delayprofile", got.path)
	}
	if len(profiles) != 1 || profiles[0].TorrentDelay != 30 || profiles[0].PreferredProtocol != "usenet" {
		t.Errorf("profiles = %+v, want torrentDelay 30 preferring usenet", profiles)
	}
}

func TestListReleaseProfilesReturnsTerms(t *testing.T) {
	srv, got := fakeService(t, 200, `[{
	  "id":2,"name":"No x265","enabled":true,
	  "required":["proper"],"ignored":["x265","hevc"],"indexerId":0,"tags":[]
	}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	profiles, err := ListReleaseProfiles(context.Background(), c)
	if err != nil {
		t.Fatalf("ListReleaseProfiles returned error: %v", err)
	}
	if got.path != "/api/v3/releaseprofile" {
		t.Errorf("path = %q, want /api/v3/releaseprofile", got.path)
	}
	if len(profiles) != 1 || len(profiles[0].Ignored) != 2 || profiles[0].Ignored[0] != "x265" {
		t.Errorf("profiles = %+v, want two ignored terms starting with x265", profiles)
	}
}

// Older releases sent the term list as one comma-separated string rather than
// an array. Decoding only the array shape would fail the whole call.
func TestListReleaseProfilesAcceptsCommaSeparatedTerms(t *testing.T) {
	srv, _ := fakeService(t, 200, `[{"id":2,"name":"legacy","ignored":"x265, hevc"}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	profiles, err := ListReleaseProfiles(context.Background(), c)
	if err != nil {
		t.Fatalf("ListReleaseProfiles returned error: %v", err)
	}
	if len(profiles) != 1 || len(profiles[0].Ignored) != 2 || profiles[0].Ignored[1] != "hevc" {
		t.Errorf("profiles = %+v, want the string split into two terms", profiles)
	}
}

// Every provider resource embeds a `fields` array whose values include indexer
// API keys and download client passwords. Returning it would hand the model
// credentials it has no business seeing.
func TestListIndexersNeverReturnsProviderFields(t *testing.T) {
	srv, got := fakeService(t, 200, `[{
	  "id":3,"name":"NZBgeek","implementation":"Newznab","protocol":"usenet",
	  "priority":25,"enableRss":true,"enableAutomaticSearch":true,
	  "enableInteractiveSearch":false,"supportsSearch":true,"tags":[1],
	  "fields":[{"name":"apiKey","value":"super-secret-indexer-key"}]
	}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	providers, err := ListIndexers(context.Background(), c)
	if err != nil {
		t.Fatalf("ListIndexers returned error: %v", err)
	}
	if got.path != "/api/v3/indexer" {
		t.Errorf("path = %q, want /api/v3/indexer", got.path)
	}
	if len(providers) != 1 || providers[0].Name != "NZBgeek" || providers[0].Priority != 25 {
		t.Fatalf("providers = %+v, want NZBgeek at priority 25", providers)
	}
	if !providers[0].Enabled {
		t.Error("an indexer with RSS and automatic search on must report enabled")
	}
	encoded, _ := json.Marshal(providers)
	if contains(string(encoded), "super-secret-indexer-key") {
		t.Fatalf("provider credentials leaked into the result: %s", encoded)
	}
}

func TestListDownloadClientsReportsEnableFlag(t *testing.T) {
	srv, got := fakeService(t, 200, `[{
	  "id":1,"name":"nzbget","implementation":"Nzbget","protocol":"usenet",
	  "enable":false,"priority":1,
	  "fields":[{"name":"password","value":"hunter2"}]
	}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	clients, err := ListDownloadClients(context.Background(), c)
	if err != nil {
		t.Fatalf("ListDownloadClients returned error: %v", err)
	}
	if got.path != "/api/v3/downloadclient" {
		t.Errorf("path = %q, want /api/v3/downloadclient", got.path)
	}
	if len(clients) != 1 || clients[0].Enabled {
		t.Errorf("clients = %+v, want a disabled nzbget", clients)
	}
	if encoded, _ := json.Marshal(clients); contains(string(encoded), "hunter2") {
		t.Fatalf("download client password leaked: %s", encoded)
	}
}

// Sonarr signals an active import list with enableAutomaticAdd; Radarr uses
// enabled. One projection has to understand both.
func TestListImportListsNormalisesTheEnabledFlag(t *testing.T) {
	sonarr, got := fakeService(t, 200, `[{"id":1,"name":"trakt","implementation":"TraktList","enableAutomaticAdd":true,"rootFolderPath":"/tv","qualityProfileId":4}]`)
	c := NewClient(sonarr.URL, SonarrSpec, Credentials{APIKey: "k"})

	lists, err := ListImportLists(context.Background(), c)
	if err != nil {
		t.Fatalf("ListImportLists returned error: %v", err)
	}
	if got.path != "/api/v3/importlist" {
		t.Errorf("path = %q, want /api/v3/importlist", got.path)
	}
	if len(lists) != 1 || !lists[0].Enabled || lists[0].RootFolderPath != "/tv" {
		t.Errorf("lists = %+v, want an enabled list rooted at /tv", lists)
	}

	radarr, _ := fakeService(t, 200, `[{"id":2,"name":"tmdb","implementation":"TMDbListImport","enabled":true,"enableAuto":true}]`)
	rc := NewClient(radarr.URL, RadarrSpec, Credentials{APIKey: "k"})
	lists, err = ListImportLists(context.Background(), rc)
	if err != nil {
		t.Fatalf("ListImportLists (radarr) returned error: %v", err)
	}
	if len(lists) != 1 || !lists[0].Enabled {
		t.Errorf("radarr lists = %+v, want enabled", lists)
	}
}

func TestListNotificationsReturnsConnections(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"id":1,"name":"Discord","implementation":"Discord","onGrab":true,"fields":[{"name":"webHookUrl","value":"https://discord/secret-hook"}]}]`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	conns, err := ListNotifications(context.Background(), c)
	if err != nil {
		t.Fatalf("ListNotifications returned error: %v", err)
	}
	if got.path != "/api/v3/notification" {
		t.Errorf("path = %q, want /api/v3/notification", got.path)
	}
	if len(conns) != 1 || conns[0].Name != "Discord" {
		t.Fatalf("connections = %+v, want Discord", conns)
	}
	if encoded, _ := json.Marshal(conns); contains(string(encoded), "secret-hook") {
		t.Fatalf("notification webhook leaked: %s", encoded)
	}
}

// The two services model the same setting with different JSON types:
// Sonarr sends colonReplacementFormat as an integer, Radarr as a string. The
// shared struct must decode both payloads rather than one of them.
func TestGetNamingConfigDecodesBothServices(t *testing.T) {
	sonarr, got := fakeService(t, 200, `{
	  "id":1,"renameEpisodes":true,"replaceIllegalCharacters":false,
	  "colonReplacementFormat":4,"multiEpisodeStyle":5,
	  "standardEpisodeFormat":"{Series Title} - S{season:00}E{episode:00}",
	  "seriesFolderFormat":"{Series TitleYear}"
	}`)
	c := NewClient(sonarr.URL, SonarrSpec, Credentials{APIKey: "k"})

	cfg, err := GetNamingConfig(context.Background(), c)
	if err != nil {
		t.Fatalf("GetNamingConfig returned error: %v", err)
	}
	if got.path != "/api/v3/config/naming" {
		t.Errorf("path = %q, want /api/v3/config/naming", got.path)
	}
	if cfg.SeriesFolderFormat != "{Series TitleYear}" {
		t.Errorf("config = %+v, want the series folder format", cfg)
	}

	radarr, _ := fakeService(t, 200, `{
	  "id":1,"renameMovies":true,"colonReplacementFormat":"smart",
	  "standardMovieFormat":"{Movie CleanTitle}","movieFolderFormat":"{Movie CleanTitle} ({Release Year})"
	}`)
	rc := NewClient(radarr.URL, RadarrSpec, Credentials{APIKey: "k"})
	cfg, err = GetNamingConfig(context.Background(), rc)
	if err != nil {
		t.Fatalf("GetNamingConfig (radarr) returned error: %v", err)
	}
	if cfg.MovieFolderFormat != "{Movie CleanTitle} ({Release Year})" {
		t.Errorf("config = %+v, want the movie folder format", cfg)
	}
}

func TestListQualityDefinitionsReturnsSizeLimits(t *testing.T) {
	srv, got := fakeService(t, 200, `[
	  {"quality":{"id":3,"name":"WEBDL-1080p","source":"web","resolution":1080},
	   "title":"WEBDL-1080p","weight":5,"minSize":12.5,"maxSize":199.9,"preferredSize":95,"id":9},
	  {"quality":{"id":18,"name":"WEBDL-2160p","resolution":2160},
	   "title":"WEBDL-2160p","weight":6,"minSize":35,"maxSize":null,"id":10}
	]`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	defs, err := ListQualityDefinitions(context.Background(), c)
	if err != nil {
		t.Fatalf("ListQualityDefinitions returned error: %v", err)
	}
	if got.path != "/api/v3/qualitydefinition" {
		t.Errorf("path = %q, want /api/v3/qualitydefinition", got.path)
	}
	if len(defs) != 2 || defs[0].Quality != "WEBDL-1080p" || defs[0].Resolution != 1080 {
		t.Fatalf("definitions = %+v, want WEBDL-1080p at 1080", defs)
	}
	if defs[0].MaxSize == nil || *defs[0].MaxSize != 199.9 {
		t.Errorf("maxSize = %v, want 199.9", defs[0].MaxSize)
	}
	// A null maxSize means "no upper limit", which is not the same as zero.
	if defs[1].MaxSize != nil {
		t.Errorf("maxSize = %v, want nil for an unlimited definition", *defs[1].MaxSize)
	}
}

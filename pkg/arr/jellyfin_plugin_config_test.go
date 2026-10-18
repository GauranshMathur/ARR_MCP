package arr

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const (
	pcIntroSkipper = "c83d86bba1e04c35a113e2101cf4ee6b"
	pcEnhanced     = "f69e946a4b3c4e9a8f0a8d7c1b2c4d9b"
	pcTweaks       = "dfee382801df49df85b15c2b75e5ea1a"
	pcLDAP         = "958aad6637844d2ab89aa7b6fab6e25c"
)

func TestJellyfinPluginConfigReturnsAllowlistedKeysOnly(t *testing.T) {
	srv, got := fakeService(t, 200, `{"AutoSkipIntro":true,"AutoSkipOutro":false,"PauseScreenDelaySeconds":5,
	  "JellyseerrApiKey":"sk-live-0123456789","SonarrInstances":"[{\"ApiKey\":\"abc\"}]","AnalyticsInstallSecret":"s3cr3t"}`)

	cfg, err := JellyfinPluginConfig(context.Background(), jellyfinClient(srv.URL), pcEnhanced)
	if err != nil {
		t.Fatalf("JellyfinPluginConfig: %v", err)
	}
	if got.method != "GET" || got.path != "/Plugins/"+pcEnhanced+"/Configuration" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	wantJellyfinAuth(t, got)
	if cfg.Plugin != "Jellyfin Enhanced" || len(cfg.Settings) != 3 || cfg.Settings["AutoSkipIntro"] != true ||
		cfg.Settings["PauseScreenDelaySeconds"] != float64(5) {
		t.Errorf("config = %+v", cfg)
	}
	if strings.Join(cfg.Withheld, ",") != "AnalyticsInstallSecret,JellyseerrApiKey,SonarrInstances" {
		t.Errorf("withheld = %v, want the three non-allowlisted keys, sorted", cfg.Withheld)
	}
}

// A plugin that is not on the allowlist is refused before its configuration
// is fetched, so its secrets never enter this process.
func TestJellyfinPluginConfigRefusesAPluginNotOnTheAllowlist(t *testing.T) {
	srv, got := fakeService(t, 200, `{"LdapBindPassword":"hunter2"}`)
	_, err := JellyfinPluginConfig(context.Background(), jellyfinClient(srv.URL), pcLDAP)
	if err == nil || !strings.Contains(err.Error(), "Intro Skipper ("+pcIntroSkipper+")") {
		t.Errorf("err = %v, want a refusal naming the allowlisted plugins", err)
	}
	if len(got.paths) != 0 {
		t.Errorf("a non-allowlisted plugin's configuration was fetched: %v", got.paths)
	}
}

// An allowlisted key whose value is not a scalar or a list of scalars is
// withheld: a later plugin version could nest credentials under it.
func TestJellyfinPluginConfigWithholdsNestedValues(t *testing.T) {
	srv, _ := fakeService(t, 200, `{"MaxDaysNextUp":30,"HomeSections":["resume","nextup"],"LatestItemsExcludes":[],
	  "SubtitleAppearance":{"Font":"x"},"BitrateOverrides":[{"Token":"t"}],"DesktopDisplayMode":"Desktop"}`)

	cfg, err := JellyfinPluginConfig(context.Background(), jellyfinClient(srv.URL), pcTweaks)
	if err != nil {
		t.Fatalf("JellyfinPluginConfig: %v", err)
	}
	for _, key := range []string{"MaxDaysNextUp", "HomeSections", "LatestItemsExcludes", "DesktopDisplayMode"} {
		if _, ok := cfg.Settings[key]; !ok {
			t.Errorf("%s missing from %v", key, cfg.Settings)
		}
	}
	if strings.Join(cfg.Withheld, ",") != "BitrateOverrides,SubtitleAppearance" {
		t.Errorf("withheld = %v, want the two nested values", cfg.Withheld)
	}
}

// Matching ignores case, as Jellyfin's ids do.
func TestJellyfinPluginConfigAcceptsAnyIDCase(t *testing.T) {
	srv, _ := fakeService(t, 200, `{"MaximumIntroDuration":120}`)
	cfg, err := JellyfinPluginConfig(context.Background(), jellyfinClient(srv.URL), strings.ToUpper(pcIntroSkipper))
	if err != nil || cfg.Settings["MaximumIntroDuration"] != float64(120) {
		t.Errorf("config = %+v, err = %v", cfg, err)
	}
}

func TestJellyfinPluginConfigErrors(t *testing.T) {
	srv, got := fakeService(t, 404, `{"title":"Not Found"}`)
	c := jellyfinClient(srv.URL)
	if _, err := JellyfinPluginConfig(context.Background(), c, "../Plugins"); err == nil {
		t.Error("a path-shaped id was accepted")
	}
	if len(got.paths) != 0 {
		t.Errorf("requests = %v, want none for a bad id", got.paths)
	}
	_, err := JellyfinPluginConfig(context.Background(), c, pcIntroSkipper)
	var se *StatusError
	if !errors.As(err, &se) || se.Status != 404 {
		t.Errorf("err = %v, want a 404 StatusError", err)
	}
}

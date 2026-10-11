package config

import (
	"reflect"
	"strings"
	"testing"
)

func synologyWithPaths(paths string) string {
	return `
services:
  synology:
    - name: nas
      url: http://nas:5000
      username: svc
      password: pw
` + paths
}

func TestSynologyAllowedPathsAreLoadedAndNormalised(t *testing.T) {
	p := writeCfg(t, synologyWithPaths("      allowedPaths: [\"/NAS\", \"/NAS//Scratch/\", \"/kube/./x\"]\n"))
	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := c.Services["synology"][0].AllowedPaths
	want := []string{"/NAS", "/NAS/Scratch", "/kube/x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllowedPaths = %v, want %v", got, want)
	}
}

func TestSynologyAllowedPathsMayBeOmittedOrEmpty(t *testing.T) {
	for _, extra := range []string{"", "      allowedPaths: []\n"} {
		c, err := Load(writeCfg(t, synologyWithPaths(extra)))
		if err != nil {
			t.Fatalf("Load(%q): %v", extra, err)
		}
		if n := len(c.Services["synology"][0].AllowedPaths); n != 0 {
			t.Errorf("AllowedPaths = %d entries, want none", n)
		}
	}
}

func TestSynologyAllowedPathsRejectBadEntries(t *testing.T) {
	for entry, why := range map[string]string{
		`"NAS"`:          "not absolute",
		`""`:             "empty",
		`"/"`:            "the root",
		`"//"`:           "the root",
		`"/NAS/../kube"`: "traversal",
		`".."`:           "traversal",
		`"/NAS/.."`:      "traversal",
		`"/NAS\\x"`:      "backslash",
	} {
		_, err := Load(writeCfg(t, synologyWithPaths("      allowedPaths: ["+entry+"]\n")))
		if err == nil {
			t.Errorf("%s (%s) was accepted", entry, why)
			continue
		}
		if !strings.Contains(err.Error(), "allowedPaths") {
			t.Errorf("%s: error %q does not name allowedPaths", entry, err)
		}
	}
}

func TestAllowedPathsAreOnlyForSynology(t *testing.T) {
	p := writeCfg(t, `
services:
  sonarr:
    - name: main
      url: http://a:8989
      apiKey: k
      allowedPaths: ["/NAS"]
`)
	if _, err := Load(p); err == nil || !strings.Contains(err.Error(), "allowedPaths") {
		t.Errorf("err = %v, want allowedPaths rejected on sonarr", err)
	}
}

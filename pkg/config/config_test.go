package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCfg writes a temporary YAML config and returns its path.
func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return p
}

func TestResolveExplicitInstanceName(t *testing.T) {
	c := &Config{Services: map[string][]Instance{
		"sonarr": {
			{Name: "main", URL: "http://a", APIKey: "k1", Default: true},
			{Name: "anime", URL: "http://b", APIKey: "k2"},
		},
	}}

	got, err := c.Resolve("sonarr", "anime")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got.URL != "http://b" {
		t.Errorf("URL = %q, want %q", got.URL, "http://b")
	}
}

func TestResolveOmittedUsesDefaultInstance(t *testing.T) {
	c := &Config{Services: map[string][]Instance{
		"sonarr": {
			{Name: "main", URL: "http://a", APIKey: "k1"},
			{Name: "anime", URL: "http://b", APIKey: "k2", Default: true},
		},
	}}

	got, err := c.Resolve("sonarr", "")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got.Name != "anime" {
		t.Errorf("Name = %q, want %q", got.Name, "anime")
	}
}

func TestResolveOmittedWithSingleInstanceUsesIt(t *testing.T) {
	c := &Config{Services: map[string][]Instance{
		"prowlarr": {{Name: "only", URL: "http://p", APIKey: "k"}},
	}}

	got, err := c.Resolve("prowlarr", "")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got.Name != "only" {
		t.Errorf("Name = %q, want %q", got.Name, "only")
	}
}

func TestResolveAmbiguousListsValidNames(t *testing.T) {
	c := &Config{Services: map[string][]Instance{
		"sonarr": {
			{Name: "main", URL: "http://a", APIKey: "k1"},
			{Name: "anime", URL: "http://b", APIKey: "k2"},
		},
	}}

	_, err := c.Resolve("sonarr", "")
	if err == nil {
		t.Fatal("expected an error when no default and multiple instances, got nil")
	}
	for _, want := range []string{"main", "anime"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention valid instance %q", err, want)
		}
	}
}

func TestResolveUnknownNameListsValidNames(t *testing.T) {
	c := &Config{Services: map[string][]Instance{
		"sonarr": {{Name: "main", URL: "http://a", APIKey: "k"}},
	}}

	_, err := c.Resolve("sonarr", "nope")
	if err == nil {
		t.Fatal("expected an error for unknown instance name, got nil")
	}
	if !strings.Contains(err.Error(), "main") {
		t.Errorf("error %q does not list valid instance %q", err, "main")
	}
}

func TestResolveUnconfiguredService(t *testing.T) {
	c := &Config{Services: map[string][]Instance{}}

	if _, err := c.Resolve("sonarr", ""); err == nil {
		t.Fatal("expected an error for unconfigured service, got nil")
	}
}

func TestInstanceNamesPreservesConfigOrder(t *testing.T) {
	c := &Config{Services: map[string][]Instance{
		"sonarr": {
			{Name: "main", URL: "http://a", APIKey: "k1"},
			{Name: "anime", URL: "http://b", APIKey: "k2"},
		},
	}}

	got := c.InstanceNames("sonarr")
	want := []string{"main", "anime"}
	if len(got) != len(want) {
		t.Fatalf("InstanceNames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("InstanceNames[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

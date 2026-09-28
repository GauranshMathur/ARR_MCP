package config

import (
	"strings"
	"testing"
)

func TestUserPassServicesRequireUsernameAndPassword(t *testing.T) {
	p := writeCfg(t, `
services:
  qbittorrent:
    - name: main
      url: http://q:8080
      username: admin
`)

	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "qbittorrent.main: missing password") {
		t.Fatalf("Load error = %v, want missing password", err)
	}
}

func TestUserPassServicesRejectAPIKey(t *testing.T) {
	p := writeCfg(t, `
services:
  nzbget:
    - name: main
      url: http://n:6789
      apiKey: abc
`)

	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "nzbget.main: nzbget authenticates with username and password, not apiKey") {
		t.Fatalf("Load error = %v, want apiKey rejection", err)
	}
}

func TestAPIKeyServicesRejectUsernameAndPassword(t *testing.T) {
	p := writeCfg(t, `
services:
  sonarr:
    - name: main
      url: http://s:8989
      apiKey: abc
      username: admin
      password: pw
`)

	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "sonarr.main: sonarr authenticates with apiKey, not username and password") {
		t.Fatalf("Load error = %v, want username rejection", err)
	}
}

func TestPasswordsAreExpandedFromEnv(t *testing.T) {
	t.Setenv("QBIT_PW", "s3cret-from-env")
	p := writeCfg(t, `
services:
  qbittorrent:
    - name: main
      url: http://q:8080
      username: admin
      password: ${QBIT_PW}
`)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	inst := c.Services["qbittorrent"][0]
	if inst.Username != "admin" || inst.Password != "s3cret-from-env" {
		t.Errorf("instance = %+v, want username=admin password=s3cret-from-env", inst)
	}
}

func TestEnvFallbackBuildsUserPassInstance(t *testing.T) {
	t.Setenv("NZBGET_URL", "http://n:6789")
	t.Setenv("NZBGET_USERNAME", "nzbget")
	t.Setenv("NZBGET_PASSWORD", "tegbzn6789")

	c, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	inst, err := c.Resolve("nzbget", "")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if inst.Name != "default" || inst.Username != "nzbget" || inst.Password != "tegbzn6789" {
		t.Errorf("instance = %+v, want default/nzbget/tegbzn6789", inst)
	}
}

func TestEnvFallbackIgnoresUserPassServiceMissingPassword(t *testing.T) {
	t.Setenv("QBITTORRENT_URL", "http://q:8080")
	t.Setenv("QBITTORRENT_USERNAME", "admin")
	t.Setenv("SONARR_URL", "http://s:8989")
	t.Setenv("SONARR_API_KEY", "k")

	c, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(c.Services["qbittorrent"]) != 0 {
		t.Errorf("qbittorrent configured without a password: %+v", c.Services["qbittorrent"])
	}
}

func TestCredentialKindFor(t *testing.T) {
	cases := map[string]CredentialKind{
		"sonarr": CredentialAPIKey, "radarr": CredentialAPIKey, "prowlarr": CredentialAPIKey,
		"bazarr": CredentialAPIKey, "qbittorrent": CredentialUserPass, "nzbget": CredentialUserPass,
		"maintainerr": CredentialNone,
	}
	for svc, want := range cases {
		if got := CredentialKindFor(svc); got != want {
			t.Errorf("CredentialKindFor(%q) = %v, want %v", svc, got, want)
		}
	}
}

// Maintainerr has no authentication at all, so a url alone is a complete
// instance; demanding a key would make the service impossible to configure.
func TestUnauthenticatedServicesNeedOnlyAURL(t *testing.T) {
	p := writeCfg(t, `
services:
  maintainerr:
    - name: main
      url: http://m:6246
`)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if inst := c.Services["maintainerr"][0]; inst.URL != "http://m:6246" {
		t.Errorf("instance = %+v, want url http://m:6246", inst)
	}
}

// A credential on a service that takes none would be silently ignored, which
// reads as protection the deployment does not have.
func TestUnauthenticatedServicesRejectCredentials(t *testing.T) {
	for field, line := range map[string]string{
		"apiKey":   "apiKey: abc",
		"username": "username: admin",
		"password": "password: pw",
	} {
		t.Run(field, func(t *testing.T) {
			p := writeCfg(t, `
services:
  maintainerr:
    - name: main
      url: http://m:6246
      `+line+`
`)

			_, err := Load(p)
			if err == nil || !strings.Contains(err.Error(), "maintainerr.main: maintainerr takes no credentials") {
				t.Fatalf("Load error = %v, want credential rejection", err)
			}
		})
	}
}

func TestEnvFallbackBuildsUnauthenticatedInstanceFromURL(t *testing.T) {
	t.Setenv("MAINTAINERR_URL", "http://m:6246")

	c, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	inst, err := c.Resolve("maintainerr", "")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if inst.Name != "default" || inst.URL != "http://m:6246" {
		t.Errorf("instance = %+v, want default at http://m:6246", inst)
	}
}

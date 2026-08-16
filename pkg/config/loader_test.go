package config

import (
	"strings"
	"testing"
)

func TestLoadParsesServicesAndInstances(t *testing.T) {
	p := writeCfg(t, `
services:
  sonarr:
    - name: main
      url: http://192.168.10.12:8989
      apiKey: abc123
      default: true
    - name: anime
      url: http://192.168.10.13:8989
      apiKey: def456
`)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := len(c.Services["sonarr"]); got != 2 {
		t.Fatalf("sonarr instances = %d, want 2", got)
	}
	inst, err := c.Resolve("sonarr", "")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if inst.Name != "main" || inst.APIKey != "abc123" {
		t.Errorf("default instance = %+v, want name=main apiKey=abc123", inst)
	}
}

func TestLoadInterpolatesEnvVars(t *testing.T) {
	t.Setenv("SONARR_MAIN_KEY", "secret-from-env")
	p := writeCfg(t, `
services:
  sonarr:
    - name: main
      url: http://a:8989
      apiKey: ${SONARR_MAIN_KEY}
`)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := c.Services["sonarr"][0].APIKey; got != "secret-from-env" {
		t.Errorf("APIKey = %q, want %q", got, "secret-from-env")
	}
}

func TestLoadFailsOnUnsetEnvVar(t *testing.T) {
	p := writeCfg(t, `
services:
  sonarr:
    - name: main
      url: http://a:8989
      apiKey: ${DEFINITELY_NOT_SET_ARRMCP}
`)

	_, err := Load(p)
	if err == nil {
		t.Fatal("expected an error for unset env var, got nil")
	}
	if !strings.Contains(err.Error(), "DEFINITELY_NOT_SET_ARRMCP") {
		t.Errorf("error %q does not name the missing variable", err)
	}
}

func TestLoadRejectsDuplicateInstanceNames(t *testing.T) {
	p := writeCfg(t, `
services:
  sonarr:
    - name: main
      url: http://a:8989
      apiKey: k1
    - name: main
      url: http://b:8989
      apiKey: k2
`)

	_, err := Load(p)
	if err == nil {
		t.Fatal("expected an error for duplicate instance names, got nil")
	}
	if !strings.Contains(err.Error(), "main") {
		t.Errorf("error %q does not name the duplicate", err)
	}
}

func TestLoadRejectsMultipleDefaults(t *testing.T) {
	p := writeCfg(t, `
services:
  sonarr:
    - name: main
      url: http://a:8989
      apiKey: k1
      default: true
    - name: anime
      url: http://b:8989
      apiKey: k2
      default: true
`)

	if _, err := Load(p); err == nil {
		t.Fatal("expected an error for multiple default instances, got nil")
	}
}

func TestLoadRejectsInstanceMissingURL(t *testing.T) {
	p := writeCfg(t, `
services:
  sonarr:
    - name: main
      apiKey: k1
`)

	_, err := Load(p)
	if err == nil {
		t.Fatal("expected an error for instance without url, got nil")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("error %q does not mention the missing url field", err)
	}
}

func TestLoadRejectsUnknownService(t *testing.T) {
	p := writeCfg(t, `
services:
  notaservice:
    - name: main
      url: http://a
      apiKey: k
`)

	_, err := Load(p)
	if err == nil {
		t.Fatal("expected an error for unknown service name, got nil")
	}
	if !strings.Contains(err.Error(), "notaservice") {
		t.Errorf("error %q does not name the unknown service", err)
	}
}

func TestLoadDefaultsPermissionsToConfirmWriteDeny(t *testing.T) {
	p := writeCfg(t, `
services:
  sonarr:
    - name: main
      url: http://a:8989
      apiKey: k
`)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	perms := c.Permissions
	if perms.Mode != ModeConfirm {
		t.Errorf("Mode = %q, want %q", perms.Mode, ModeConfirm)
	}
	if perms.ConfirmScope != ScopeWrite {
		t.Errorf("ConfirmScope = %q, want %q", perms.ConfirmScope, ScopeWrite)
	}
	if perms.Fallback != FallbackDeny {
		t.Errorf("Fallback = %q, want %q", perms.Fallback, FallbackDeny)
	}
}

func TestLoadRejectsInvalidPermissionMode(t *testing.T) {
	p := writeCfg(t, `
permissions:
  mode: yolo
services:
  sonarr:
    - name: main
      url: http://a:8989
      apiKey: k
`)

	_, err := Load(p)
	if err == nil {
		t.Fatal("expected an error for invalid permission mode, got nil")
	}
	if !strings.Contains(err.Error(), "yolo") {
		t.Errorf("error %q does not name the invalid mode", err)
	}
}

func TestInstancePermissionsOverrideGlobal(t *testing.T) {
	p := writeCfg(t, `
permissions:
  mode: full
services:
  sonarr:
    - name: main
      url: http://a:8989
      apiKey: k
    - name: locked
      url: http://b:8989
      apiKey: k
      permissions:
        mode: readonly
`)

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	main, _ := c.Resolve("sonarr", "main")
	if got := c.EffectiveMode(main); got != ModeFull {
		t.Errorf("main mode = %q, want %q", got, ModeFull)
	}

	locked, _ := c.Resolve("sonarr", "locked")
	if got := c.EffectiveMode(locked); got != ModeReadOnly {
		t.Errorf("locked mode = %q, want %q", got, ModeReadOnly)
	}
}

func TestLoadFromEnvBuildsDefaultInstance(t *testing.T) {
	t.Setenv("SONARR_URL", "http://envhost:8989")
	t.Setenv("SONARR_API_KEY", "envkey")

	c, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	inst, err := c.Resolve("sonarr", "")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if inst.URL != "http://envhost:8989" || inst.APIKey != "envkey" {
		t.Errorf("instance = %+v, want url/key from env", inst)
	}
}

func TestLoadWithNoConfigAndNoEnvFails(t *testing.T) {
	_, err := Load("")
	if err == nil {
		t.Fatal("expected an error when nothing is configured, got nil")
	}
}

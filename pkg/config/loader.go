package config

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Mode controls whether mutating tools are exposed and whether they require
// confirmation before running.
type Mode string

// Supported permission modes.
const (
	// ModeReadOnly registers only read tools; mutating tools are invisible.
	ModeReadOnly Mode = "readonly"
	// ModeConfirm registers all tools but confirms before mutating ones run.
	ModeConfirm Mode = "confirm"
	// ModeFull registers all tools and runs them without confirmation.
	ModeFull Mode = "full"
)

// Scope selects which access tiers require confirmation under ModeConfirm.
type Scope string

// Supported confirmation scopes.
const (
	// ScopeWrite confirms both write and destructive tools.
	ScopeWrite Scope = "write"
	// ScopeDestructive confirms only destructive tools.
	ScopeDestructive Scope = "destructive"
)

// Fallback decides what happens when the client cannot prompt the user.
type Fallback string

// Supported fallback behaviours.
const (
	// FallbackDeny refuses the call when confirmation is impossible.
	FallbackDeny Fallback = "deny"
	// FallbackAllow runs the call when confirmation is impossible.
	FallbackAllow Fallback = "allow"
)

// KnownServices lists the services this build can expose tools for. Config
// referencing anything else is rejected rather than silently ignored.
var KnownServices = []string{"sonarr", "radarr", "prowlarr"}

// Permissions describes how mutating tools are treated.
type Permissions struct {
	Mode         Mode     `yaml:"mode"`
	ConfirmScope Scope    `yaml:"confirmScope"`
	Fallback     Fallback `yaml:"fallback"`
}

// ServerConfig holds transport and logging settings.
type ServerConfig struct {
	Transport string `yaml:"transport"`
	Addr      string `yaml:"addr"`
	LogLevel  string `yaml:"logLevel"`
}

var envRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnv replaces ${VAR} references with their environment values. An unset
// variable is an error: an empty API key would otherwise surface much later as
// a confusing 401 from the upstream service.
func expandEnv(field, value string) (string, error) {
	var missing []string
	out := envRef.ReplaceAllStringFunc(value, func(m string) string {
		name := envRef.FindStringSubmatch(m)[1]
		v, ok := os.LookupEnv(name)
		if !ok || v == "" {
			missing = append(missing, name)
			return ""
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("%s references unset environment variable(s): %s",
			field, strings.Join(missing, ", "))
	}
	return out, nil
}

// Load reads configuration from a YAML file. When path is empty it falls back
// to building a single default instance per service from environment variables.
func Load(path string) (*Config, error) {
	c := &Config{
		Server:      ServerConfig{Transport: "stdio", Addr: "0.0.0.0:8080", LogLevel: "info"},
		Permissions: Permissions{Mode: ModeConfirm, ConfirmScope: ScopeWrite, Fallback: FallbackDeny},
		Services:    map[string][]Instance{},
	}

	if path == "" {
		if err := c.loadFromEnv(); err != nil {
			return nil, err
		}
	} else {
		// The path is the operator's own --config flag, not untrusted input;
		// choosing which local file to load is the purpose of the flag.
		raw, err := os.ReadFile(path) // #nosec G304
		if err != nil {
			return nil, fmt.Errorf("reading config %s: %w", path, err)
		}
		if err := yaml.Unmarshal(raw, c); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", path, err)
		}
	}

	if err := c.expand(); err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// loadFromEnv builds one instance per service from <SERVICE>_URL/<SERVICE>_API_KEY,
// covering the single-instance docker-compose quickstart with no config file.
func (c *Config) loadFromEnv() error {
	for _, svc := range KnownServices {
		prefix := strings.ToUpper(svc)
		url := os.Getenv(prefix + "_URL")
		key := os.Getenv(prefix + "_API_KEY")
		if url == "" || key == "" {
			continue
		}
		c.Services[svc] = []Instance{{Name: "default", URL: url, APIKey: key, Default: true}}
	}
	if len(c.Services) == 0 {
		return fmt.Errorf("no configuration found: pass --config, or set <SERVICE>_URL and "+
			"<SERVICE>_API_KEY for at least one of: %s", strings.Join(KnownServices, ", "))
	}
	return nil
}

// expand resolves ${VAR} references in every instance field that accepts them.
func (c *Config) expand() error {
	for svc, instances := range c.Services {
		for i := range instances {
			inst := &instances[i]
			var err error
			if inst.URL, err = expandEnv(fmt.Sprintf("%s.%s.url", svc, inst.Name), inst.URL); err != nil {
				return err
			}
			if inst.APIKey, err = expandEnv(fmt.Sprintf("%s.%s.apiKey", svc, inst.Name), inst.APIKey); err != nil {
				return err
			}
		}
		c.Services[svc] = instances
	}
	return nil
}

// validate rejects configuration that would fail confusingly at request time.
func (c *Config) validate() error {
	if err := validatePermissions(&c.Permissions, "permissions"); err != nil {
		return err
	}

	known := map[string]bool{}
	for _, s := range KnownServices {
		known[s] = true
	}

	services := make([]string, 0, len(c.Services))
	for svc := range c.Services {
		services = append(services, svc)
	}
	sort.Strings(services)

	for _, svc := range services {
		if !known[svc] {
			return fmt.Errorf("unknown service %q; supported services: %s",
				svc, strings.Join(KnownServices, ", "))
		}

		seen := map[string]bool{}
		defaults := 0
		for i := range c.Services[svc] {
			inst := &c.Services[svc][i]
			if inst.Name == "" {
				return fmt.Errorf("%s: every instance needs a name", svc)
			}
			if seen[inst.Name] {
				return fmt.Errorf("%s: duplicate instance name %q", svc, inst.Name)
			}
			seen[inst.Name] = true

			if inst.URL == "" {
				return fmt.Errorf("%s.%s: missing url", svc, inst.Name)
			}
			if inst.APIKey == "" {
				return fmt.Errorf("%s.%s: missing apiKey", svc, inst.Name)
			}
			if inst.Default {
				defaults++
			}
			if inst.Permissions != nil {
				if err := validatePermissions(inst.Permissions,
					fmt.Sprintf("%s.%s.permissions", svc, inst.Name)); err != nil {
					return err
				}
			}
		}
		if defaults > 1 {
			return fmt.Errorf("%s: only one instance may be marked default", svc)
		}
	}

	if len(c.Services) == 0 {
		return fmt.Errorf("no services configured; supported services: %s",
			strings.Join(KnownServices, ", "))
	}
	return nil
}

// validatePermissions checks a permission block and fills unset fields with the
// same defaults Load applies globally.
func validatePermissions(p *Permissions, field string) error {
	switch p.Mode {
	case "":
		p.Mode = ModeConfirm
	case ModeReadOnly, ModeConfirm, ModeFull:
	default:
		return fmt.Errorf("%s.mode: unknown mode %q; want one of: %s, %s, %s",
			field, p.Mode, ModeReadOnly, ModeConfirm, ModeFull)
	}

	switch p.ConfirmScope {
	case "":
		p.ConfirmScope = ScopeWrite
	case ScopeWrite, ScopeDestructive:
	default:
		return fmt.Errorf("%s.confirmScope: unknown scope %q; want one of: %s, %s",
			field, p.ConfirmScope, ScopeWrite, ScopeDestructive)
	}

	switch p.Fallback {
	case "":
		p.Fallback = FallbackDeny
	case FallbackDeny, FallbackAllow:
	default:
		return fmt.Errorf("%s.fallback: unknown fallback %q; want one of: %s, %s",
			field, p.Fallback, FallbackDeny, FallbackAllow)
	}
	return nil
}

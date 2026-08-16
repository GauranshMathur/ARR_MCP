// Package config loads ARR-MCP configuration and resolves service instances.
package config

import (
	"fmt"
	"strings"
)

// Instance is a single configured endpoint for a service.
type Instance struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	APIKey  string `yaml:"apiKey"`
	Default bool   `yaml:"default"`
	// Permissions optionally overrides the global policy for this instance.
	Permissions *Permissions `yaml:"permissions"`
}

// Config is the fully resolved server configuration.
type Config struct {
	Server      ServerConfig          `yaml:"server"`
	Permissions Permissions           `yaml:"permissions"`
	Services    map[string][]Instance `yaml:"services"`
}

// InstanceNames returns configured instance names for service, in config order.
func (c *Config) InstanceNames(service string) []string {
	instances := c.Services[service]
	names := make([]string, 0, len(instances))
	for _, inst := range instances {
		names = append(names, inst.Name)
	}
	return names
}

// ConfiguredServices returns the services that have at least one instance.
func (c *Config) ConfiguredServices() []string {
	out := make([]string, 0, len(c.Services))
	for _, svc := range KnownServices {
		if len(c.Services[svc]) > 0 {
			out = append(out, svc)
		}
	}
	return out
}

// EffectiveMode returns the permission mode governing inst, preferring the
// instance override over the global policy.
func (c *Config) EffectiveMode(inst *Instance) Mode {
	if inst != nil && inst.Permissions != nil && inst.Permissions.Mode != "" {
		return inst.Permissions.Mode
	}
	return c.Permissions.Mode
}

// EffectivePermissions returns the full permission policy governing inst.
func (c *Config) EffectivePermissions(inst *Instance) Permissions {
	if inst != nil && inst.Permissions != nil {
		return *inst.Permissions
	}
	return c.Permissions
}

// Resolve returns the instance of service selected by name. An empty name
// selects the instance marked default, or the sole instance when only one is
// configured. Errors name the valid instances so the caller can correct itself.
func (c *Config) Resolve(service, name string) (*Instance, error) {
	instances := c.Services[service]
	if len(instances) == 0 {
		return nil, fmt.Errorf("service %s is not configured", service)
	}

	if name != "" {
		for i := range instances {
			if instances[i].Name == name {
				return &instances[i], nil
			}
		}
		return nil, fmt.Errorf("unknown %s instance %q; configured instances: %s",
			service, name, strings.Join(c.InstanceNames(service), ", "))
	}

	for i := range instances {
		if instances[i].Default {
			return &instances[i], nil
		}
	}

	if len(instances) == 1 {
		return &instances[0], nil
	}

	return nil, fmt.Errorf("%s has multiple instances and no default; specify one of: %s",
		service, strings.Join(c.InstanceNames(service), ", "))
}

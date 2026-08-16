package server

import "runtime/debug"

// devVersion is reported when neither a build-time value nor a module version
// is available, which is the case for a plain `go build` in a working tree.
const devVersion = "dev"

// ldflagsVersion is set at build time with
// -ldflags "-X github.com/GauranshMathur/ARR_MCP/pkg/server.ldflagsVersion=v1.2.3".
// It must have no initializer: the linker can only rewrite string variables
// that are not assigned a computed value.
var ldflagsVersion string

// Version is the version this server reports over MCP and via --version.
var Version = resolveVersion()

// resolveVersion reads the module version recorded by `go install module@version`
// and defers to any build-time override.
func resolveVersion() string {
	var buildInfoVersion string
	if info, ok := debug.ReadBuildInfo(); ok {
		buildInfoVersion = info.Main.Version
	}
	return pickVersion(ldflagsVersion, buildInfoVersion)
}

// pickVersion applies the precedence rules: an explicit build-time value first,
// then the module version, then a placeholder. "(devel)" is what Go records for
// a build from a working tree, so it is treated as absent rather than as a
// version.
func pickVersion(ldflags, buildInfo string) string {
	if ldflags != "" {
		return ldflags
	}
	if buildInfo != "" && buildInfo != "(devel)" {
		return buildInfo
	}
	return devVersion
}

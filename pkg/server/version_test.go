package server

import "testing"

// A build-time -ldflags value always wins: it is the most specific statement of
// what was built, and it is what the release image sets.
func TestLdflagsVersionWinsOverBuildInfo(t *testing.T) {
	if got := pickVersion("v1.2.3", "v9.9.9"); got != "v1.2.3" {
		t.Errorf("pickVersion = %q, want %q", got, "v1.2.3")
	}
}

// `go install module@version` records the tag in build info, which is the only
// version signal a source install has.
func TestBuildInfoVersionUsedWhenLdflagsAbsent(t *testing.T) {
	if got := pickVersion("", "v1.0.0"); got != "v1.0.0" {
		t.Errorf("pickVersion = %q, want %q", got, "v1.0.0")
	}
}

// Building from a working tree reports "(devel)", which is not a version.
func TestDevelBuildInfoFallsBackToDev(t *testing.T) {
	if got := pickVersion("", "(devel)"); got != devVersion {
		t.Errorf("pickVersion = %q, want %q", got, devVersion)
	}
}

func TestMissingBuildInfoFallsBackToDev(t *testing.T) {
	if got := pickVersion("", ""); got != devVersion {
		t.Errorf("pickVersion = %q, want %q", got, devVersion)
	}
}

// The running binary must always report something usable.
func TestVersionIsNeverEmpty(t *testing.T) {
	if Version == "" {
		t.Error("Version is empty")
	}
}

package logger

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Under the stdio transport, stdout carries the JSON-RPC stream. A log line
// written there corrupts the protocol and kills the MCP session, so the logger
// must never default to stdout.
func TestLoggerDefaultsToStderr(t *testing.T) {
	l := New("info", "test")
	if l.writer != os.Stderr {
		t.Errorf("default writer = %v, want os.Stderr", l.writer)
	}
}

func TestLoggerSuppressesBelowConfiguredLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New("warn", "test")
	l.SetOutput(&buf)

	l.Info("should not appear")
	l.Warn("should appear")

	out := buf.String()
	if strings.Contains(out, "should not appear") {
		t.Errorf("info message logged at warn level: %q", out)
	}
	if !strings.Contains(out, "should appear") {
		t.Errorf("warn message missing: %q", out)
	}
}

func TestLoggerIncludesLevelAndComponent(t *testing.T) {
	var buf bytes.Buffer
	l := New("debug", "sonarr")
	l.SetOutput(&buf)

	l.Error("boom %d", 42)

	out := buf.String()
	for _, want := range []string{"ERROR", "sonarr", "boom 42"} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q missing %q", out, want)
		}
	}
}

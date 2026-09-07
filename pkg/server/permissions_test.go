package server

import (
	"errors"
	"strings"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeConfirmer stands in for an MCP client's elicitation support. answered
// models the second pass, where the client has retried the call carrying the
// user's decision; leaving it false models the first pass.
type fakeConfirmer struct {
	approve     bool
	answered    bool
	unsupported bool
	prompts     []string
}

func (f *fakeConfirmer) Decision() (bool, bool, error) {
	return f.approve, f.answered, nil
}

func (f *fakeConfirmer) Ask(prompt string) (*mcp.CallToolResult, error) {
	f.prompts = append(f.prompts, prompt)
	if f.unsupported {
		return nil, ErrConfirmUnsupported
	}
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{confirmationID: confirmElicitParams(prompt)},
	}, nil
}

func gate(mode config.Mode, scope config.Scope, fb config.Fallback) Gate {
	return Gate{Perms: config.Permissions{Mode: mode, ConfirmScope: scope, Fallback: fb}}
}

func TestReadOnlyModeRegistersOnlyReadTools(t *testing.T) {
	g := gate(config.ModeReadOnly, config.ScopeWrite, config.FallbackDeny)

	if !g.Registers(AccessRead) {
		t.Error("read tools must be registered in readonly mode")
	}
	if g.Registers(AccessWrite) {
		t.Error("write tools must not be registered in readonly mode")
	}
	if g.Registers(AccessDestructive) {
		t.Error("destructive tools must not be registered in readonly mode")
	}
}

func TestFullModeRegistersEveryTool(t *testing.T) {
	g := gate(config.ModeFull, config.ScopeWrite, config.FallbackDeny)

	for _, a := range []Access{AccessRead, AccessWrite, AccessDestructive} {
		if !g.Registers(a) {
			t.Errorf("access %v must be registered in full mode", a)
		}
	}
}

func TestFullModeRunsWithoutConfirmation(t *testing.T) {
	g := gate(config.ModeFull, config.ScopeWrite, config.FallbackDeny)
	c := &fakeConfirmer{approve: false}

	res, err := g.Authorize(c, "sonarr_add_series", AccessWrite)
	if err != nil {
		t.Fatalf("Authorize returned error in full mode: %v", err)
	}
	if res != nil {
		t.Error("full mode asked the client to confirm")
	}
	if len(c.prompts) != 0 {
		t.Errorf("full mode prompted for confirmation: %v", c.prompts)
	}
}

func TestReadToolsNeverPrompt(t *testing.T) {
	g := gate(config.ModeConfirm, config.ScopeWrite, config.FallbackDeny)
	c := &fakeConfirmer{approve: false}

	res, err := g.Authorize(c, "sonarr_list_series", AccessRead)
	if err != nil {
		t.Fatalf("Authorize returned error for a read tool: %v", err)
	}
	if res != nil {
		t.Error("a read tool asked the client to confirm")
	}
	if len(c.prompts) != 0 {
		t.Errorf("read tool prompted for confirmation: %v", c.prompts)
	}
}

// The first pass asks and stops; the retry carrying approval runs the call.
func TestConfirmModeRunsWriteWhenApproved(t *testing.T) {
	g := gate(config.ModeConfirm, config.ScopeWrite, config.FallbackDeny)
	c := &fakeConfirmer{approve: true}

	res, err := g.Authorize(c, "sonarr_add_series", AccessWrite)
	if err != nil {
		t.Fatalf("Authorize returned error on the first pass: %v", err)
	}
	if res == nil {
		t.Fatal("first pass must ask the client to confirm, got no input request")
	}
	if _, ok := res.InputRequests[confirmationID]; !ok {
		t.Errorf("input requests %v do not carry %q", res.InputRequests, confirmationID)
	}
	if len(c.prompts) != 1 {
		t.Fatalf("prompts = %d, want 1", len(c.prompts))
	}
	if !strings.Contains(c.prompts[0], "sonarr_add_series") {
		t.Errorf("prompt %q does not name the tool", c.prompts[0])
	}

	c.answered = true
	res, err = g.Authorize(c, "sonarr_add_series", AccessWrite)
	if err != nil {
		t.Fatalf("Authorize returned error after approval: %v", err)
	}
	if res != nil {
		t.Error("the approved retry asked a second time")
	}
	if len(c.prompts) != 1 {
		t.Errorf("prompts = %d after the retry, want 1", len(c.prompts))
	}
}

func TestConfirmModeBlocksWriteWhenDeclined(t *testing.T) {
	g := gate(config.ModeConfirm, config.ScopeWrite, config.FallbackDeny)
	c := &fakeConfirmer{approve: false, answered: true}

	_, err := g.Authorize(c, "sonarr_add_series", AccessWrite)
	if err == nil {
		t.Fatal("expected an error when the user declines, got nil")
	}
	if !errors.Is(err, ErrDeclined) {
		t.Errorf("error = %v, want ErrDeclined", err)
	}
}

// With confirmScope=destructive, ordinary writes run unprompted but deletes ask.
func TestDestructiveScopeOnlyPromptsForDestructiveTools(t *testing.T) {
	g := gate(config.ModeConfirm, config.ScopeDestructive, config.FallbackDeny)

	c := &fakeConfirmer{approve: true}
	res, err := g.Authorize(c, "sonarr_add_series", AccessWrite)
	if err != nil {
		t.Fatalf("Authorize returned error for write under destructive scope: %v", err)
	}
	if res != nil {
		t.Error("write asked to confirm under destructive scope")
	}
	if len(c.prompts) != 0 {
		t.Errorf("write prompted under destructive scope: %v", c.prompts)
	}

	if _, err := g.Authorize(c, "sonarr_delete_series", AccessDestructive); err != nil {
		t.Fatalf("Authorize returned error for destructive call: %v", err)
	}
	if len(c.prompts) != 1 {
		t.Errorf("destructive call did not prompt: %v", c.prompts)
	}
}

// A client that cannot prompt must not silently upgrade confirm mode to full.
func TestFallbackDenyBlocksWhenClientCannotPrompt(t *testing.T) {
	g := gate(config.ModeConfirm, config.ScopeWrite, config.FallbackDeny)
	c := &fakeConfirmer{unsupported: true}

	_, err := g.Authorize(c, "sonarr_add_series", AccessWrite)
	if err == nil {
		t.Fatal("expected an error when the client cannot prompt, got nil")
	}
	if !errors.Is(err, ErrConfirmUnsupported) {
		t.Errorf("error = %v, want it to wrap ErrConfirmUnsupported", err)
	}
}

func TestFallbackAllowRunsWhenClientCannotPrompt(t *testing.T) {
	g := gate(config.ModeConfirm, config.ScopeWrite, config.FallbackAllow)
	c := &fakeConfirmer{unsupported: true}

	res, err := g.Authorize(c, "sonarr_add_series", AccessWrite)
	if err != nil {
		t.Fatalf("Authorize returned error under fallback=allow: %v", err)
	}
	if res != nil {
		t.Error("fallback=allow must run the call, not ask again")
	}
}

// readonly mode filters at registration, but Authorize must refuse too in case a
// tool is ever registered by another path.
func TestReadOnlyModeRefusesWritesAtCallTime(t *testing.T) {
	g := gate(config.ModeReadOnly, config.ScopeWrite, config.FallbackDeny)
	c := &fakeConfirmer{approve: true}

	if _, err := g.Authorize(c, "sonarr_add_series", AccessWrite); err == nil {
		t.Fatal("expected readonly mode to refuse a write at call time, got nil")
	}
}

func TestAnnotationsMarkReadAndDestructiveTools(t *testing.T) {
	if a := AccessRead.Annotations(); !a.ReadOnlyHint {
		t.Error("read tools must set ReadOnlyHint")
	}
	a := AccessDestructive.Annotations()
	if a.ReadOnlyHint {
		t.Error("destructive tools must not set ReadOnlyHint")
	}
	if a.DestructiveHint == nil || !*a.DestructiveHint {
		t.Error("destructive tools must set DestructiveHint")
	}
}

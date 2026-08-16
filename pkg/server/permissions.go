package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Access describes how much damage a tool can do.
type Access int

const (
	// AccessRead only reads state.
	AccessRead Access = iota
	// AccessWrite creates or modifies state.
	AccessWrite
	// AccessDestructive removes state or files.
	AccessDestructive
)

// String names the access tier for prompts and logs.
func (a Access) String() string {
	switch a {
	case AccessRead:
		return "read"
	case AccessWrite:
		return "write"
	case AccessDestructive:
		return "destructive"
	}
	return "unknown"
}

// Annotations returns the MCP hints matching this access tier so clients can
// render their own warnings independently of our gating.
func (a Access) Annotations() *mcp.ToolAnnotations {
	destructive := a == AccessDestructive
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    a == AccessRead,
		DestructiveHint: &destructive,
		IdempotentHint:  a == AccessRead,
	}
}

// Errors returned when a call is not permitted.
var (
	// ErrDeclined means the user was asked and said no.
	ErrDeclined = errors.New("the user declined this action")
	// ErrConfirmUnsupported means the client cannot prompt the user at all.
	ErrConfirmUnsupported = errors.New("client does not support elicitation")
	// ErrReadOnly means the server is configured to refuse all mutations.
	ErrReadOnly = errors.New("server is running in readonly mode")
)

// Confirmer asks the user to approve an action. Implementations return
// ErrConfirmUnsupported when the connected client cannot prompt.
type Confirmer interface {
	Confirm(ctx context.Context, prompt string) (bool, error)
}

// Gate applies the configured permission policy to tool calls.
type Gate struct {
	Perms config.Permissions
}

// Registers reports whether a tool of this access tier should be exposed at all.
// Filtering at registration keeps readonly deployments from advertising tools
// they would only refuse later.
func (g Gate) Registers(a Access) bool {
	if g.Perms.Mode == config.ModeReadOnly {
		return a == AccessRead
	}
	return true
}

// needsConfirmation reports whether this tier falls inside the confirm scope.
func (g Gate) needsConfirmation(a Access) bool {
	if g.Perms.ConfirmScope == config.ScopeDestructive {
		return a == AccessDestructive
	}
	return a == AccessWrite || a == AccessDestructive
}

// Authorize decides whether a tool call may proceed, prompting the user when
// the policy requires it. A nil return means the call is allowed.
func (g Gate) Authorize(ctx context.Context, c Confirmer, tool string, a Access) error {
	if a == AccessRead {
		return nil
	}

	switch g.Perms.Mode {
	case config.ModeReadOnly:
		return fmt.Errorf("%w: refusing %s tool %s", ErrReadOnly, a, tool)
	case config.ModeFull:
		return nil
	}

	if !g.needsConfirmation(a) {
		return nil
	}

	approved, err := c.Confirm(ctx, fmt.Sprintf(
		"Allow %s to run? This is a %s operation on your media stack.", tool, a))
	if err != nil {
		if errors.Is(err, ErrConfirmUnsupported) {
			// Failing closed matters: a client without elicitation would
			// otherwise silently turn confirm mode into full write access.
			if g.Perms.Fallback == config.FallbackAllow {
				return nil
			}
			return fmt.Errorf("%w: cannot confirm %s tool %s; set permissions.fallback=allow "+
				"or permissions.mode=full to permit it", ErrConfirmUnsupported, a, tool)
		}
		return fmt.Errorf("confirming %s: %w", tool, err)
	}
	if !approved {
		return fmt.Errorf("%w: %s", ErrDeclined, tool)
	}
	return nil
}

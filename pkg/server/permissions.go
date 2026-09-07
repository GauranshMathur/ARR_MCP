package server

import (
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

// Confirmer mediates the approval exchange with the connected client.
//
// Approval spans two calls: Ask builds the request that puts the question to
// the user, and Decision reports the answer once the client has retried the
// call carrying it. Ask returns ErrConfirmUnsupported when the client cannot
// prompt at all.
type Confirmer interface {
	Decision() (approved, answered bool, err error)
	Ask(prompt string) (*mcp.CallToolResult, error)
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

// Authorize decides whether a tool call may proceed. Three outcomes:
//
//   - (nil, nil)    the call is allowed
//   - (res, nil)    the client must approve first; res carries the input request
//   - (nil, err)    the call is refused
func (g Gate) Authorize(c Confirmer, tool string, a Access) (*mcp.CallToolResult, error) {
	if a == AccessRead {
		return nil, nil
	}

	switch g.Perms.Mode {
	case config.ModeReadOnly:
		return nil, fmt.Errorf("%w: refusing %s tool %s", ErrReadOnly, a, tool)
	case config.ModeFull:
		return nil, nil
	}

	if !g.needsConfirmation(a) {
		return nil, nil
	}

	// A retry carries the answer to the question the first pass asked.
	approved, answered, err := c.Decision()
	if err != nil {
		return nil, fmt.Errorf("confirming %s: %w", tool, err)
	}
	if answered {
		if !approved {
			return nil, fmt.Errorf("%w: %s", ErrDeclined, tool)
		}
		return nil, nil
	}

	res, err := c.Ask(fmt.Sprintf(
		"Allow %s to run? This is a %s operation on your media stack.", tool, a))
	if err != nil {
		if errors.Is(err, ErrConfirmUnsupported) {
			// Failing closed matters: a client without elicitation would
			// otherwise silently turn confirm mode into full write access.
			if g.Perms.Fallback == config.FallbackAllow {
				return nil, nil
			}
			return nil, fmt.Errorf("%w: cannot confirm %s tool %s; set permissions.fallback=allow "+
				"or permissions.mode=full to permit it", ErrConfirmUnsupported, a, tool)
		}
		return nil, fmt.Errorf("confirming %s: %w", tool, err)
	}
	return res, nil
}

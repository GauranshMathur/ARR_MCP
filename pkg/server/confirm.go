package server

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// sessionConfirmer asks the connected MCP client to prompt its user, using the
// protocol's elicitation capability.
type sessionConfirmer struct {
	session *mcp.ServerSession
}

// Confirm implements Confirmer against a live MCP session.
func (c sessionConfirmer) Confirm(ctx context.Context, prompt string) (bool, error) {
	if c.session == nil {
		return false, ErrConfirmUnsupported
	}
	init := c.session.InitializeParams()
	if init == nil || init.Capabilities == nil || init.Capabilities.Elicitation == nil {
		return false, ErrConfirmUnsupported
	}

	res, err := c.session.Elicit(ctx, &mcp.ElicitParams{
		Mode:    "confirm",
		Message: prompt,
	})
	if err != nil {
		return false, fmt.Errorf("eliciting confirmation: %w", err)
	}
	return interpretElicit(res.Action)
}

// interpretElicit maps an elicitation action onto an approval decision.
// Anything that is not an explicit acceptance is treated as a refusal.
func interpretElicit(action string) (bool, error) {
	switch action {
	case "accept":
		return true, nil
	case "decline", "cancel":
		return false, nil
	default:
		return false, fmt.Errorf("unrecognised elicitation action %q", action)
	}
}

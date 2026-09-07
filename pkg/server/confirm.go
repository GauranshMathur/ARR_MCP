package server

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// confirmationID keys the approval exchange in the input request and response
// maps. The client echoes it back on the retry, which is how the second pass
// finds the answer to the question the first pass asked.
const confirmationID = "arr-mcp-confirm"

// sessionConfirmer asks the connected MCP client to prompt its user.
//
// Approval is a two-step exchange (SEP-2322): a server may not send
// elicitation/create while it is serving a tool call, so the first pass returns
// an input request and ends, the client prompts, and the retried call carries
// the answer. Clients on protocol versions before 2026-07-28 never see the
// difference — the SDK's multi round-trip middleware fulfils the request by
// eliciting on our behalf and reinvokes the handler.
type sessionConfirmer struct {
	session *mcp.ServerSession
	params  *mcp.CallToolParamsRaw
}

// Decision reports the user's answer when this call is a retry carrying one.
// A false answered means nobody has been asked yet.
func (c sessionConfirmer) Decision() (approved, answered bool, err error) {
	if c.params == nil {
		return false, false, nil
	}
	resp, ok := c.params.InputResponses[confirmationID]
	if !ok {
		return false, false, nil
	}
	res, ok := resp.(*mcp.ElicitResult)
	if !ok {
		return false, true, fmt.Errorf("confirmation answered with unexpected type %T", resp)
	}
	approved, err = interpretElicit(res.Action)
	return approved, true, err
}

// Ask builds the result that asks the client to confirm prompt. It reports
// ErrConfirmUnsupported when the client cannot prompt its user at all.
func (c sessionConfirmer) Ask(prompt string) (*mcp.CallToolResult, error) {
	if c.session == nil {
		return nil, ErrConfirmUnsupported
	}
	init := c.session.InitializeParams()
	if init == nil || init.Capabilities == nil || init.Capabilities.Elicitation == nil {
		return nil, ErrConfirmUnsupported
	}

	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{
			confirmationID: confirmElicitParams(prompt),
		},
	}, nil
}

// confirmElicitParams builds the elicitation asking a yes/no question.
//
// Mode is left unset so the SDK infers "form", the only mode besides "url" the
// protocol defines. The schema is deliberately empty: the decision travels in
// the result's action, so accept/decline stays the single source of truth
// rather than being split across the action and a form field that could
// disagree with it.
func confirmElicitParams(prompt string) *mcp.ElicitParams {
	return &mcp.ElicitParams{
		Message: prompt,
		RequestedSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
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

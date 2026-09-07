package server

import (
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestInterpretElicitAcceptApproves(t *testing.T) {
	ok, err := interpretElicit("accept")
	if err != nil {
		t.Fatalf("interpretElicit returned error: %v", err)
	}
	if !ok {
		t.Error("accept must approve the action")
	}
}

func TestInterpretElicitDeclineRejects(t *testing.T) {
	ok, err := interpretElicit("decline")
	if err != nil {
		t.Fatalf("interpretElicit returned error: %v", err)
	}
	if ok {
		t.Error("decline must reject the action")
	}
}

// "cancel" means dismissed without an explicit choice. Treating that as
// approval would run a destructive call the user never agreed to.
func TestInterpretElicitCancelRejects(t *testing.T) {
	ok, err := interpretElicit("cancel")
	if err != nil {
		t.Fatalf("interpretElicit returned error: %v", err)
	}
	if ok {
		t.Error("cancel must reject the action")
	}
}

func TestInterpretElicitUnknownActionRejects(t *testing.T) {
	ok, err := interpretElicit("something-new")
	if ok {
		t.Error("an unrecognised action must not approve the action")
	}
	if err == nil {
		t.Fatal("expected an error for an unrecognised action, got nil")
	}
}

func TestSessionConfirmerWithoutSessionIsUnsupported(t *testing.T) {
	c := sessionConfirmer{}

	_, err := c.Ask("do the thing")
	if !errors.Is(err, ErrConfirmUnsupported) {
		t.Errorf("error = %v, want ErrConfirmUnsupported", err)
	}
}

// Nothing has been asked yet on the first pass, so there is no answer to read.
func TestSessionConfirmerHasNoDecisionOnFirstPass(t *testing.T) {
	c := sessionConfirmer{params: &mcp.CallToolParamsRaw{}}

	_, answered, err := c.Decision()
	if err != nil {
		t.Fatalf("Decision returned error: %v", err)
	}
	if answered {
		t.Error("Decision reported an answer before the user was asked")
	}
}

func TestSessionConfirmerReadsTheRetriedAnswer(t *testing.T) {
	for _, tc := range []struct {
		action string
		want   bool
	}{{"accept", true}, {"decline", false}, {"cancel", false}} {
		c := sessionConfirmer{params: &mcp.CallToolParamsRaw{
			InputResponses: mcp.InputResponseMap{
				confirmationID: &mcp.ElicitResult{Action: tc.action},
			},
		}}

		approved, answered, err := c.Decision()
		if err != nil {
			t.Fatalf("Decision(%q) returned error: %v", tc.action, err)
		}
		if !answered {
			t.Errorf("Decision(%q) did not report an answer", tc.action)
		}
		if approved != tc.want {
			t.Errorf("Decision(%q) approved = %v, want %v", tc.action, approved, tc.want)
		}
	}
}

// The elicitation must be one the protocol actually defines. "confirm" is not a
// mode — only "form" (the default, inferred from an unset Mode) and "url" are —
// and form elicitation requires a flat object schema.
func TestConfirmElicitParamsAreSpecValid(t *testing.T) {
	p := confirmElicitParams("Allow sonarr_grab_release to run?")

	if p.Mode != "" && p.Mode != "form" {
		t.Errorf("Mode = %q, want it unset or %q", p.Mode, "form")
	}
	if p.URL != "" {
		t.Errorf("URL = %q, want it unset for form elicitation", p.URL)
	}
	if p.Message == "" {
		t.Error("Message is empty; the user would be asked a blank question")
	}

	schema, ok := p.RequestedSchema.(map[string]any)
	if !ok {
		t.Fatalf("RequestedSchema is %T, want a JSON schema object", p.RequestedSchema)
	}
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want \"object\"", schema["type"])
	}
	if _, ok := schema["properties"].(map[string]any); !ok {
		t.Errorf("schema properties = %v, want an object", schema["properties"])
	}
}

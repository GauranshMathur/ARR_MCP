package server

import (
	"context"
	"errors"
	"testing"
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

	_, err := c.Confirm(context.Background(), "do the thing")
	if !errors.Is(err, ErrConfirmUnsupported) {
		t.Errorf("error = %v, want ErrConfirmUnsupported", err)
	}
}

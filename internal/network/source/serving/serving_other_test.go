//go:build !linux && !darwin

package serving

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
)

// The selected Unix-family input-custody and process-guard mechanism is absent
// on this platform, so the Direct-Origin Source operation must return the same
// explicit typed unavailable outcome (ADR-0129): no input, process guard or
// State is acquired and nothing is created. This is the refusal boundary, never
// a weaker accepting path.
func TestUnsupportedServingPlatformRefusesBeforeAnyState(t *testing.T) {
	reported := false
	outcome := Run(context.Background(), "plan", false, func(context.Context, Event) error {
		reported = true
		return nil
	})
	if outcome != Refused || reported {
		t.Fatal("unsupported Run accepted or reported an event", outcome, reported)
	}
	if ValidDeclaration(servingdeclaration.ServingDeclaration{}) {
		t.Fatal("unsupported platform admitted a declaration")
	}
	var out bytes.Buffer
	if err := WriteFailureReport(&out, "serve", errors.New("operation"), nil); err != errServingPlatformUnavailable || out.Len() != 0 {
		t.Fatal("unsupported report opened output or lost the typed refusal", err, out.Len())
	}
	if facts := DescribeFailure(errors.New("operation"), nil); facts.FirstOwner != "" || facts.CleanupOwners != "" || facts.Resource != nil {
		t.Fatal("unsupported platform classified an original failure order", facts)
	}
	cause := errors.New("original")
	terminal := &Terminal{FirstOwner: "process", First: cause}
	if terminal.Error() != "original Source failed" {
		t.Fatal("terminal failure lost its fixed message", terminal.Error())
	}
	if causes := terminal.Unwrap(); len(causes) != 1 || causes[0] != cause {
		t.Fatal("terminal failure lost its original cause order", causes)
	}
}

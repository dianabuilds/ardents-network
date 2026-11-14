package descriptor

import (
	"context"
	"testing"
)

func TestOriginalInputsMissingCallerAndZeroCustodyRefuse(t *testing.T) {
	var empty Inputs
	if empty.Check() == nil || InspectInitialInputs(empty) == nil {
		t.Fatal("zero inputs granted original Descriptor observation")
	}
	if empty.Close() != nil {
		t.Fatal("empty input Close failed")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, caller := range []context.Context{nil, ctx} {
		input, err := ReadInputs(caller, "/var/lib/descriptor-request.json")
		if err == nil || input.original != nil || input.Close() != nil {
			t.Fatal("unavailable original caller acquired Descriptor input custody", err)
		}
	}
}

package daemon

import (
	"testing"
)

func TestBootResultFromTransportNormalizesState(t *testing.T) {
	result := BootResultFromTransport(true, "unknown", "detail")
	if result.State != BootIdle {
		t.Fatalf("state = %q, want %q", result.State, BootIdle)
	}
	if !result.Joined || result.Reason != "detail" {
		t.Fatalf("result = %#v, want joined/detail preserved", result)
	}
}

func TestStoppedBootResult(t *testing.T) {
	result := StoppedBootResult()
	if result.State != BootIdle || result.Reason != "node stopped" {
		t.Fatalf("result = %#v, want stopped idle result", result)
	}
}

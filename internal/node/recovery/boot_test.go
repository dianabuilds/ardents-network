package recovery_test

import (
	"testing"

	noderecovery "ardents/internal/node/recovery"
)

func TestBootResultFromTransportNormalizesState(t *testing.T) {
	result := noderecovery.BootResultFromTransport(true, "unknown", "detail")
	if result.State != noderecovery.BootIdle {
		t.Fatalf("state = %q, want %q", result.State, noderecovery.BootIdle)
	}
	if !result.Joined || result.Reason != "detail" {
		t.Fatalf("result = %#v, want joined/detail preserved", result)
	}
}

func TestStoppedBootResult(t *testing.T) {
	result := noderecovery.StoppedBootResult()
	if result.State != noderecovery.BootIdle || result.Reason != "node stopped" {
		t.Fatalf("result = %#v, want stopped idle result", result)
	}
}

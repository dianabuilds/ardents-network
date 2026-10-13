//go:build !linux

package runtime

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

// Portable refusal is not installed qualification. Even a valid local Grant
// cannot produce a native Reader invocation on an unsupported platform.
func TestUnsupportedReaderLaunchCannotProduceInvocation(t *testing.T) {
	principal := [32]byte{2}
	owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: principal, Surface: execution.Connection}}})
	if err != nil {
		t.Fatal(err)
	}
	invocation, err := owner.LaunchReader(t.Context(), principal)
	if err == nil || invocation != nil {
		t.Fatal("unsupported native launch supplied an invocation", invocation, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal("refused launch retained unjoined cleanup", err)
	}
}

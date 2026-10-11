//go:build linux

package runtime

import (
	"bytes"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

func TestPublisherSnapshotRefusesBeforeNativeActivation(t *testing.T) {
	for _, body := range [][]byte{{0xff}, bytes.Repeat([]byte{'a'}, textdocument.MaximumBytes+1)} {
		var owner *Owner
		if invocation, err := owner.LaunchPublisher(t.Context(), [32]byte{1}, body); invocation != nil || err == nil {
			t.Fatal("invalid snapshot reached a qualified launch")
		}
		if len(launchGate) != 0 {
			t.Fatal("invalid snapshot retained native activation")
		}
	}
}

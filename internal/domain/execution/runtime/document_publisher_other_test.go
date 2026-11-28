//go:build !linux

package runtime

import (
	"bytes"
	"testing"
)

func TestUnsupportedSnapshotPublisherCannotProduceInvocation(t *testing.T) {
	var owner *Owner
	invocation, err := owner.LaunchSnapshotPublisher(t.Context(), [32]byte{1}, bytes.NewReader([]byte{0xff}), 1)
	if invocation != nil || err == nil {
		t.Fatalf("unsupported snapshot launch supplied an invocation: %v", err)
	}
}

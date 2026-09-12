//go:build linux

package route

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestClosedSourcePrefixOpenEmissionUsesPendingDeadline(t *testing.T) {
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()
	end := time.Now().UTC().Add(30 * time.Minute).Truncate(time.Second)
	if err := local.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	prefix := &ClosedSourcePrefix{connection: local}
	done := make(chan error, 1)
	go func() {
		done <- prefix.openChild(t.Context(), closedBootstrapPeer{node: [32]byte{1}, generation: 1}, end, time.Now().Add(50*time.Millisecond))
	}()
	// The parent accepted its own role but stops reading before child OPEN.
	select {
	case err := <-done:
		var timeout net.Error
		if !errors.As(err, &timeout) || !timeout.Timeout() {
			t.Fatalf("blocked OPEN outcome: %v", err)
		}
		if prefix.child != nil {
			t.Fatal("child started before OPEN completed")
		}
	case <-time.After(2 * time.Second):
		_ = local.Close()
		<-done
		t.Fatal("OPEN waited beyond pending interval")
	}
}

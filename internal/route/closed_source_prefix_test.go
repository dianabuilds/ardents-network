//go:build linux

package route

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestClosedSourceOpenFailureRetainsStageAndCause(t *testing.T) {
	cause := errors.New("interior admission refused")
	failure := closedSourceOpenFailureAt("interior-admission", cause)
	if got := ClosedSourceOpenFailureStage(failure); got != "interior-admission" {
		t.Fatalf("closed Source open stage = %q", got)
	}
	if !errors.Is(failure, cause) {
		t.Fatal("closed Source open failure lost its cause")
	}
}

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

func TestClosedSourcePrefixLifetimeExtendsBeyondBootstrapWindow(t *testing.T) {
	now := time.Now().UTC()
	plan := closedBootstrapPlan{deadline: now.Add(10 * time.Second), profile: state.ClosedProfileView{NotAfter: now.Add(time.Hour)}}
	plan.peers[0].notAfter = now.Add(time.Hour)
	plan.peers[1].notAfter = now.Add(time.Hour)
	snapshot := state.Snapshot{ValidUntil: now.Add(time.Hour)}

	end := closedSourcePrefixEnd(plan, snapshot, now)
	remaining := end.Sub(now)
	if remaining < 29*time.Minute || remaining > 30*time.Minute {
		t.Fatalf("retained source prefix lifetime = %s, want the bounded 1,800-second class-2 lease", remaining)
	}
}

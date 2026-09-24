//go:build linux

package route

import (
	"context"
	"errors"
	"net"
	"syscall"
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

func TestClosedSourceOpenFailureDetailRetainsCarrierBoundary(t *testing.T) {
	cause := errors.New("transport unavailable")
	failure := closedSourceOpenFailureAt("entry-carrier", closedRoleOpenFailureAt("quic-dial", cause))
	if got := ClosedSourceOpenFailureDetail(failure); got != "entry-carrier-quic-dial-other" {
		t.Fatalf("closed Source open detail = %q", got)
	}
	if !errors.Is(failure, cause) {
		t.Fatal("closed Source open detail lost its cause")
	}
}

func TestClosedSourceOpenFailureDetailRetainsNestedTCPBoundary(t *testing.T) {
	cause := errors.New("handshake refused")
	failure := closedSourceOpenFailureAt("entry-carrier", closedRoleOpenFailureAt("tcp-tls", closedRoleOpenFailureAt("tls-handshake", cause)))
	if got := ClosedSourceOpenFailureDetail(failure); got != "entry-carrier-tcp-tls-tls-handshake-other" {
		t.Fatalf("closed Source nested open detail = %q", got)
	}
	if !errors.Is(failure, cause) {
		t.Fatal("closed Source nested open detail lost its cause")
	}
}

func TestClosedSourceOpenFailureDetailRetainsDeadlineCategory(t *testing.T) {
	failure := closedSourceOpenFailureAt("interior-tls", closedRoleOpenFailureAt("tls-handshake", context.DeadlineExceeded))
	if got := ClosedSourceOpenFailureDetail(failure); got != "interior-tls-tls-handshake-deadline" {
		t.Fatalf("closed Source deadline detail = %q", got)
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

func TestClosedSourceOpenFailureDetailClassifiesWrappedTCPDialSyscalls(t *testing.T) {
	for _, test := range []struct {
		name  string
		cause error
		want  string
	}{
		{name: "refused", cause: syscall.ECONNREFUSED, want: "refused"},
		{name: "network unreachable", cause: syscall.ENETUNREACH, want: "network-unreachable"},
		{name: "host unreachable", cause: syscall.EHOSTUNREACH, want: "host-unreachable"},
		{name: "address unavailable", cause: syscall.EADDRNOTAVAIL, want: "address-unavailable"},
		{name: "reset", cause: syscall.ECONNRESET, want: "reset"},
		{name: "permission", cause: syscall.EACCES, want: "permission"},
		{name: "resource", cause: syscall.EMFILE, want: "resource"},
		{name: "invalid", cause: syscall.EINVAL, want: "invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cause := &net.OpError{Op: "dial", Net: "tcp", Err: test.cause}
			failure := closedSourceOpenFailureAt("entry-carrier", closedRoleOpenFailureAt("tcp-dial", cause))
			if got := ClosedSourceOpenFailureDetail(failure); got != "entry-carrier-tcp-dial-"+test.want {
				t.Fatalf("closed Source TCP dial detail = %q", got)
			}
			if !errors.Is(failure, test.cause) {
				t.Fatal("closed Source TCP dial failure lost its syscall cause")
			}
		})
	}
}

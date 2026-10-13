package join

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
)

// This local identity fixture supplies no accepting acquisition or transport.
// Failed retirement may affect only the exact originally published child.
func TestCopiedJoinedCannotChangeOriginalRetirement(t *testing.T) {
	a := &JoinAcquisition{}
	a.self = a
	original := &Joined{acquisition: a}
	a.stream = original
	copied := &Joined{acquisition: a}
	if err := copied.MarkFailedRetirement(); err == nil || original.retirementDisposition.Load() != retirementUnselected {
		t.Fatal("copied child changed original terminal disposition", err)
	}
	if err := original.MarkFailedRetirement(); err != nil || original.retirementDisposition.Load() != retirementFailed {
		t.Fatal("original child failed to retain its own disposition", err)
	}
	a.closed = true
	if err := original.MarkFailedRetirement(); err == nil {
		t.Fatal("late marker claimed to replace an already committed retirement")
	}
}

// Actual framing exposes the selected terminal while the acquisition is still
// locally open. This supplies no accepting JOIN or Service authority.
func TestFailedMarkerRefusesAlreadySelectedTerminal(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "clean", true: "refused"}[failed], func(t *testing.T) {
			local, peer := net.Pipe()
			defer peer.Close()
			end := time.Now().Add(time.Second)
			if err := peer.SetDeadline(end); err != nil {
				t.Fatal(err)
			}
			session, lane, err := newJoinedSession(t.Context(), local, end, 1<<20, nil, framing.NewBudget(4<<20))
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			caller, cancel := context.WithCancel(t.Context())
			defer cancel()
			a := &JoinAcquisition{ctx: t.Context()}
			a.self = a
			j := &Joined{acquisition: a, session: session, lane: lane, retiring: make(chan struct{}), watcherDone: make(chan struct{})}
			a.stream = j
			if failed {
				if err := j.MarkFailedRetirement(); err != nil {
					t.Fatal(err)
				}
			}
			go j.watch(caller)
			cancel()
			frame, err := ardp.ReadFrame(peer)
			want := byte(0)
			if failed {
				want = 1
			}
			if err != nil || frame.Kind != ardp.KindClose || len(frame.Body) != 1 || frame.Body[0] != want {
				t.Fatal("original selected terminal differs", frame, err)
			}
			if err := j.MarkFailedRetirement(); err == nil {
				t.Fatal("late marker claimed to change a terminal already emitted")
			}
			<-j.watcherDone
		})
	}
}

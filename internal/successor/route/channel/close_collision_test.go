package channel

import (
	"net"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// A canonical peer CLOSE arrives after local retirement, while the original
// terminal is queued or observing authority. It must prevent a new physical
// write, without turning local retirement or peer refusal into clean EOF.
func TestPeerCLOSEDischargesUnemittedLocalCLOSE(t *testing.T) {
	for _, phase := range []string{"queued", "selected"} {
		for _, status := range []byte{0, 1} {
			t.Run(phase+"/"+string(rune('0'+status)), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					local, peer := net.Pipe()
					resume := make(chan struct{})
					var observed atomic.Uint32
					check := func() error {
						if phase == "selected" && observed.Add(1) == 1 {
							<-resume
						}
						return nil
					}
					s := Prepare(t.Context(), local, time.Now().Add(10*time.Second), 1<<20, check, false, NewBudget(4<<20), Handlers{})
					lane := lifecycleLane(t, s, 1)
					lane.openEmitted = true
					release := func() {}
					if phase == "queued" {
						sibling := lifecycleLane(t, s, 3)
						var err error
						release, err = s.turn(sibling, ardp.Frame{Kind: ardp.KindCredit, Lane: 3, Body: []byte{0, 0, 0, 1}}, false, ardp.HeaderSize+4)
						if err != nil {
							t.Fatal(err)
						}
					}
					s.Start()
					joined := make(chan error, 1)
					go func() { joined <- lane.Close() }()
					synctest.Wait()
					if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{status}}); err != nil {
						t.Fatal(err)
					}
					synctest.Wait()
					s.mu.Lock()
					received := lane.localClosed && lane.peerClosed && lane.peerRefused == (status != 0)
					s.mu.Unlock()
					close(resume)
					release()
					err := <-joined
					// A later conflicting terminal cannot replace the original
					// peer outcome, including converting refusal into success.
					if writeErr := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{1 - status}}); writeErr != nil {
						t.Fatal(writeErr)
					}
					synctest.Wait()
					s.mu.Lock()
					retained := lane.peerRefused == (status != 0)
					s.mu.Unlock()
					witness := lane.retirementWitness()
					if !received || !retained || err != nil || witness.attempts != 0 || !witness.local || s.PhysicalFailure() != nil {
						t.Errorf("peer terminal lost or unnecessary physical CLOSE started: received=%v err=%v witness=%+v physical=%v", received, err, witness, s.PhysicalFailure())
					}
					if lane.Close() != err {
						t.Error("original close result changed")
					}
					_ = peer.Close()
					_ = s.Close()
				})
			})
		}
	}
}

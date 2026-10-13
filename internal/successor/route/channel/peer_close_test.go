package channel

import (
	"context"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// Real dedicated framing proves a decoded transport fact only, not a Service
// authentication, live permission or successful Application exchange.
func TestPeerCloseEvidenceRequiresDecodedExactChild(t *testing.T) {
	for _, terminal := range []string{"clean", "refused", "raw-EOF", "directional-EOF", "local-close", "cancelled"} {
		t.Run(terminal, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				local, remote := net.Pipe()
				defer remote.Close()
				session, lane, err := newJoinedSession(ctx, local, time.Now().Add(time.Minute), 1<<20, nil, NewBudget(4<<20))
				if err != nil {
					t.Fatal(err)
				}
				defer session.Close()
				if _, err := lane.DecodedPeerClose(); err == nil {
					t.Fatal("undecoded child supplied evidence")
				}
				switch terminal {
				case "clean", "refused":
					status := byte(0)
					if terminal == "refused" {
						status = 1
					}
					if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{status}}); err != nil {
						t.Fatal(err)
					}
					<-session.readerDone
				case "raw-EOF":
					_ = remote.Close()
					<-session.readerDone
				case "directional-EOF":
					if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindEOF, Lane: 1}); err != nil {
						t.Fatal(err)
					}
					synctest.Wait()
				case "local-close":
					closed := make(chan error, 1)
					go func() { closed <- lane.Close() }()
					frame, err := ardp.ReadFrame(remote)
					if err != nil || frame.Kind != ardp.KindClose {
						t.Fatal("original local CLOSE absent", err)
					}
					if err := <-closed; err != nil {
						t.Fatal(err)
					}
				case "cancelled":
					cancel()
					<-session.readerDone
				}
				proof, err := lane.DecodedPeerClose()
				if terminal != "clean" {
					if err == nil {
						t.Fatal("non-clean terminal supplied decoded evidence")
					}
					return
				}
				if err != nil || proof.CheckChild(lane) != nil {
					t.Fatal("actual decoded CLOSE lost original child", err)
				}
				copied := Lane{s: session, id: lane.id, peerClosed: true}
				if proof.CheckChild(&copied) == nil {
					t.Fatal("copy acquired original evidence")
				}
				if _, err := copied.DecodedPeerClose(); err == nil {
					t.Fatal("copied lane minted evidence")
				}
				if (PeerClose{}).CheckChild(lane) == nil {
					t.Fatal("zero evidence accepted")
				}
				lane.Finish()
				if err := proof.CheckChild(lane); err != nil {
					t.Fatal("joined retirement erased decoded fact", err)
				}
			})
		})
	}
}

func TestMultiplexedChildCannotMintDedicatedPeerCloseEvidence(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	session := New(t.Context(), local, time.Now().Add(time.Minute), 1<<20, nil, false, NewBudget(4<<20), nil)
	defer session.Close()
	lane := lifecycleLane(t, session, 1)
	if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
		t.Fatal(err)
	}
	waitFramingState(t, lane, func() bool { return lane.peerClosed })
	if _, err := lane.DecodedPeerClose(); err == nil {
		t.Fatal("multiplexed sibling supplied dedicated JOIN terminal evidence")
	}
}

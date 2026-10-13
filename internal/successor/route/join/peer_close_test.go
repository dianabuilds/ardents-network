package join

import (
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
)

// This tests projection of a real decoded frame through exact local handles.
// The incomplete acquisition supplies no admitted roles, JOIN ACK or Service
// authority; genuine participant profiles own those separate acceptance facts.
func TestJoinedPeerCloseCannotMoveToAnotherStream(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	session, lane, err := newJoinedSession(t.Context(), local, time.Now().Add(time.Minute), 1<<20, nil, framing.NewBudget(4<<20))
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	acquisition := &JoinAcquisition{}
	joined := &Joined{acquisition: acquisition, session: session, lane: lane}
	if _, err := joined.DecodedPeerClose(); err == nil {
		t.Fatal("unpublished JOIN supplied evidence")
	}
	acquisition.stream = joined
	if _, err := joined.DecodedPeerClose(); err == nil {
		t.Fatal("JOIN supplied undecoded evidence")
	}
	if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
		t.Fatal(err)
	}
	if err := session.WaitTerminal(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	proof, err := joined.DecodedPeerClose()
	if err != nil || proof.CheckOriginal(joined) != nil {
		t.Fatal("original decoded child lost exact projection", err)
	}
	copied := Joined{acquisition: acquisition, session: session, lane: lane}
	if proof.CheckOriginal(&copied) == nil {
		t.Fatal("copied JOIN acquired original evidence")
	}
	if _, err := copied.DecodedPeerClose(); err == nil {
		t.Fatal("copied JOIN minted evidence")
	}
	if (PeerClose{}).CheckOriginal(joined) == nil || proof.CheckOriginal(nil) == nil {
		t.Fatal("zero/foreign JOIN acquired evidence")
	}
}

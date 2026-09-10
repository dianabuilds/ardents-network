//go:build linux

package route

import (
	"crypto/ed25519"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// The peer has the exact State-pinned role TLS key but deliberately delays its
// RESULT. Admission bytes are a fixture: this tests the client's trust boundary,
// not the issuer or Rendezvous server (covered by the real Node network tests).
func TestClosedJoinClientRejectsResultAfterSetupDeadline(t *testing.T) {
	prefix, source := sourceResolutionSelectionFixture(t)
	certificate := entryBindingCertificate(t, 191)
	source.view.NodeCount, source.snapshot.CandidateCount = 6, 6
	for index, subrole := range []uint8{3, 4} {
		at := index + 4
		source.view.Nodes[at] = state.ClosedRouteNodeView{NodeID: [32]byte{byte(50 + at)}, RecordDigest: [32]byte{byte(60 + at)}, RoleDomain: 2, Subrole: subrole, DutyGeneration: uint64(at + 1)}
		peer := source.snapshot.Candidates[3]
		peer.NodeID, peer.RecordDigest = source.view.Nodes[at].NodeID, source.view.Nodes[at].RecordDigest
		peer.PublicKey, peer.FamilyID = [32]byte{byte(70 + at)}, [32]byte{byte(80 + at)}
		source.snapshot.Candidates[at] = peer
	}
	source.view.Nodes[4].RoleDomain = 4
	source.snapshot.Candidates[5].PublicKey = identifierFromKey(certificate.Leaf.PublicKey.(ed25519.PublicKey))
	client, server := net.Pipe()
	end := time.Now().UTC().Add(20 * time.Second).Truncate(time.Second)
	prefix.channels = newClosedSourceChannelOwner(client, end, client.Close)
	prefix.channels.start()
	defer prefix.channels.Close()
	defer server.Close()
	setup := time.Now().UTC().Add(2 * time.Second).Truncate(time.Second)
	resultAttempted := make(chan struct{})
	serverDone := make(chan error, 1)
	clientReturned := make(chan struct{})
	go func() {
		serverDone <- func() error {
			opening, err := ReadClosedLaneFrame(server)
			if err != nil {
				return err
			}
			if opening.Kind != closedFrameOpen || opening.Lane != 1 {
				return errors.New("unexpected outer OPEN")
			}
			channels := newClosedSourceChannelOwner(server, end, server.Close)
			lane := &closedSourceLane{owner: channels, id: 1, end: end, readEnd: end, writeEnd: end, opened: true, active: true, credit: 64 << 10, receiveCredit: 64 << 10}
			channels.last, channels.lanes[1] = 1, lane
			channels.start()
			defer channels.Close()
			secured, err := AcceptClosedRoleTLS(t.Context(), lane, certificate, setup)
			if err != nil {
				return err
			}
			if _, err := ReadClosedLaneFrame(secured); err != nil {
				return err
			}
			if _, err := ReadClosedLaneFrame(secured); err != nil {
				return err
			}
			if err := WriteClosedLaneFrame(secured, ClosedLaneFrame{Kind: closedFrameAccept, Body: []byte{0, 0, 1, 0, 0}}); err != nil {
				return err
			}
			operation, err := ReadClosedLaneFrame(secured)
			if err != nil {
				return err
			}
			request, err := DecodeClosedJoinRequest(operation.Body)
			if err != nil {
				return err
			}
			<-time.After(time.Until(setup.Add(100 * time.Millisecond)))
			close(resultAttempted)
			body, err := EncodeClosedJoinResult(request.Nonce, 0)
			if err != nil {
				return err
			}
			err = WriteClosedLaneFrame(secured, ClosedLaneFrame{Kind: closedFrameResult, Lane: 1, Body: body})
			<-clientReturned
			return err
		}()
	}()
	stream, err := prefix.Join(t.Context(), func(ClosedHello, uint8) ([]byte, error) { return make([]byte, 354), nil }, ClosedJoinIntent{Secret: [32]byte{1}, Context: [32]byte{2}, SetupDeadline: setup, WorkDeadline: end})
	close(clientReturned)
	if stream != nil {
		_ = stream.Close()
	}
	if err == nil {
		t.Error("late authenticated RESULT activated data after setup expired")
	}
	_ = prefix.channels.Close()
	serverErr := <-serverDone
	select {
	case <-resultAttempted:
	default:
		t.Fatalf("test peer failed before delayed RESULT: client %v; server %v", err, serverErr)
	}
}

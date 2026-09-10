//go:build linux

package node

import (
	"crypto/ecdh"
	"crypto/rand"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// The parent alone owns recipient key and plaintext canaries. Real HPKE seals
// them before the actual admitted Submission enters the receiving process.
// Publication/Endpoint job ownership is a fixture, not a full Service journey.
func startHeapSubmission(t *testing.T, fixture *resolutionNetworkFixture, request route.ClosedRegistrationRequest) (net.Conn, func(), [32]byte, [2][32]byte, []byte) {
	t.Helper()
	submitter := *fixture
	submitter.receiver.ExpectedPurpose = route.ClosedPurposeSubmission
	connection, closeCarrier, err := submitter.openTerminal(t.Context(), fixture.supplementary[1][0], 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeCarrier)
	end := time.Now().UTC().Add(9 * time.Second).Truncate(time.Second)
	if err := connection.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	var private [2][32]byte
	var nonce, delivery [32]byte
	for _, value := range [][]byte{private[0][:], private[1][:], nonce[:], delivery[:]} {
		if _, err := rand.Read(value); err != nil {
			t.Fatal(err)
		}
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := route.ClosedIntroductionPlaintext{Network: fixture.profile.NetworkID, ProfileDigest: fixture.profile.Digest,
		Target: private[0], JoinSecret: private[1], PublicationDigest: [32]byte{201}, Revision: request.Revision,
		RendezvousNode: [32]byte{202}, RendezvousDutyGeneration: 1, HandshakeContext: [32]byte{203}, ConnectionNonce: [32]byte{204},
		AttachmentGeneration: 1, Deadline: end, InitiatorBinding: [32]byte{205}, WorkSafetyNotAfter: end.Unix(), WorkSafetyMaximum: end.Unix(), NoNewRecoveryAfter: end.Unix()}
	envelope := route.ClosedIntroductionCapsule{Slot: request.Slot, Revision: request.Revision, Expiry: end, DeliveryNonce: delivery}
	sealed, _, err := route.SealClosedIntroduction(envelope, [32]byte(key.PublicKey().Bytes()), plaintext)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := route.EncodeClosedIntroductionSubmission(nonce, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 10, Body: operation}); err != nil {
		t.Fatal(err)
	}
	return connection, closeCarrier, nonce, private, sealed.Ciphertext
}

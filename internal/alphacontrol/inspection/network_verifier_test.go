package inspection

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestVerifyNetworkAcceptsInitialClosedEpochWithOnePinnedAuthority(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	nodePublic, nodePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	network := [32]byte{7}
	message, err := epoch.PrepareInitialClosedRecord(epoch.InitialClosedRecord{
		NetworkID: network, NodeID: [32]byte{8}, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour),
		Family: "one-honest-family", Endpoint: "127.0.0.1:18491", Carrier: "ardents-carrier-tcp-tls-v2", Capacity: 1, PublicKey: nodePublic,
	})
	if err != nil {
		t.Fatal(err)
	}
	record := append(message, ed25519.Sign(nodePrivate, message)...)
	unsigned, err := epoch.PrepareInitialClosedEpoch(epoch.InitialClosedEpoch{
		NetworkID: network, AssignmentSeed: [32]byte{9}, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour),
		Domains: []string{"initiator", "introduction", "rendezvous", "responder"}, Records: [][]byte{record}, AuthorityKeys: []ed25519.PublicKey{public},
	})
	if err != nil {
		t.Fatal(err)
	}
	digest, id := sha256.Sum256(unsigned), sha256.Sum256(public)
	raw := append(unsigned, 1)
	raw = append(raw, id[:]...)
	raw = append(raw, ed25519.Sign(private, digest[:])...)
	verified, err := epoch.Verify(epoch.Policy{NetworkID: network, Authorities: map[[32]byte]ed25519.PublicKey{id: public}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: now}, raw, [][]byte{record}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	material, err := verified.Materialization(0)
	if err != nil {
		t.Fatal(err)
	}
	body, err := encodeNetworkEvidence(NetworkEvidence{NetworkID: network, EpochDigest: digest, Profile: epoch.ProfileClosedRoute, Threshold: 1,
		Authorities: []ed25519.PublicKey{public}, Epoch: raw, Inputs: [][]byte{record}, Materials: [][]byte{material}})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot state.Snapshot
	accepted := false
	outcome, closeErr := verifyNetwork(context.Background(), t.TempDir(), body, now, &snapshot, &accepted)
	if closeErr != nil || outcome != alphacontrol.OutcomeAccepted || !accepted || snapshot.Digest != digest || snapshot.Profile != epoch.ProfileClosedRoute {
		t.Fatalf("closed Network inspection = %q, accepted=%v, epoch=%d, profile=%q, close=%v", outcome, accepted, snapshot.Epoch, snapshot.Profile, closeErr)
	}
	other, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, threshold := range []uint8{1, 2} {
		ambiguous, err := encodeNetworkEvidence(NetworkEvidence{NetworkID: network, EpochDigest: digest, Profile: epoch.ProfileClosedRoute, Threshold: threshold,
			Authorities: []ed25519.PublicKey{public, other}, Epoch: raw, Inputs: [][]byte{record}, Materials: [][]byte{material}})
		if err != nil {
			t.Fatal(err)
		}
		accepted = false
		outcome, closeErr := verifyNetwork(context.Background(), t.TempDir(), ambiguous, now, &snapshot, &accepted)
		if closeErr != nil || outcome != alphacontrol.OutcomeInvalid || accepted {
			t.Fatalf("ambiguous closed authority threshold=%d: outcome=%q, accepted=%v, close=%v", threshold, outcome, accepted, closeErr)
		}
	}
	raw[len(raw)-1] ^= 1
	forged, err := encodeNetworkEvidence(NetworkEvidence{NetworkID: network, EpochDigest: digest, Profile: epoch.ProfileClosedRoute, Threshold: 1,
		Authorities: []ed25519.PublicKey{public}, Epoch: raw, Inputs: [][]byte{record}, Materials: [][]byte{material}})
	if err != nil {
		t.Fatal(err)
	}
	accepted = false
	outcome, closeErr = verifyNetwork(context.Background(), t.TempDir(), forged, now, &snapshot, &accepted)
	if closeErr != nil || outcome != alphacontrol.OutcomeInvalid || accepted {
		t.Fatalf("forged closed Epoch: outcome=%q, accepted=%v, close=%v", outcome, accepted, closeErr)
	}
}

func TestVerifyNetworkUsesMaintainedNetworkStateAcceptance(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	network := [32]byte{7}
	epoch, epochDigest := signedEmptyEpoch(network, now, private)
	body, err := encodeNetworkEvidence(NetworkEvidence{NetworkID: network, Profile: "h3-role-probe-v1", Threshold: 1,
		Authorities: []ed25519.PublicKey{public}, Epoch: epoch, EpochDigest: epochDigest})

	if err != nil {
		t.Fatal(err)
	}
	var snapshot state.Snapshot
	accepted := false
	stateRoot := t.TempDir()
	outcome, closeErr := verifyNetwork(context.Background(), stateRoot, body, now, &snapshot, &accepted)
	if closeErr != nil || outcome != alphacontrol.OutcomeAccepted || !accepted || snapshot.Epoch != 1 {
		t.Fatalf("network inspection = %q, accepted=%v, snapshot=%+v, close=%v", outcome, accepted, snapshot, closeErr)
	}
	if outcome, closeErr := verifyNetwork(context.Background(), stateRoot, body, now, &snapshot, &accepted); closeErr != nil || outcome != alphacontrol.OutcomeAccepted || !accepted || snapshot.Epoch != 1 {
		t.Fatalf("cached network inspection = %q, accepted=%v, snapshot=%+v, close=%v", outcome, accepted, snapshot, closeErr)
	}
	body[len(body)-1]++
	if outcome, closeErr := verifyNetwork(context.Background(), t.TempDir(), body, now, &snapshot, &accepted); outcome != alphacontrol.OutcomeInvalid || closeErr != nil {
		t.Fatalf("altered network evidence outcome = %q, close=%v", outcome, closeErr)
	}
}

func signedEmptyEpoch(network [32]byte, now time.Time, signer ed25519.PrivateKey) ([]byte, [32]byte) {
	inputRoot, viewRoot, rejectedRoot := sha256.Sum256([]byte{0x10}), sha256.Sum256([]byte{0x11}), sha256.Sum256([]byte{0x12})
	var raw bytes.Buffer
	raw.WriteString("AREP")
	raw.WriteByte(1)
	raw.Write(network[:])
	writeU64(&raw, 1)
	raw.Write(make([]byte, 32))
	writeU64(&raw, uint64(now.Add(-time.Minute).Unix()))
	writeU64(&raw, uint64(now.Add(time.Minute).Unix()))
	writeU32(&raw, 0)
	writeText(&raw, "h3-role-probe-v1")
	raw.Write(inputRoot[:])
	raw.Write(viewRoot[:])
	writeU32(&raw, 0)
	raw.Write(rejectedRoot[:])
	writeU32(&raw, 0)
	raw.Write(make([]byte, 32))
	writeText(&raw, "ardents-h3-role-domain-v1")
	writeU32(&raw, 0)
	writeU32(&raw, 0)
	writeU16(&raw, 0)
	writeU16(&raw, 0)
	writeU32(&raw, 0)
	raw.WriteByte(1)
	writeText(&raw, "alpha")
	writeU16(&raw, 0)
	writeU32(&raw, 0)
	unsigned := raw.Bytes()
	digest := sha256.Sum256(unsigned)
	public := signer.Public().(ed25519.PublicKey)
	id := sha256.Sum256(public)
	raw.WriteByte(1)
	raw.Write(id[:])
	raw.Write(ed25519.Sign(signer, digest[:]))
	return raw.Bytes(), digest
}

func writeText(buffer *bytes.Buffer, value string) {
	buffer.WriteByte(byte(len(value)))
	buffer.WriteString(value)
}

func writeU16(buffer *bytes.Buffer, value uint16) {
	var raw [2]byte
	binary.BigEndian.PutUint16(raw[:], value)
	buffer.Write(raw[:])
}

func writeU32(buffer *bytes.Buffer, value uint32) {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], value)
	buffer.Write(raw[:])
}

func writeU64(buffer *bytes.Buffer, value uint64) {
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], value)
	buffer.Write(raw[:])
}

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
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

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
	if outcome := verifyNetwork(context.Background(), stateRoot, body, now, &snapshot, &accepted); outcome != alphacontrol.OutcomeAccepted || !accepted || snapshot.Epoch != 1 {
		t.Fatalf("network inspection = %q, accepted=%v, snapshot=%+v", outcome, accepted, snapshot)
	}
	if outcome := verifyNetwork(context.Background(), stateRoot, body, now, &snapshot, &accepted); outcome != alphacontrol.OutcomeAccepted || !accepted || snapshot.Epoch != 1 {
		t.Fatalf("cached network inspection = %q, accepted=%v, snapshot=%+v", outcome, accepted, snapshot)
	}
	body[len(body)-1]++
	if outcome := verifyNetwork(context.Background(), t.TempDir(), body, now, &snapshot, &accepted); outcome != alphacontrol.OutcomeInvalid {
		t.Fatalf("altered network evidence outcome = %q", outcome)
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

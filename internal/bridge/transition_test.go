package bridge_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"
)

func TestTransitionPublishesOneContactBeforeCarrierWork(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	owner := fixture.open(t)
	invite := fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)
	if _, err := owner.Import(invite); err != nil {
		t.Fatal(err)
	}
	manifest := sha256.Sum256([]byte("route manifest"))
	frame := transitionFrame(manifest)
	identity, candidate, ordinal, err := owner.BeginContact(frame, manifest, fixture.now.Add(time.Minute))
	if err != nil || identity != fixture.members[0].identity || !bytes.Equal(candidate, fixture.candidate) || ordinal != 0 {
		t.Fatalf("begin contact = %x %x %d, %v", identity, candidate, ordinal, err)
	}
	if _, _, _, err := owner.BeginContact(frame, manifest, fixture.now.Add(time.Minute)); err == nil {
		t.Fatal("second transition obtained another contact")
	}
	if err := owner.FinishContact(ordinal, true, true); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner = fixture.open(t)
	t.Cleanup(func() { _ = owner.Close() })
	if _, _, _, err := owner.BeginContact(frame, manifest, fixture.now.Add(time.Minute)); err == nil {
		t.Fatal("restart reset the consumed transition")
	}
}

func TestTransitionRejectsMutationBeforeExposure(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	owner := fixture.open(t)
	t.Cleanup(func() { _ = owner.Close() })
	if _, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)); err != nil {
		t.Fatal(err)
	}
	manifest := sha256.Sum256([]byte("route manifest"))
	frame := transitionFrame(manifest)
	frame[len(frame)-1] ^= 1
	if _, _, _, err := owner.BeginContact(frame, manifest, fixture.now.Add(time.Minute)); err == nil {
		t.Fatal("manifest-mutated transition was accepted")
	}
	if _, _, _, err := owner.BeginContact(transitionFrame(manifest), manifest, fixture.now.Add(time.Minute)); err != nil {
		t.Fatalf("rejected transition consumed exposure: %v", err)
	}
}

func TestTransitionSkipsAbsentSlotWithoutRenumberingOrdinal(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	owner := fixture.open(t)
	t.Cleanup(func() { _ = owner.Close() })
	if _, err := owner.Import(fixture.invite(t, 1, 1, nil, fixture.notBefore, fixture.notAfter)); err != nil {
		t.Fatal(err)
	}
	manifest := sha256.Sum256([]byte("slot-one manifest"))
	_, _, ordinal, err := owner.BeginContact(transitionFrame(manifest), manifest, fixture.now.Add(time.Minute))
	if err != nil || ordinal != 2 {
		t.Fatalf("slot-one contact ordinal = %d, %v", ordinal, err)
	}
	if err := owner.FinishContact(ordinal, false, true); err != nil {
		t.Fatal(err)
	}
}

func transitionFrame(manifest [32]byte) []byte {
	var frame bytes.Buffer
	frame.WriteString("ardents-h3-bridge-transition-v1")
	frame.Write(bytes.Repeat([]byte{1}, 32))
	frame.WriteByte(1)
	frame.Write(bytes.Repeat([]byte{2}, 32))
	_ = binary.Write(&frame, binary.BigEndian, uint64(1))
	frame.Write(manifest[:])
	return frame.Bytes()
}

package bridge_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/bridge"
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
	identity, candidate, ordinal, started, _, err := owner.BeginContact(frame, manifest, fixture.now.Add(time.Minute))
	if err != nil || identity != fixture.members[0].identity || !bytes.Equal(candidate, fixture.candidate) ||
		ordinal != 0 || started != 1 {
		t.Fatalf("begin contact = %x %x %d %d, %v", identity, candidate, ordinal, started, err)
	}
	if _, _, _, _, _, err := owner.BeginContact(frame, manifest, fixture.now.Add(time.Minute)); err == nil {
		t.Fatal("second transition obtained another contact")
	}
	if err := owner.FinishContact(ordinal, 2, true, true); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner = fixture.open(t)
	t.Cleanup(func() { _ = owner.Close() })
	if _, _, _, _, _, err := owner.BeginContact(frame, manifest, fixture.now.Add(time.Minute)); err == nil {
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
	if _, _, _, _, _, err := owner.BeginContact(frame, manifest, fixture.now.Add(time.Minute)); err == nil {
		t.Fatal("manifest-mutated transition was accepted")
	}
	if _, _, _, _, _, err := owner.BeginContact(transitionFrame(manifest), manifest, fixture.now.Add(time.Minute)); err != nil {
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
	_, _, ordinal, _, _, err := owner.BeginContact(transitionFrame(manifest), manifest, fixture.now.Add(time.Minute))
	if err != nil || ordinal != 2 {
		t.Fatalf("slot-one contact ordinal = %d, %v", ordinal, err)
	}
	if err := owner.FinishContact(ordinal, 2, false, true); err != nil {
		t.Fatal(err)
	}
}

func TestTransitionSkipsExpiredSlotForEligibleLaterSlot(t *testing.T) {
	fixture := newFixture(t)
	now := fixture.now
	config := fixture.config()
	config.Clock = func() time.Time { return now }
	owner, err := bridge.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	expired := fixture.now.Add(time.Second)
	if result, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, expired)); err != nil || result.Class != "accepted" {
		t.Fatalf("slot zero = %+v, %v", result, err)
	}
	if result, err := owner.Import(fixture.inviteFor(t, 1, 1, 1, nil, fixture.notBefore, fixture.notAfter)); err != nil || result.Class != "accepted" {
		t.Fatalf("slot one = %+v, %v", result, err)
	}
	now = expired.Add(time.Second)
	manifest := sha256.Sum256([]byte("expired slot manifest"))
	identity, _, ordinal, _, _, err := owner.BeginContact(transitionFrame(manifest), manifest, time.Time{})
	if err != nil || ordinal != 2 || identity != fixture.members[1].identity {
		t.Fatalf("eligible later slot = %x ordinal %d, %v", identity, ordinal, err)
	}
}

func TestTransitionConsumesExactFourOrdinalSequence(t *testing.T) {
	for episode := range 5 {
		t.Run(strconv.Itoa(episode), testTransitionConsumesExactFourOrdinalSequence)
	}
}

func testTransitionConsumesExactFourOrdinalSequence(t *testing.T) {
	fixture := newFixture(t)
	owner := fixture.open(t)
	t.Cleanup(func() { _ = owner.Close() })
	for slot := byte(0); slot < 2; slot++ {
		if result, err := owner.Import(fixture.invite(t, slot, 1, nil, fixture.notBefore, fixture.notAfter)); err != nil || result.Class != "accepted" {
			t.Fatalf("import slot %d = %+v, %v", slot, result, err)
		}
	}
	manifest := sha256.Sum256([]byte("four-contact manifest"))
	_, _, ordinal, started, _, err := owner.BeginContact(transitionFrame(manifest), manifest, fixture.now.Add(time.Minute))
	if err != nil || ordinal != 0 || started != 1 {
		t.Fatalf("first contact = %d %d, %v", ordinal, started, err)
	}
	for want := byte(0); want < 4; want++ {
		terminal := uint64(want+1) * uint64(time.Second)
		if err := owner.FinishContact(want, terminal, false, true); err != nil {
			t.Fatalf("finish ordinal %d: %v", want, err)
		}
		if want == 3 {
			break
		}
		_, _, got, err := owner.NextContact(context.Background())
		if err != nil || got != want+1 {
			t.Fatalf("next after %d = %d, %v", want, got, err)
		}
	}
	if _, _, _, err := owner.NextContact(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "bridge-attempt-exhausted") {
		t.Fatal("fifth contact was exposed")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner = fixture.open(t)
	if _, _, _, _, _, err := owner.BeginContact(transitionFrame(manifest), manifest, time.Time{}); err == nil ||
		!strings.Contains(err.Error(), "bridge-attempt-exhausted") {
		t.Fatalf("restart lost durable exhaustion: %v", err)
	}
}

func TestRestartDurablyInterruptsUnfinishedAttempt(t *testing.T) {
	fixture := newFixture(t)
	owner := fixture.open(t)
	if _, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)); err != nil {
		t.Fatal(err)
	}
	manifest := sha256.Sum256([]byte("interrupted manifest"))
	if _, _, _, _, _, err := owner.BeginContact(transitionFrame(manifest), manifest, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner = fixture.open(t)
	t.Cleanup(func() { _ = owner.Close() })
	if _, _, _, _, _, err := owner.BeginContact(transitionFrame(manifest), manifest, time.Time{}); err == nil ||
		!strings.Contains(err.Error(), "bridge-interrupted") {
		t.Fatalf("restart lost durable interruption: %v", err)
	}
}

func TestParentDeadlineIsDurableAndStopsLaterContacts(t *testing.T) {
	for episode := range 5 {
		t.Run(strconv.Itoa(episode), testParentDeadlineIsDurableAndStopsLaterContacts)
	}
}

func testParentDeadlineIsDurableAndStopsLaterContacts(t *testing.T) {
	fixture := newFixture(t)
	owner := fixture.open(t)
	if _, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)); err != nil {
		t.Fatal(err)
	}
	manifest := sha256.Sum256([]byte("parent deadline manifest"))
	parent := fixture.now.Add(15 * time.Second)
	_, _, ordinal, _, deadline, err := owner.BeginContact(transitionFrame(manifest), manifest, parent)
	if err != nil || deadline != parent {
		t.Fatalf("clipped attempt deadline = %s, %v", deadline, err)
	}
	if err := owner.FinishContact(ordinal, 2, false, true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, err := owner.NextContact(ctx); err == nil || !strings.Contains(err.Error(), "bridge-deadline-exceeded") {
		t.Fatalf("cancelled parent did not terminalize attempt: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner = fixture.open(t)
	t.Cleanup(func() { _ = owner.Close() })
	if _, _, _, _, _, err := owner.BeginContact(transitionFrame(manifest), manifest, time.Time{}); err == nil ||
		!strings.Contains(err.Error(), "bridge-deadline-exceeded") {
		t.Fatalf("restart lost durable parent deadline: %v", err)
	}
}

func TestInviteExpiryClipsAttemptDeadline(t *testing.T) {
	fixture := newFixture(t)
	owner := fixture.open(t)
	t.Cleanup(func() { _ = owner.Close() })
	expires := fixture.now.Add(9 * time.Second)
	if _, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, expires)); err != nil {
		t.Fatal(err)
	}
	manifest := sha256.Sum256([]byte("expiry-bound manifest"))
	_, _, _, _, deadline, err := owner.BeginContact(transitionFrame(manifest), manifest, time.Time{})
	if err != nil || !deadline.Equal(expires.Truncate(time.Second)) {
		t.Fatalf("Invite-clipped deadline = %s, %v", deadline, err)
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

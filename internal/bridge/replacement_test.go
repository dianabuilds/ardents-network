package bridge_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"time"
)

func TestReplacementCannotEraseLiveAttemptConfiguration(t *testing.T) {
	fixture := newFixture(t)
	owner := fixture.open(t)
	t.Cleanup(func() { _ = owner.Close() })
	first, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter))
	if err != nil || first.Class != "accepted" {
		t.Fatalf("initial import = %+v, %v", first, err)
	}
	manifest := sha256.Sum256([]byte("live replacement manifest"))
	_, _, ordinal, _, _, err := owner.BeginContact(transitionFrame(manifest), manifest, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	replacement := fixture.inviteFor(t, 2, 0, 2, &first.InviteID, fixture.notBefore, fixture.notAfter)
	if result, err := owner.Import(replacement); err != nil || result.Class != "accepted" {
		t.Fatalf("live replacement = %+v, %v", result, err)
	}
	if err := owner.FinishContact(ordinal, uint64(time.Second), false, true); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := owner.NextContact(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "bridge-attempt-exhausted") {
		t.Fatalf("replacement attempt terminal = %v", err)
	}
	identity, candidate, err := owner.Contact()
	if err != nil || identity != fixture.members[2].identity || !bytes.Equal(candidate, fixture.candidate) {
		t.Fatalf("activated replacement = %x %x, %v", identity, candidate, err)
	}
}

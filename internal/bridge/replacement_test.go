package bridge_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/bridge"
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

func TestExpiredVerifiedReplacementCannotActivateAfterLiveWork(t *testing.T) {
	fixture := newFixture(t)
	now := fixture.now
	config := fixture.config()
	config.Clock = func() time.Time { return now }
	owner, err := bridge.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	first, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter))
	if err != nil || first.Class != "accepted" {
		t.Fatalf("initial import = %+v, %v", first, err)
	}
	manifest := sha256.Sum256([]byte("expired live replacement manifest"))
	_, _, ordinal, _, _, err := owner.BeginContact(transitionFrame(manifest), manifest, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	expires := now.Add(time.Second)
	replacement := fixture.inviteFor(t, 2, 0, 2, &first.InviteID, fixture.notBefore, expires)
	if result, importErr := owner.Import(replacement); importErr != nil || result.Class != "accepted" {
		t.Fatalf("live replacement = %+v, %v", result, importErr)
	}
	now = expires.Add(time.Second)
	if err := owner.FinishContact(ordinal, uint64(time.Second), false, false); err != nil {
		t.Fatal(err)
	}
	if _, _, contactErr := owner.Contact(); contactErr == nil {
		t.Fatal("expired replacement became contactable after live work")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner, err = bridge.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, contactErr := owner.Contact(); contactErr == nil {
		t.Fatal("expired replacement became contactable after restart")
	}
}

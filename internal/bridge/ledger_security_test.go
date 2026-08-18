package bridge_test

import (
	"context"
	"crypto/sha256"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestG9RejectsRepeatedTransitionAndPrematureRetryWithoutLedgerMutation(t *testing.T) {
	fixture := newFixture(t)
	owner := fixture.open(t)
	t.Cleanup(func() { _ = owner.Close() })
	for slot := byte(0); slot < 2; slot++ {
		if _, err := owner.Import(fixture.invite(t, slot, 1, nil, fixture.notBefore, fixture.notAfter)); err != nil {
			t.Fatal(err)
		}
	}
	manifest := sha256.Sum256([]byte("G9 ledger command rejection"))
	frame := transitionFrame(manifest)
	_, _, ordinal, _, _, err := owner.BeginContact(frame, manifest, fixture.now.Add(time.Minute))
	if err != nil || ordinal != 0 {
		t.Fatalf("initial contact ordinal=%d err=%v", ordinal, err)
	}
	before := durableFiles(t, fixture.root)
	if _, _, _, _, _, err := owner.BeginContact(frame, manifest, fixture.now.Add(time.Minute)); err == nil ||
		!strings.Contains(err.Error(), "bridge-local-denial") {
		t.Fatalf("second regime transition = %v", err)
	}
	if _, _, _, err := owner.NextContact(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "bridge-local-denial") {
		t.Fatalf("retry before initial completion = %v", err)
	}
	if after := durableFiles(t, fixture.root); !reflect.DeepEqual(before, after) {
		t.Fatal("rejected G9 commands changed the durable Bridge ledger")
	}
	if err := owner.FinishContact(ordinal, uint64(time.Second), false, true); err != nil {
		t.Fatal(err)
	}
	_, _, next, err := owner.NextContact(context.Background())
	if err != nil || next != 1 {
		t.Fatalf("first retry ordinal=%d err=%v", next, err)
	}
	before = durableFiles(t, fixture.root)
	if _, _, _, err := owner.NextContact(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "bridge-local-denial") {
		t.Fatalf("duplicate retry command = %v", err)
	}
	if after := durableFiles(t, fixture.root); !reflect.DeepEqual(before, after) {
		t.Fatal("duplicate retry command changed the durable Bridge ledger")
	}
}

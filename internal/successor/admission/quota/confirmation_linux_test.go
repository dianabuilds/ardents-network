//go:build linux

package quota

import "github.com/dianabuilds/ardents-network/internal/successor/admission"

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestUncertainDebitNeverCreatesConfirmation(t *testing.T) {
	l, _, _ := newTestLedger(t, 1)
	defer l.Close()
	raw, f, _ := batchFixture(t, 2, 1, 1, 1)
	l.state.fault = func(phase string) error {
		if phase == "journal-close" {
			return errors.New("injected close")
		}
		return nil
	}
	outcome, c := l.DebitVerified(t.Context(), raw, f, Bootstrap)
	if outcome != Uncertain {
		t.Fatal(outcome)
	}
	if _, _, _, _, ok := c.Snapshot(); ok {
		t.Fatal("uncertain grant")
	}
}

func TestDebitConfirmationRequiresDurableCommit(t *testing.T) {
	l, _, _ := newTestLedger(t, 1)
	defer l.Close()
	raw, f, _ := batchFixture(t, 2, 1, 1, 1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	outcome, c := l.DebitVerified(ctx, raw, f, Bootstrap)
	if outcome != admission.Canceled {
		t.Fatal(outcome)
	}
	if _, _, _, _, ok := c.Snapshot(); ok {
		t.Fatal("canceled grant")
	}
	outcome, c = l.DebitVerified(t.Context(), raw, f, Bootstrap)
	if outcome != Debited {
		t.Fatal(outcome)
	}
	snapshot, b, _, _, ok := c.Snapshot()
	if !ok {
		t.Fatal("missing grant")
	}
	snapshot[0] ^= 1
	b.Keys[0].SPKI[0] ^= 1
	raw[0] ^= 1
	owned, b2, _, _, ok := c.Snapshot()
	if !ok || owned[0] == raw[0] || b2.Keys[0].SPKI[0] == b.Keys[0].SPKI[0] {
		t.Fatal("mutable grant")
	}
	outcome, repeat := l.DebitVerified(t.Context(), owned, f, Bootstrap)
	if outcome != AlreadyDebited {
		t.Fatal(outcome)
	}
	if _, _, _, _, ok := repeat.Snapshot(); !ok {
		t.Fatal("missing retry grant")
	}
	f.Now = f.Now.Add(time.Hour)
	outcome, c = l.DebitVerified(t.Context(), owned, f, Bootstrap)
	if outcome != admission.Validity {
		t.Fatal(outcome)
	}
	if _, _, _, _, ok := c.Snapshot(); ok {
		t.Fatal("expired grant")
	}
}

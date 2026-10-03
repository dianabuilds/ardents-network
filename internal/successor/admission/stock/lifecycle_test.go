//go:build linux

package stock

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func nextIntent(h *spendHostFixture) IssuanceIntent {
	p := h.profile
	return IssuanceIntent{Challenges: []token.ClosedTokenContext{{NetworkID: p.NetworkID, ProfileDigest: p.Digest, IssuerNodeID: p.IssuerNodeID, ReceiverNodeID: fixtureID(6), ReceiverDutyGeneration: 8, Class: 2, WindowStart: h.now.Truncate(time.Hour)}},
		Selection: ExchangeBinding{ID: fixtureID(11), ProfileDigest: p.Digest}, Bootstrap: true, Deadline: h.now.Add(time.Minute)}
}
func TestHolderRetryRevocationAndSingleCompletion(t *testing.T) {
	o, h, _ := issuedStockFixture(t)
	intent := nextIntent(h)
	a, err := o.Begin(intent)
	if err != nil {
		t.Fatal(err)
	}
	raw, end, err := a.Request()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.Begin(intent); err == nil {
		t.Fatal("parallel attempt admitted")
	}
	before := o.Status()
	cause := errors.New("transport interrupted")
	if err := a.Complete(nil, cause); !errors.Is(err, cause) {
		t.Fatal(err)
	}
	changed := intent
	changed.Selection.ID[0]++
	if _, err := o.Begin(changed); err == nil {
		t.Fatal("retry changed delivery")
	}
	intent.Deadline = end.Add(time.Minute)
	retry, err := o.Begin(intent)
	if err != nil {
		t.Fatal(err)
	}
	again, retryEnd, err := retry.Request()
	if err != nil || string(again) != string(raw) || retryEnd != end || o.Status().Remaining != before.Remaining {
		t.Fatal("retry changed reservation/request/deadline", err)
	}
	copyOfRetry := retry
	if err := o.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := retry.Request(); err == nil {
		t.Fatal("revoked request accessible")
	}
	if err := copyOfRetry.Complete(nil, nil); err == nil {
		t.Fatal("revoked response accepted")
	}
	if o.Status().Accepted || o.Status().Pending {
		t.Fatal("revocation retained secrets")
	}
	if _, _, err := o.Request([3]uint32{1, 0, 0}); err == nil {
		t.Fatal("closed holder revived")
	}
}

func TestCurrentIssuerChangesAfterDebitDoNotRefund(t *testing.T) {
	o, h, _ := issuedStockFixture(t)
	a, err := o.Begin(nextIntent(h))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Discard()
	raw, _, err := a.Request()
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(h.plan.AdmissionRoot, "admission.journal")
	before, err := os.Stat(journal)
	if err != nil {
		t.Fatal(err)
	}
	observer := func() (admission.AuthorityFacts, time.Time, error) {
		p, now, err := h.ProfileLocked()
		after, statErr := os.Stat(journal)
		if statErr != nil {
			return p, now, statErr
		}
		if after.Size() > before.Size() {
			p.StateGeneration[0]++
		}
		return p, now, err
	}
	result := issuer.IssueCurrent(context.Background(), h.plan, raw, quota.Bootstrap, observer)
	if result.Outcome != "authority-unavailable" || result.Response != nil {
		t.Fatalf("stale issuance=%+v", result)
	}
	after, err := os.Stat(journal)
	if err != nil || after.Size() <= before.Size() {
		t.Fatal("debit missing", err)
	}
	retry := issuer.IssueCurrent(context.Background(), h.plan, raw, quota.Bootstrap, h.ProfileLocked)
	if retry.Outcome != "issued-offline" {
		t.Fatalf("exact retained debit cannot finish: %+v", retry)
	}
	afterRetry, _ := os.Stat(journal)
	if afterRetry.Size() != after.Size() {
		t.Fatal("exact retry debited again")
	}
	if err := a.Complete(retry.Response, nil); err != nil {
		t.Fatal(err)
	}
	if err := a.Complete(retry.Response, nil); err == nil {
		t.Fatal("duplicate completion")
	}
}

func TestHolderClockRollbackRefusesWithoutConsuming(t *testing.T) {
	o, h, p := issuedStockFixture(t)
	h.now = h.now.Add(-time.Second)
	if _, err := o.Take(context.Background(), p, 2); err == nil {
		t.Fatal("clock rollback accepted")
	}
	h.now = h.now.Add(time.Second)
	if _, err := o.Take(context.Background(), p, 2); err != nil {
		t.Fatal("refusal consumed stock", err)
	}
}

func TestIssuerWithholdsCommittedResponseAfterPermissionHour(t *testing.T) {
	o, h, _ := issuedStockFixtureHours(t, 2)
	a, err := o.Begin(nextIntent(h))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Discard()
	raw, _, err := a.Request()
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(h.plan.ResultRoot, "results.journal")
	before, err := os.Stat(journal)
	if err != nil {
		t.Fatal(err)
	}
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		p, now, err := h.ProfileLocked()
		after, statErr := os.Stat(journal)
		if statErr != nil {
			return p, now, statErr
		}
		if after.Size() > before.Size() {
			now = now.Truncate(time.Hour).Add(time.Hour)
		}
		return p, now, err
	}
	r := issuer.IssueCurrent(context.Background(), h.plan, raw, quota.Bootstrap, observe)
	if r.Outcome != "authority-unavailable" || r.Response != nil {
		t.Fatalf("expired permission response escaped: %+v", r)
	}
	after, err := os.Stat(journal)
	if err != nil || after.Size() <= before.Size() {
		t.Fatal("result was not committed", err)
	}
	// The retained exact response remains readable under the original valid
	// observation; storage was not rolled back by suppressing response export.
	retry := issuer.IssueCurrent(context.Background(), h.plan, raw, quota.Bootstrap, h.ProfileLocked)
	if retry.Outcome != "already-issued" {
		t.Fatalf("retained result lost: %+v", retry)
	}
}

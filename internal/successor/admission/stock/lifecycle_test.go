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
	"sync/atomic"
	"testing"
	"time"
)

func nextIntent(h *spendHostFixture) IssuanceIntent {
	p := h.profile
	return IssuanceIntent{Challenges: []token.ClosedTokenContext{{NetworkID: p.NetworkID, ProfileDigest: p.Digest, IssuerNodeID: p.IssuerNodeID, ReceiverNodeID: fixtureID(6), ReceiverDutyGeneration: 8, Class: 2, WindowStart: h.now.Truncate(time.Hour)}},
		Selection: ExchangeBinding{ID: fixtureID(11), ProfileDigest: p.Digest}, Bootstrap: true, Deadline: h.now.Add(time.Minute)}
}

func TestHolderBoundCompletionRefusesVerifiedTokensAfterOriginalLifetimeRetires(t *testing.T) {
	for _, retired := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "retired"}[retired], func(t *testing.T) {
			o, h, presentation := issuedStockFixture(t)
			original, err := o.Take(t.Context(), presentation, 2)
			if err != nil {
				t.Fatal(err)
			}
			clear(original)
			a, err := o.Begin(nextIntent(h))
			if err != nil {
				t.Fatal(err)
			}
			raw, _, err := a.Request()
			if err != nil {
				t.Fatal(err)
			}
			defer clear(raw)
			issued := issuer.IssueCurrent(t.Context(), h.plan, raw, quota.Bootstrap, h.ProfileLocked)
			if issued.Outcome != "issued-offline" {
				t.Fatal(issued.Outcome)
			}
			defer clear(issued.Response)
			before := o.Status().Remaining
			cause := errors.New("original opening retired")
			checked := 0
			err = a.CompleteBound(issued.Response, nil, func() error {
				checked++
				if retired {
					return cause
				}
				return nil
			})
			if checked != 1 || retired && !errors.Is(err, cause) || !retired && err != nil {
				t.Fatal("original finalization guard lost", checked, err)
			}
			if o.Status().Remaining != before || o.Status().Pending {
				t.Fatal("failed completion refunded or retained batch")
			}
			got, takeErr := o.Take(t.Context(), presentation, 2)
			defer clear(got)
			if retired && takeErr == nil || !retired && takeErr != nil {
				t.Fatal("verified stock crossed original lifetime", retired, takeErr)
			}
			if err := a.CompleteBound(issued.Response, nil, func() error { return nil }); err == nil {
				t.Fatal("replacement guard revived completed batch")
			}
		})
	}
}

// Genuine blind issuance is retained; only the final observation failure is
// injected. Supplied fixture authority facts do not establish Network authenticity.
func TestHolderBoundCompletionRechecksAuthorityAfterVerification(t *testing.T) {
	o, h, presentation := issuedStockFixture(t)
	original, err := o.Take(t.Context(), presentation, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(original)
	a, err := o.Begin(nextIntent(h))
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := a.Request()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(raw)
	issued := issuer.IssueCurrent(t.Context(), h.plan, raw, quota.Bootstrap, h.ProfileLocked)
	if issued.Outcome != "issued-offline" {
		t.Fatal(issued.Outcome)
	}
	defer clear(issued.Response)
	before := o.Status().Remaining
	cause := errors.New("State lost after signature verification")
	observe := o.observe
	observations, guards := 0, 0
	o.observe = func() (admission.AuthorityFacts, time.Time, error) {
		observations++
		if observations == 2 {
			return admission.AuthorityFacts{}, time.Time{}, cause
		}
		return observe()
	}
	err = a.CompleteBound(issued.Response, nil, func() error { guards++; return nil })
	o.observe = observe
	if !errors.Is(err, cause) || observations != 2 || guards != 0 {
		t.Fatal("final State observation lost", err, observations, guards)
	}
	if o.Status().Pending || o.Status().Remaining != before {
		t.Fatal("failed finalization retained or refunded batch")
	}
	got, err := o.Take(t.Context(), presentation, 2)
	clear(got)
	if err == nil {
		t.Fatal("obsolete verified tokens deposited")
	}
	if err := a.CompleteBound(issued.Response, nil, func() error { return nil }); err == nil {
		t.Fatal("replacement revived obsolete completion")
	}
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

// These component controls use genuine Permission, blind preparation, RSA,
// quota and result journals. Supplied authority isolates the Admission boundary;
// actual signed Network and native Node acceptance remain command scenarios.
func retainedIssuerRequest(t *testing.T) (*Owner, *spendHostFixture, *Attempt, []byte) {
	t.Helper()
	holder, host, _ := issuedStockFixture(t)
	a, err := holder.Begin(nextIntent(host))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Discard() })
	raw, _, err := a.Request()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(raw) })
	return holder, host, &a, raw
}

func TestRetainedIssuerHealthyRequestRefusalsAndExactRetry(t *testing.T) {
	_, h, a, raw := retainedIssuerRequest(t)
	o, opened := issuer.OpenCurrent(t.Context(), h.plan, h.ProfileLocked)
	if opened.Outcome != "opened" {
		t.Fatal(opened)
	}
	t.Cleanup(func() {
		if r := o.Close(context.Background()); r.Outcome != "closed" {
			t.Error(r)
		}
	})
	if r := o.IssueCurrent(t.Context(), nil, quota.Bootstrap); r.Outcome != "invalid-input" {
		t.Fatal(r)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if r := o.IssueCurrent(canceled, raw, quota.Bootstrap); r.Outcome != "canceled" {
		t.Fatal(r)
	}
	if err := o.Check(); err != nil {
		t.Fatal("request refusal retired owner", err)
	}
	r := o.IssueCurrent(t.Context(), raw, quota.Bootstrap)
	if r.Outcome != "issued-offline" {
		t.Fatal(r)
	}
	defer clear(r.Response)
	journal := filepath.Join(h.plan.AdmissionRoot, "admission.journal")
	before, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	again := o.IssueCurrent(t.Context(), raw, quota.Bootstrap)
	defer clear(again.Response)
	if again.Outcome != "already-issued" || string(again.Response) != string(r.Response) {
		t.Fatal("exact response changed", again.Outcome)
	}
	wrong := o.IssueCurrent(t.Context(), raw, quota.Admitted)
	if wrong.Outcome != "request-conflict" || wrong.Response != nil {
		t.Fatal(wrong)
	}
	after, err := os.ReadFile(journal)
	if err != nil || string(before) != string(after) {
		t.Fatal("retry changed quota", err)
	}
	if err := a.Complete(r.Response, nil); err != nil {
		t.Fatal("genuine finalization", err)
	}
	if err := o.Check(); err != nil {
		t.Fatal("conflict retired healthy owner", err)
	}
	if closed := o.Close(context.Background()); closed.Outcome != "closed" || o.Err() != nil {
		t.Fatal("healthy joined close", closed, o.Err())
	}
}

func TestRetainedIssuerBusyAndCloseRetainRootsUntilOriginalObserverJoins(t *testing.T) {
	_, h, _, raw := retainedIssuerRequest(t)
	journal := filepath.Join(h.plan.AdmissionRoot, "admission.journal")
	before, err := os.Stat(journal)
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var held atomic.Bool
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		info, err := os.Stat(journal)
		if err != nil {
			return admission.AuthorityFacts{}, time.Time{}, err
		}
		if info.Size() > before.Size() && held.CompareAndSwap(false, true) {
			close(entered)
			<-release
		}
		return h.ProfileLocked()
	}
	o, opened := issuer.OpenCurrent(t.Context(), h.plan, observe)
	if opened.Outcome != "opened" {
		t.Fatal(opened)
	}
	finished := make(chan issuer.Result, 1)
	go func() { finished <- o.IssueCurrent(t.Context(), raw, quota.Bootstrap) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("post-debit observer not reached")
	}
	if r := o.IssueCurrent(t.Context(), raw, quota.Bootstrap); r.Outcome != "busy" {
		t.Fatal(r)
	}
	wait, cancel := context.WithCancel(t.Context())
	cancel()
	if r := o.Close(wait); r.Outcome != "completion-unavailable" {
		t.Fatal(r)
	}
	if l, err := quota.Open(h.plan.AdmissionRoot, h.plan.AdmissionBinding); !errors.Is(err, quota.ErrBusy) {
		if l != nil {
			l.Close()
		}
		t.Fatal("root released before join", err)
	}
	close(release)
	r := <-finished
	if r.Response != nil {
		t.Fatal("late response crossed original close")
	}
	closed := o.Close(t.Context())
	if closed.Outcome != "closed" {
		t.Fatal(closed)
	}
	if again := o.Close(t.Context()); again.Phase != closed.Phase || again.Outcome != closed.Outcome || again.Cleanup != closed.Cleanup {
		t.Fatal("unstable retirement")
	}
	fresh := issuer.IssueCurrent(t.Context(), h.plan, raw, quota.Bootstrap, h.ProfileLocked)
	defer clear(fresh.Response)
	if fresh.Outcome != "issued-offline" {
		t.Fatal("committed debit was not retryable", fresh)
	}
	after, err := os.Stat(journal)
	if err != nil || after.Size() == before.Size() {
		t.Fatal("debit refunded", err)
	}
}

func TestRetainedIssuerCommittedResultAuthorityLossCannotRevive(t *testing.T) {
	_, h, _, raw := retainedIssuerRequest(t)
	journal := filepath.Join(h.plan.ResultRoot, "results.journal")
	before, err := os.Stat(journal)
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("original State lost after durable result")
	var loss atomic.Bool
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		info, err := os.Stat(journal)
		if err != nil {
			return admission.AuthorityFacts{}, time.Time{}, err
		}
		if info.Size() > before.Size() && loss.Load() {
			return admission.AuthorityFacts{}, time.Time{}, cause
		}
		return h.ProfileLocked()
	}
	o, opened := issuer.OpenCurrent(t.Context(), h.plan, observe)
	if opened.Outcome != "opened" {
		t.Fatal(opened)
	}
	loss.Store(true)
	r := o.IssueCurrent(t.Context(), raw, quota.Bootstrap)
	if r.Response != nil || r.Outcome != "authority-unavailable" || !errors.Is(o.Err(), cause) {
		t.Fatal("lost original cause/withholding", r, o.Err())
	}
	loss.Store(false)
	if err := o.Check(); err == nil {
		t.Fatal("restored observation revived owner")
	}
	closed := o.Close(t.Context())
	if closed.Outcome != "authority-unavailable" || !errors.Is(o.Err(), cause) {
		t.Fatal("retirement lost first failure", closed)
	}
	fresh := issuer.IssueCurrent(t.Context(), h.plan, raw, quota.Bootstrap, h.ProfileLocked)
	defer clear(fresh.Response)
	if fresh.Outcome != "already-issued" {
		t.Fatal("committed result lost on independent reopen", fresh)
	}
}

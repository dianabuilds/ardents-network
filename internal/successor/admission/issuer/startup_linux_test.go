//go:build linux

package issuer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
)

func planAuthority(p Plan) admission.AuthorityFacts {
	b := p.AdmissionBinding
	f := admission.AuthorityFacts{NetworkID: b.Network, IssuerNodeID: b.Issuer, IssuanceAuthorityKey: b.Authority, Digest: b.Profile,
		StateGeneration: [32]byte{6}, StateDigest: [32]byte{7}, IssuerDutyGeneration: b.Duty, NotBefore: b.Start, NotAfter: b.End, TokenKeyCount: uint8(len(b.Keys))}
	for i, k := range b.Keys {
		f.TokenKeys[i].Class = k.Class
		f.TokenKeys[i].WindowStart = time.Unix(int64(k.Window), 0).UTC()
		copy(f.TokenKeys[i].SPKI[:], k.SPKI)
	}
	return f
}

func TestRetainedIssuerPartialStartupJoinsEveryAcquiredLease(t *testing.T) {
	for _, missing := range []string{"keys", "results"} {
		t.Run(missing, func(t *testing.T) {
			p := issuancePlan(t)
			if r := Initialize(t.Context(), p); r.Outcome != "initialized-results" {
				t.Fatal(r)
			}
			path := p.KeyRoot
			if missing == "results" {
				path = p.ResultRoot
			}
			moved := filepath.Join(filepath.Dir(path), "held")
			if err := os.Rename(path, moved); err != nil {
				t.Fatal(err)
			}
			f := planAuthority(p)
			rejected, opened := OpenCurrent(t.Context(), p, func() (admission.AuthorityFacts, time.Time, error) { return f, f.NotBefore, nil })
			if opened.Outcome == "opened" || opened.Cleanup[2].Outcome != "closed" {
				t.Fatal("partial startup accepted or leaked quota", opened)
			}
			if missing == "results" && opened.Cleanup[1].Outcome != "closed" {
				t.Fatal("partial startup leaked keys", opened)
			}
			if rejected.Err() == nil {
				t.Fatal("rejected acquisition lost original cause")
			}
			if err := os.Rename(moved, path); err != nil {
				t.Fatal(err)
			}
			ledger, err := quota.Open(p.AdmissionRoot, p.AdmissionBinding)
			if err != nil {
				t.Fatal(err)
			}
			if err := ledger.Close(); err != nil {
				t.Fatal(err)
			}
			keys, err := issuance.Open(t.Context(), p.KeyRoot, p.KeyBinding)
			if err != nil {
				t.Fatal(err)
			}
			if err := keys.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRetainedIssuerOriginalCancellationIsRetainedWithoutAnotherCheck(t *testing.T) {
	p := issuancePlan(t)
	if r := Initialize(t.Context(), p); r.Outcome != "initialized-results" {
		t.Fatal(r)
	}
	f := planAuthority(p)
	ctx, cancel := context.WithCancel(t.Context())
	o, opened := OpenCurrent(ctx, p, func() (admission.AuthorityFacts, time.Time, error) { return f, f.NotBefore, nil })
	if opened.Outcome != "opened" {
		cancel()
		t.Fatal(opened)
	}
	copyOwner := o
	cancel()
	r := o.Close(context.Background())
	if r.Outcome != "canceled" || !errors.Is(o.Err(), context.Canceled) {
		t.Fatal("original cause replaced by normal close", r, o.Err())
	}
	if again := copyOwner.Close(ctx); again.Outcome != r.Outcome || again.Cleanup != r.Cleanup {
		t.Fatal("expired waiter forgot joined result", again)
	}
}

func TestRetainedIssuerDenyWithoutFailureRetainsRootsUntilClose(t *testing.T) {
	p := issuancePlan(t)
	if r := Initialize(t.Context(), p); r.Outcome != "initialized-results" {
		t.Fatal(r)
	}
	f := planAuthority(p)
	o, opened := OpenCurrent(t.Context(), p, func() (admission.AuthorityFacts, time.Time, error) { return f, f.NotBefore, nil })
	if opened.Outcome != "opened" {
		t.Fatal(opened)
	}
	o.Deny(nil)
	if r := o.IssueCurrent(t.Context(), nil, quota.Bootstrap); r.Outcome != "closed" || r.Response != nil {
		t.Fatal("unbounded/accepting denial", r)
	}
	if _, err := quota.Open(p.AdmissionRoot, p.AdmissionBinding); !errors.Is(err, quota.ErrBusy) {
		t.Fatal("Deny released original roots", err)
	}
	closed := o.Close(context.Background())
	if closed.Outcome != "closed" || o.Err() != nil {
		t.Fatal("normal denial acquired a fake failure", closed, o.Err())
	}
}

func TestRetainedIssuerCloseJoinsChecksOutsideLifecycleMutex(t *testing.T) {
	p := issuancePlan(t)
	if r := Initialize(t.Context(), p); r.Outcome != "initialized-results" {
		t.Fatal(r)
	}
	f := planAuthority(p)
	entered, release := make(chan struct{}), make(chan struct{})
	var o Owner
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		if o.state != nil {
			close(entered)
			<-release
		}
		return f, f.NotBefore, nil
	}
	var opened Result
	o, opened = OpenCurrent(t.Context(), p, observe)
	if opened.Outcome != "opened" {
		t.Fatal(opened)
	}
	checked := make(chan error, 1)
	go func() { checked <- o.Check() }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("observer not reached")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if r := o.Close(canceled); r.Outcome != "completion-unavailable" {
		t.Fatal("close did not retain observer", r)
	}
	if _, err := quota.Open(p.AdmissionRoot, p.AdmissionBinding); !errors.Is(err, quota.ErrBusy) {
		t.Fatal("early lease return", err)
	}
	close(release)
	if err := <-checked; err == nil {
		t.Fatal("late observation accepted")
	}
	if r := o.Close(context.Background()); r.Outcome != "closed" {
		t.Fatal(r)
	}
}

//go:build linux

package issuer

import "github.com/dianabuilds/ardents-network/internal/successor/admission/quota"

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
)

// The public operation receives cancellation after it holds all selected root
// leases. At that boundary the test removes the pins before canceling, so the
// real owners encounter close failures; no domain owner is mocked.
type canceledWithRetiredRoots struct {
	context.Context
	once        sync.Once
	t           *testing.T
	locks, pins []string
	cancel      context.CancelFunc
}

func (c *canceledWithRetiredRoots) Err() error {
	if c.Context.Err() != nil {
		return c.Context.Err()
	}
	for _, path := range c.locks {
		f, err := os.Open(path)
		if err != nil {
			c.t.Fatal(err)
		}
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			unlock := syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			if e := errors.Join(unlock, f.Close()); e != nil {
				c.t.Fatal(e)
			}
			return nil
		}
		closeErr := f.Close()
		if !errors.Is(err, syscall.EWOULDBLOCK) || closeErr != nil {
			c.t.Fatal(err, closeErr)
		}
	}
	c.once.Do(func() {
		for _, path := range c.pins {
			if err := os.Remove(path); err != nil {
				c.t.Fatal(err)
			}
		}
		c.cancel()
	})
	return c.Context.Err()
}

func TestIssueRetainsPrimaryResultWhenMultipleOwnersFailClose(t *testing.T) {
	p := issuancePlan(t)
	if got := Initialize(t.Context(), p); got.Outcome != "initialized-results" {
		t.Fatal(got)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	interrupted := &canceledWithRetiredRoots{Context: ctx, cancel: cancel, t: t,
		locks: []string{filepath.Join(p.AdmissionRoot, "admission.lock"), filepath.Join(p.KeyRoot, "issuer.lock"), filepath.Join(p.ResultRoot, "results.lock")},
		pins:  []string{filepath.Join(p.KeyRoot, "issuer.pin"), filepath.Join(p.ResultRoot, "results.pin")},
	}
	got := Issue(interrupted, p, nil, admission.Facts{}, quota.Bootstrap)
	if ctx.Err() == nil {
		t.Fatal("operation did not reach the acquired-root boundary")
	}
	if got.Phase != "debit" || got.Outcome != "canceled" || got.Response != nil {
		t.Fatalf("primary operation result was erased by cleanup: %+v", got)
	}
	want := [3]Completion{{"close-results", "storage-unavailable"}, {"close-keys", "storage-unavailable"}, {"close-admission", "closed"}}
	if got.Cleanup != want {
		t.Fatalf("cleanup results = %+v, want %+v", got.Cleanup, want)
	}
	if phase, outcome := got.Status(); phase != "close-keys" || outcome != "storage-uncertain" {
		t.Fatalf("command projection = %s/%s", phase, outcome)
	}
	for _, path := range interrupted.locks {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err != nil {
			_ = file.Close()
			t.Fatal("cleanup retained root lease", err)
		}
		if err = errors.Join(syscall.Flock(int(file.Fd()), syscall.LOCK_UN), file.Close()); err != nil {
			t.Fatal(err)
		}
	}
}

// Exercise retirement with a nonempty completed response, using a real owner
// close failure. The full canceled-operation/multiple-failure path is above.
func TestRetirementSuppressesResponseWithoutErasingCompletion(t *testing.T) {
	p := issuancePlan(t)
	keys, err := issuance.Open(t.Context(), p.KeyRoot, p.KeyBinding)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(p.KeyRoot, "issuer.pin")); err != nil {
		t.Fatal(err)
	}
	result := Result{Phase: "issue", Outcome: "issued-offline", Response: []byte{1, 2, 3}}
	result.closeOwner(1, "close-keys", keys.Close)
	if result.Phase != "issue" || result.Outcome != "issued-offline" || result.Response != nil {
		t.Fatalf("retirement lost completion or retained export: %+v", result)
	}
	for i := 0; i < 2; i++ {
		if phase, outcome := result.Status(); phase != "close-keys" || outcome != "storage-uncertain" {
			t.Fatalf("unstable projection: %s/%s", phase, outcome)
		}
	}
	if result.Cleanup[1] != (Completion{Phase: "close-keys", Outcome: "storage-unavailable"}) {
		t.Fatal(result.Cleanup)
	}
	if err := keys.Close(); err == nil {
		t.Fatal("repeated close forgot the failure")
	}
}

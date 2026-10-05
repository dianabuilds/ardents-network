package join

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// A local lifetime control cannot supply successful authority or physical JOIN.
// Exact pair identity/claims/lock ordering are tested by their Prefix owner.
type acquisitionLifetimeClaim struct {
	returns atomic.Int32
	local   func() error
}

func (b *acquisitionLifetimeClaim) CheckLocal() error {
	if b.local != nil {
		return b.local()
	}
	return nil
}
func (b *acquisitionLifetimeClaim) CheckOriginal() error {
	return errors.New("no authenticated original Prefix fixture")
}
func (b *acquisitionLifetimeClaim) WaitRetirement(ctx context.Context) { <-ctx.Done() }
func (b *acquisitionLifetimeClaim) CheckRecipient(network.RetainedDuty, time.Time) error {
	return errors.New("no authenticated recipient fixture")
}
func (b *acquisitionLifetimeClaim) OpenChannel(context.Context, context.Context, network.RetainedDuty, time.Time, time.Time) (*prefix.JoinChannel, uint8, error) {
	return nil, 0, errors.New("no successful channel fixture")
}
func (b *acquisitionLifetimeClaim) Publish(_ context.Context, _ context.Context, _ context.Context, handoff func(func() error) error) error {
	return handoff(b.CheckLocal)
}
func (b *acquisitionLifetimeClaim) ReturnJoined() { b.returns.Add(1) }
func lifetimeAcquisition(t *testing.T, ctx context.Context) (*JoinAcquisition, *acquisitionLifetimeClaim) {
	t.Helper()
	claim := new(acquisitionLifetimeClaim)
	a, err := startAcquisition(ctx, func(func(), func() error) (joinClaim, error) { return claim, nil })
	if err != nil {
		t.Fatal(err)
	}
	return a, claim
}

func TestJoinPublicAcquisitionRefusesMissingOriginalAndConsumesFailedAttempt(t *testing.T) {
	for _, acquire := range []func(context.Context, *prefix.Prefix) (*JoinAcquisition, error){AcquireSourceJoin, AcquireResponderJoin} {
		if got, err := acquire(t.Context(), nil); got != nil || err == nil {
			t.Fatal("missing original role produced an acquisition")
		}
	}
	a, claim := lifetimeAcquisition(t, t.Context())
	defer a.Close()
	end := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
	config := JoinConfig{Secret: [32]byte{1}, Context: [32]byte{2}, Deadline: end, SetupDeadline: end}
	if stream, err := a.Join(t.Context(), config); stream != nil || err == nil {
		t.Fatal("unavailable original authority produced a stream")
	}
	if stream, err := a.Join(t.Context(), config); stream != nil || err == nil || err.Error() != "route JOIN acquisition already used" {
		t.Fatal("failed original attempt could be reused", err)
	}
	if claim.returns.Load() != 0 {
		t.Fatal("failed attempt returned its original acquisition before Close")
	}
}

func TestJoinAcquisitionKeepsOriginalSourceAndJoinsOpening(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		acquisition, claim := lifetimeAcquisition(t, t.Context())
		acquisition.mu.Lock()
		acquisition.opening = make(chan struct{})
		opening := acquisition.opening
		acquisition.mu.Unlock()
		closed := make(chan error, 1)
		go func() { closed <- acquisition.Close() }()
		synctest.Wait()
		if acquisition.ctx.Err() == nil {
			t.Fatal("close did not interrupt original opening")
		}
		select {
		case <-closed:
			t.Fatal("returned acquisition before opening joined")
		default:
		}
		if claim.returns.Load() != 0 {
			t.Fatal("released original pair claim before join")
		}
		close(opening)
		if err := <-closed; err != nil {
			t.Fatal(err)
		}
		if claim.returns.Load() != 1 {
			t.Fatal("original claim retained after join")
		}
		if err := acquisition.current(); !errors.Is(err, context.Canceled) {
			t.Fatal("retired acquisition regained authority", err)
		}
		if err := acquisition.Close(); err != nil || claim.returns.Load() != 1 {
			t.Fatal("repeated close returned original claim twice", err)
		}
	})
}

func TestJoinPublicationChecksExactOriginalCaller(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		caller := &handoffDeferredCaller{Context: context.Background()}
		a, _ := lifetimeAcquisition(t, caller)
		defer a.Close()
		caller.canceled.Store(true)
		if err := a.publish(t.Context(), &Joined{}); err == nil {
			a.mu.Lock()
			a.stream = nil
			a.mu.Unlock()
			t.Fatal("original caller cancellation lost at publication")
		}
		a.mu.Lock()
		stream := a.stream
		a.mu.Unlock()
		if stream != nil {
			t.Fatal("canceled original caller published stream")
		}
	})
}

type handoffDeferredCaller struct {
	context.Context
	canceled atomic.Bool
}

func (c *handoffDeferredCaller) Err() error {
	if c.canceled.Load() {
		return context.Canceled
	}
	return nil
}

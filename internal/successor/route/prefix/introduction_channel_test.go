package prefix

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// The exact recipient reobservation can fail after the preceding leg check.
// Failure-only inputs cannot supply a member, ACK or accepting registration.
func TestRegistrationRecipientObservationRetainsOriginalFailure(t *testing.T) {
	physical := errors.New("original Network observation physical failure")
	for _, cause := range []error{context.Canceled, physical, errors.Join(context.Canceled, physical)} {
		p := &Prefix{}
		a := role.Authority{Current: func() (network.RuntimeView, error) { return network.RuntimeView{}, cause }}
		err := p.checkIntroductionDuty(a, time.Now().Add(time.Minute))
		if err != cause {
			t.Errorf("failed recipient observation acquired a different terminal cause: got %v, want %v", err, cause)
		}
	}
}

// These controls use physical parents only. They deliberately supply no
// successful Network observation, Stock presentation, receiving spend or ACK.
func TestRegistrationRetiredOriginalCallerRefusesBeforeEffects(t *testing.T) {
	p, _, returns, cleanup := terminalSetupPhysicalPrefix(t)
	defer cleanup()
	var observations, presentations atomic.Int32
	p.config.Current = func() (network.RuntimeView, error) {
		observations.Add(1)
		return network.RuntimeView{}, errors.New("unexpected observation")
	}
	p.config.Present = func(context.Context, ardp.Hello) ([]byte, error) {
		presentations.Add(1)
		return nil, errors.New("unexpected presentation")
	}
	caller, cancel := context.WithCancel(t.Context())
	cancel()
	r, err := p.OpenIntroductionChannel(caller, network.RetainedDuty{}, time.Now().Add(time.Minute).UTC().Truncate(time.Second))
	if r != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("retired caller returned a registration or lost original cancellation", r, err)
	}
	if observations.Load() != 0 || presentations.Load() != 0 || returns.Load() != 0 {
		t.Fatal("refusal crossed an effect or returned live parent capacity")
	}
	if err := p.Close(); err != nil || returns.Load() != 1 {
		t.Fatal("original parents did not join and return once", err, returns.Load())
	}
}

func TestSubmissionRetiredCallerRefusesBeforeObservationOrPresentation(t *testing.T) {
	p, _, returns, cleanup := terminalSetupPhysicalPrefix(t)
	defer cleanup()
	var observations, presentations atomic.Int32
	p.config.Current = func() (network.RuntimeView, error) {
		observations.Add(1)
		return network.RuntimeView{}, errors.New("unexpected observation")
	}
	p.config.Present = func(context.Context, ardp.Hello) ([]byte, error) {
		presentations.Add(1)
		return nil, errors.New("unexpected presentation")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	terminal, err := p.OpenSubmissionChannel(ctx, network.RetainedDuty{}, time.Now().Add(8*time.Second).UTC().Truncate(time.Second), nil)
	if terminal != nil || !errors.Is(err, context.Canceled) || observations.Load() != 0 || presentations.Load() != 0 || returns.Load() != 0 {
		t.Fatal("retired submission caller crossed an effect or lost refusal", terminal, err)
	}
}

func TestRegistrationStoppedSetupCallbackCannotBlockPhysicalRetirement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		done := make(chan struct{})
		defer close(done) // failed assertion still joins the controlled goroutine
		var callbacks, returns atomic.Int32
		cause := errors.New("registration setup physical cleanup failed")
		physical := newLifecycleConn(false)
		physical.closeFailure = cause
		terminal := &IntroductionChannel{ctx: ctx, cancel: cancel, conn: physical, interruption: prefixSetupInterruption{done: done},
			release: func() { returns.Add(1) }}
		terminal.interruption.stop = context.AfterFunc(ctx, func() { callbacks.Add(1) })
		if err := terminal.StopOpening(); err != nil {
			t.Fatal(err)
		}
		retired := make(chan error, 1)
		go func() { retired <- terminal.CloseSetup() }()
		synctest.Wait()
		select {
		case err := <-retired:
			if !errors.Is(err, cause) || returns.Load() != 1 || callbacks.Load() != 0 {
				t.Fatal("registration cleanup changed physical/callback ownership", err, returns.Load(), callbacks.Load())
			}
		default:
			t.Fatal("registration cleanup waited for an already stopped setup callback")
		}
	})
}

func TestRegistrationOriginalObservationFailureCompletesSetupBeforeParentJoin(t *testing.T) {
	p, readerDone, returns, cleanup := terminalSetupPhysicalPrefix(t)
	defer cleanup()
	original := errors.New("original Network observation unavailable")
	var presentations atomic.Int32
	p.config.Current = func() (network.RuntimeView, error) { return network.RuntimeView{}, original }
	p.config.Present = func(context.Context, ardp.Hello) ([]byte, error) {
		presentations.Add(1)
		return nil, errors.New("unexpected presentation")
	}
	r, err := p.OpenIntroductionChannel(t.Context(), network.RetainedDuty{}, time.Now().Add(time.Minute).UTC().Truncate(time.Second))
	if r != nil || !errors.Is(err, original) || presentations.Load() != 0 {
		t.Fatal("failed observation was replaced or crossed presentation", r, err)
	}
	select {
	case <-readerDone:
		t.Fatal("failed child setup retired original parent")
	default:
	}
	if returns.Load() != 0 {
		t.Fatal("failed child setup returned original parent capacity")
	}
	// Close is the observable completion oracle: a leaked pending setup would
	// hold this original parent's join even though no child was published.
	closed := make(chan error, 1)
	go func() { closed <- p.Close() }()
	select {
	case err := <-closed:
		if err != nil || returns.Load() != 1 {
			t.Fatal("failed setup did not complete before joined return", err, returns.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("failed observation leaked the original setup claim")
	}
}

// The original Prefix caller and the registration request are independent.
// Its real cancellation must refuse new setup before asynchronous propagation
// retires the physical parents. These pipes grant no successful authority.
func TestRegistrationRetiredPrefixCallerRefusesBeforeEffects(t *testing.T) {
	original, revoke := context.WithCancel(t.Context())
	defer revoke()
	p, _, returns, cleanup := terminalSetupPhysicalPrefix(t, &prefixDeferredPropagation{Context: original})
	defer cleanup()
	var observations, presentations atomic.Int32
	p.config.Current = func() (network.RuntimeView, error) {
		observations.Add(1)
		return network.RuntimeView{}, errors.New("unexpected original observation")
	}
	p.config.Present = func(context.Context, ardp.Hello) ([]byte, error) {
		presentations.Add(1)
		return nil, errors.New("unexpected presentation")
	}
	revoke()
	if p.ctx.Err() != nil || !errors.Is(p.caller.Err(), context.Canceled) {
		t.Fatal("fixture did not isolate delayed original Prefix cancellation")
	}
	r, err := p.OpenIntroductionChannel(t.Context(), network.RetainedDuty{}, time.Now().Add(time.Minute).UTC().Truncate(time.Second))
	if r != nil || !errors.Is(err, context.Canceled) || observations.Load() != 0 || presentations.Load() != 0 || returns.Load() != 0 {
		t.Fatalf("retired Prefix caller crossed setup: registration=%p error=%v observations=%d presentations=%d returns=%d", r, err, observations.Load(), presentations.Load(), returns.Load())
	}
	if err := p.Close(); err != nil || returns.Load() != 1 {
		t.Fatal("original physical parents failed joined return", err, returns.Load())
	}
}

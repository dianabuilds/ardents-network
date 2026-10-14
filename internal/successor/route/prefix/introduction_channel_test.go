package prefix

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
)

type introductionCleanupWrite struct {
	net.Conn
	emitted, release chan struct{}
	once             sync.Once
	failure          error
}

func (c *introductionCleanupWrite) Write(body []byte) (int, error) {
	n, err := c.Conn.Write(body[:min(8, len(body))])
	if err != nil {
		return n, err
	}
	c.once.Do(func() { close(c.emitted) })
	<-c.release
	return n, c.failure
}

// Actual encoded pipe output isolates physical retirement. These inputs
// provide no Network/Registration/Publication or successful TLS authority.
func TestIntroductionCleanupRequiresJoinedLowerLaneAndRetainsPhysicalFailure(t *testing.T) {
	for _, failOutput := range []bool{false, true} {
		t.Run(map[bool]string{false: "pre-output-refusal", true: "late-output"}[failOutput], func(t *testing.T) {
			local, peer := net.Pipe()
			defer peer.Close()
			var conn net.Conn = local
			failure := errors.New("original Introduction lower write failed late")
			check := func() error { return context.Canceled }
			var physical *introductionCleanupWrite
			if failOutput {
				physical = &introductionCleanupWrite{Conn: local, emitted: make(chan struct{}), release: make(chan struct{}), failure: failure}
				conn, check = physical, nil
			}
			owner, lane, err := framing.PrepareJoined(t.Context(), conn, time.Now().Add(5*time.Second), 1<<20, check, framing.NewBudget(4<<20))
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			var releaseOnce sync.Once
			unblock := func() {
				if physical != nil {
					releaseOnce.Do(func() { close(physical.release) })
				}
			}
			defer unblock()
			var returns atomic.Int32
			terminal := &IntroductionChannel{conn: lane, lane: lane, cancel: func() {}, release: func() { returns.Add(1) }}
			terminal.self = terminal
			if _, joined := terminal.CleanupResult(); joined {
				t.Fatal("unclosed Introduction supplied joined cleanup")
			}
			if failOutput {
				go func() { var header [8]byte; _, _ = peer.Read(header[:]) }()
				closed := make(chan error, 1)
				go func() { closed <- terminal.CloseSetup() }()
				select {
				case <-physical.emitted:
				case <-time.After(time.Second):
					t.Fatal("original CLOSE did not emit its physical prefix")
				}
				if _, joined := terminal.CleanupResult(); joined || returns.Load() != 0 {
					t.Fatal("Introduction cleanup preceded original writer join")
				}
				unblock()
				if err := <-closed; !errors.Is(err, failure) {
					t.Fatal("full Introduction outcome lost actual physical failure", err)
				}
			} else if err := terminal.CloseSetup(); !errors.Is(err, context.Canceled) {
				t.Fatal("original Introduction refusal was erased", err)
			}
			cleanup, joined := terminal.CleanupResult()
			if !joined || returns.Load() != 1 || failOutput && !errors.Is(cleanup, failure) || !failOutput && cleanup != nil {
				t.Fatal("original physical cleanup result changed", cleanup, joined, returns.Load())
			}
			_ = terminal.CloseSetup()
			if repeated, complete := terminal.CleanupResult(); !complete || repeated != cleanup || returns.Load() != 1 {
				t.Fatal("repeated retirement changed result or returned twice", repeated, returns.Load())
			}
			copied := &IntroductionChannel{self: terminal}
			if _, joined := copied.CleanupResult(); joined {
				t.Fatal("foreign terminal supplied original cleanup")
			}
		})
	}
}

// The shared fresh leg/recipient observation retains its original failure.
// Failure-only inputs cannot supply a member, ACK or accepting registration.
func TestRegistrationRecipientObservationRetainsOriginalFailure(t *testing.T) {
	physical := errors.New("original Network observation physical failure")
	for _, cause := range []error{context.Canceled, physical, errors.Join(context.Canceled, physical)} {
		calls := 0
		p := &Prefix{ctx: t.Context(), caller: t.Context(), config: Config{Current: func() (network.RuntimeView, error) {
			calls++
			return network.RuntimeView{}, cause
		}}}
		_, err := p.checkIntroductionDuty(network.RetainedDuty{}, time.Now().Add(time.Minute))
		if !errors.Is(err, cause) || calls != 1 {
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
	tooLate, boundErr := p.OpenSubmissionChannel(t.Context(), network.RetainedDuty{}, time.Now().Add(31*time.Second).UTC().Truncate(time.Second), nil)
	if tooLate != nil || boundErr == nil || observations.Load() != 0 || presentations.Load() != 0 {
		t.Fatal("submission beyond setup ceiling crossed an effect", tooLate, boundErr)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	terminal, err := p.OpenSubmissionChannel(ctx, network.RetainedDuty{}, time.Now().Add(30*time.Second).UTC().Truncate(time.Second), nil)
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

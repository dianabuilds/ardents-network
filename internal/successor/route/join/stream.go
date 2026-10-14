package join

import (
	"context"
	"errors"
	"fmt"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Joined is one protected physical stream, not an authenticated Service
// Connection. The original acquisition remains held until Close joins all
// framing and lower-channel borrowers.
type Joined struct {
	acquisition *JoinAcquisition
	session     *framing.Session
	lane        *framing.Lane
	parent      interface {
		CloseParent() error
		ParentCleanupResult() (error, bool)
	}
	once                  sync.Once
	result                error
	retiring, watcherDone chan struct{}
	stopOnce              sync.Once
	terminal              error
	cleanup               error
	end                   time.Time // Immutable actual admitted child lifetime.
	setupEnd              time.Time
	side                  uint8
	retirementDisposition atomic.Uint32
	secretDigest, intent  [32]byte // Local commitments, never a shared tracing identity.
}

const (
	retirementUnselected uint32 = iota
	retirementFailed
	retirementSelected
)

func (j *Joined) CheckResponderScope(scope *prefix.ResponderScope) error {
	if err := j.CheckOriginal(); err != nil {
		return err
	}
	borrow, ok := j.acquisition.borrows.(*prefix.ResponderJoinBorrow)
	if !ok {
		return errors.New("JOIN has no original scoped Responder")
	}
	return borrow.CheckScope(scope)
}

// CheckSourceCaller reobserves this exact physical acquisition under its
// original Source scope. One full observation covers their retained identical
// leg and authentic observer; provenance and local seals are checked on both
// sides, including the original caller and absolute bounds.
func (j *Joined) CheckSourceCaller(scope *prefix.SourceScope, caller context.Context, network, profile [32]byte, end time.Time) error {
	if j == nil || j.acquisition == nil {
		return errors.New("route JOIN original stream unavailable")
	}
	borrow, ok := j.acquisition.borrows.(*prefix.SourceJoinBorrow)
	if !ok {
		return errors.New("JOIN has no original scoped Source")
	}
	if err := borrow.MatchSourceCaller(scope, caller, network, profile, end); err != nil {
		return err
	}
	return errors.Join(j.CheckOriginal(), borrow.MatchSourceCaller(scope, caller, network, profile, end))
}

// CheckOriginal reobserves this exact acquisition; it does not authorize a
// Service or grant a new physical attempt. A copied/zero stream is unavailable.
func (j *Joined) CheckOriginal() error {
	if j == nil || j.acquisition == nil || j.session == nil || j.lane == nil {
		return errors.New("route JOIN original stream unavailable")
	}
	j.acquisition.mu.Lock()
	original := j.acquisition.stream == j
	j.acquisition.mu.Unlock()
	if !original {
		return errors.New("route JOIN original stream unavailable")
	}
	return j.acquisition.current()
}

// CommitOriginal reobserves the genuine parents before locking, then serializes
// a bounded local handoff with their Source/Responder seals and this exact
// acquisition. The consumer must do no observation, I/O or join in handoff.
func (j *Joined) CommitOriginal(ctx context.Context, handoff func(func() error) error) error {
	if ctx == nil || handoff == nil {
		return errors.New("route JOIN handoff unavailable")
	}
	if err := j.CheckOriginal(); err != nil {
		return err
	}
	a := j.acquisition
	return a.borrows.Publish(ctx, a.caller, a.ctx, func(checkParents func() error) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		check := func() error {
			if a.self != a || a.closed || a.stream != j || !time.Now().Before(j.end) {
				return errors.New("route JOIN original handoff retired")
			}
			return errors.Join(ctx.Err(), a.caller.Err(), a.ctx.Err(), checkParents())
		}
		if err := check(); err != nil {
			return err
		}
		return handoff(check)
	})
}

// CheckSourceBinding requires the exact initial intent admitted through the
// original Source. Copied capsule facts are comparison inputs, not authority.
func (j *Joined) CheckSourceBinding(secretDigest, intent [32]byte, end, setupEnd time.Time) error {
	return j.checkBinding(1, secretDigest, intent, end, setupEnd)
}

// CheckResponderBinding requires the exact original Source-bound Responder
// intent. Neither a live opposite-side JOIN nor an extended child suffices.
func (j *Joined) CheckResponderBinding(secretDigest, intent [32]byte, end, setupEnd time.Time) error {
	return j.checkBinding(2, secretDigest, intent, end, setupEnd)
}

func (j *Joined) checkBinding(side uint8, secretDigest, intent [32]byte, end, setupEnd time.Time) error {
	if j == nil {
		return errors.New("route JOIN original stream unavailable")
	}
	if j.side != side || secretDigest == [32]byte{} || intent == [32]byte{} || j.secretDigest != secretDigest || j.intent != intent || !j.end.Equal(end) || !j.setupEnd.Equal(setupEnd) {
		return errors.New("route JOIN original intent mismatch")
	}
	// These immutable commitments have no effects. Observe the original
	// acquisition once, after all comparisons and immediately before return.
	return j.CheckOriginal()
}

// CheckLifetime refuses a Service bound beyond this exact admitted child.
// It checks a bound; it neither extends transport nor authorizes a Service.
func (j *Joined) CheckLifetime(end time.Time) error {
	if err := j.CheckOriginal(); err != nil {
		return err
	}
	if end.IsZero() || j.end.IsZero() || end.After(j.end) {
		return errors.New("service lifetime exceeds original JOIN")
	}
	return nil
}

func (j *Joined) Read(body []byte) (int, error) {
	if err := j.acquisition.current(); err != nil {
		return 0, err
	}
	n, err := j.lane.Read(body)
	if current := j.acquisition.current(); current != nil {
		return 0, errors.Join(err, current)
	}
	return n, err
}

func (j *Joined) Write(body []byte) (int, error) {
	if err := j.acquisition.current(); err != nil {
		return 0, err
	}
	n, err := j.lane.Write(body)
	return n, errors.Join(err, j.acquisition.current())
}

func (j *Joined) CloseWrite() error {
	if err := j.acquisition.current(); err != nil {
		return err
	}
	return errors.Join(j.lane.CloseWrite(), j.acquisition.current())
}

func (j *Joined) Close() error { return j.acquisition.Close() }

// MarkFailedRetirement denies a clean terminal for this exact original child.
// It performs no I/O and grants no accepting state; Close still joins all of
// its original resources under the admitted bounds. A copied Joined cannot
// change the terminal disposition of the original acquisition.
func (j *Joined) MarkFailedRetirement() error {
	if j == nil || j.acquisition == nil || j.acquisition.self != j.acquisition {
		return errors.New("route JOIN original acquisition absent")
	}
	a := j.acquisition
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stream != j || a.closed {
		return errors.New("route JOIN failed retirement belongs to another child")
	}
	if j.retirementDisposition.CompareAndSwap(retirementUnselected, retirementFailed) || j.retirementDisposition.Load() == retirementFailed {
		return nil
	}
	return errors.New("route JOIN terminal disposition already selected")
}
func (j *Joined) LocalAddr() net.Addr                { return j.lane.LocalAddr() }
func (j *Joined) RemoteAddr() net.Addr               { return j.lane.RemoteAddr() }
func (j *Joined) SetDeadline(t time.Time) error      { return j.lane.SetDeadline(t) }
func (j *Joined) SetReadDeadline(t time.Time) error  { return j.lane.SetReadDeadline(t) }
func (j *Joined) SetWriteDeadline(t time.Time) error { return j.lane.SetWriteDeadline(t) }

// InterruptIO interrupts only this original child. Already stopped framing
// requires no new physical deadline operation; its retained failures still
// belong to the subsequent joined cleanup result.
func (j *Joined) InterruptIO() error {
	if j == nil || j.acquisition == nil || j.lane == nil {
		return errors.New("route JOIN original stream unavailable")
	}
	return j.lane.InterruptIO()
}

// CleanupResult observes this exact acquisition after all physical work and
// returns have joined. EOF/refusal are retained by Close as work outcomes;
// actual owned write/close failures remain in this separate cleanup result.
// Neither this receipt nor a nil cleanup result grants a new JOIN or Service.
func (j *Joined) CleanupResult() (error, bool) {
	if j == nil || j.acquisition == nil || j.acquisition.self != j.acquisition || j.acquisition.done == nil {
		return nil, false
	}
	select {
	case <-j.acquisition.done:
		if j.acquisition.stream != j {
			return nil, false
		}
		return j.acquisition.cleanup, true
	default:
		return nil, false
	}
}

// Synchronous effect seal contains no I/O or waits. The watcher then interrupts
// and joins the already selected writer; scheduling it is not revocation.
func (j *Joined) seal() {
	j.lane.Seal()
}

func (j *Joined) closePhysical() error {
	j.once.Do(func() {
		j.stopOnce.Do(func() { close(j.retiring) })
		<-j.watcherDone
		if j.terminal == nil {
			// Sending local CLOSE does not prove the pair's opposite writer
			// joined. Read its authenticated terminal before retiring the lower
			// lane; otherwise that retirement races the peer's pending CLOSE.
			j.terminal = j.session.WaitTerminal(time.Now().Add(time.Second))
		}
		terminal, session, parent := j.terminal, j.session.Close(), j.parent.CloseParent()
		parentCleanup, complete := j.parent.ParentCleanupResult()
		if !complete {
			parentCleanup = errors.Join(parentCleanup, errors.New("original JOIN parent cleanup did not join"))
		}
		j.cleanup = errors.Join(j.session.PhysicalFailure(), parentCleanup)
		if terminal != nil {
			terminal = fmt.Errorf("JOIN terminal retirement: %w", terminal)
		}
		if session != nil {
			session = fmt.Errorf("JOIN record-owner retirement: %w", session)
		}
		if parent != nil {
			parent = fmt.Errorf("JOIN parent retirement: %w", parent)
		}
		j.result = errors.Join(terminal, session, parent)
	})
	return j.result
}

func (j *Joined) watch(caller context.Context) {
	defer close(j.watcherDone)
	select {
	case <-j.retiring:
	case <-j.acquisition.ctx.Done():
	case <-caller.Done():
	}
	// Seal writes and interrupt/join any selected writer before terminal output.
	// Selection and a failed-retirement marker have one atomic ordering.
	// A later marker cannot claim to change a terminal already being written.
	if j.retirementDisposition.Swap(retirementSelected) == retirementFailed {
		j.terminal = j.lane.CloseRefused()
	} else {
		j.terminal = j.lane.Close()
	}
}

var _ net.Conn = (*Joined)(nil)

package stock

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/attempts"
	"sync"
	"time"
)

// Owner retains one context's holder and stock. It owns no transport, worker,
// goroutine or external mutex. Open owns the journal lease; New borrows it.
type Owner struct {
	mu         sync.Mutex
	observe    func() (admission.AuthorityFacts, time.Time, error)
	role       admission.AllocationRole
	journal    *attempts.Journal
	permission *permission
	active     *attemptState
	preparing  *Preparation
	closed     bool
	failure    error
	floor      time.Time
	ownJournal bool
	users      sync.WaitGroup // Admission and Close seal share mu.
	closing    chan struct{}
}

// Open owns the presentation journal lease as well as the volatile holder.
// The selected journal implementation requires an existing private Linux root.
func Open(root string, role admission.AllocationRole, observe func() (admission.AuthorityFacts, time.Time, error)) (*Owner, error) {
	if observe == nil || role != admission.AllocationUser && role != admission.AllocationPublisher {
		return nil, errors.New("holder observer absent")
	}
	p, now, err := observe()
	if err != nil || p.ValidateAt(now) != nil {
		return nil, errors.Join(errors.New("holder authority unavailable"), err)
	}
	j, err := attempts.Open(root, p.NetworkID, func() time.Time {
		current, at, err := observe()
		if err != nil || current.NetworkID != p.NetworkID {
			return time.Time{}
		}
		return at
	})
	if err != nil {
		return nil, err
	}
	o, err := New(role, observe, j)
	if err != nil {
		return nil, errors.Join(err, j.Close())
	}
	o.ownJournal = true
	o.floor = now
	return o, nil
}

func New(role admission.AllocationRole, observe func() (admission.AuthorityFacts, time.Time, error), journal *attempts.Journal) (*Owner, error) {
	if observe == nil || journal == nil || role != admission.AllocationUser && role != admission.AllocationPublisher {
		return nil, errors.New("invalid holder owner")
	}
	return &Owner{observe: observe, role: role, journal: journal}, nil
}

// beginUse retains only original operation/journal lifetime, never authority.
// External observations run outside mu; every effect rechecks local admission.
func (o *Owner) beginUse() error {
	if o == nil {
		return errors.New("holder unavailable")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.failure != nil || o.observe == nil {
		return errors.Join(errors.New("holder unavailable"), o.failure)
	}
	o.users.Add(1)
	return nil
}

// current compares freshly observed authority under mu. It performs no I/O.
func (o *Owner) current(p admission.AuthorityFacts, now time.Time, err error) (admission.AuthorityFacts, time.Time, error) {
	if o.closed || o.failure != nil {
		return admission.AuthorityFacts{}, time.Time{}, errors.Join(errors.New("holder unavailable"), o.failure)
	}
	if err != nil || now.IsZero() || now.Before(o.floor) || p.ValidateAt(now) != nil {
		return admission.AuthorityFacts{}, time.Time{}, errors.Join(errors.New("holder authority unavailable"), err)
	}
	o.floor = now
	return p, now, nil
}

func (o *Owner) Request(maxima [3]uint32) ([]byte, [32]byte, error) {
	if err := o.beginUse(); err != nil {
		return nil, [32]byte{}, err
	}
	defer o.users.Done()
	profile, at, observedErr := o.observe()
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.preparing != nil {
		return nil, [32]byte{}, errors.New("original stock opening is in progress")
	}
	p, now, err := o.current(profile, at, observedErr)
	if err != nil {
		return nil, [32]byte{}, err
	}
	scope, err := newRequestScope(p, now, o.role, maxima)
	if err != nil {
		return nil, [32]byte{}, err
	}
	if previous := o.permission; previous != nil {
		public, digest, active, err := previous.retainedRequest(scope)
		if active {
			return public, digest, err
		}
		o.clear()
	}
	prepared, public, digest, err := preparePermission(scope)
	if err != nil {
		return nil, [32]byte{}, err
	}
	o.permission = prepared
	return public, digest, nil
}

func (o *Owner) Import(digest [32]byte, raw []byte) error {
	if err := o.beginUse(); err != nil {
		return err
	}
	defer o.users.Done()
	grant, err := admission.DecodePermission(raw)
	if err != nil {
		return err
	}
	profile, at, observedErr := o.observe()
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.preparing != nil {
		return errors.New("original stock opening is in progress")
	}
	p, now, err := o.current(profile, at, observedErr)
	if err != nil {
		return err
	}
	return o.permission.acceptResponse(p, now, digest, grant)
}

func (o *Owner) clear() {
	o.active = nil
	if o.permission != nil {
		clearPermission(o.permission)
		o.permission = nil
	}
}

// Close immediately revokes secrets. Outstanding public attempts cannot revive
// them. The application separately cancels/joins its own exchange.
func (o *Owner) Close() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	if o.closing != nil {
		done := o.closing
		o.mu.Unlock()
		<-done
		o.mu.Lock()
		defer o.mu.Unlock()
		return o.failure
	}
	o.closed = true
	o.closing = make(chan struct{})
	o.clear()
	o.mu.Unlock()
	o.users.Wait()
	var closeErr error
	if o.ownJournal {
		closeErr = o.journal.Close()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failure = errors.Join(o.failure, closeErr)
	close(o.closing)
	return o.failure
}

// Take irreversibly marks presentation before returning a token. Binding and
// currentness are checked again after journal I/O; refusal never restores stock.
func (o *Owner) Take(ctx context.Context, presentation Presentation, class uint8) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("holder unavailable")
	}
	if err := o.beginUse(); err != nil {
		return nil, err
	}
	defer o.users.Done()
	profile, observedAt, observedErr := o.observe()
	o.mu.Lock()
	p, now, err := o.current(profile, observedAt, observedErr)
	if err != nil {
		o.mu.Unlock()
		return nil, err
	}
	if ctx.Err() != nil || presentation.NetworkID != p.NetworkID || presentation.StateGeneration != p.StateGeneration || presentation.StateDigest != p.StateDigest ||
		presentation.ProfileDigest != p.Digest || presentation.RecipientNodeID == [32]byte{} || presentation.RecipientDutyGeneration == 0 || presentation.ChannelNonce == [32]byte{} ||
		!now.Before(presentation.Deadline) || presentation.Deadline.After(p.NotAfter) {
		o.mu.Unlock()
		return nil, errors.New("presentation binding unavailable")
	}
	raw, err := o.permission.consumeToken(p, now, presentation, class)
	if err != nil {
		o.mu.Unlock()
		return nil, err
	}
	original := o.permission
	record := attempts.Attempt{Profile: p.Digest, Receiver: presentation.RecipientNodeID, Duty: presentation.RecipientDutyGeneration, Window: original.accepted.NotBefore, Class: class, Nonce: presentation.ChannelNonce}
	permissionEnd := original.accepted.NotAfter
	o.mu.Unlock()
	err = o.journal.Mark(raw, record)
	if err != nil {
		clear(raw)
		o.mu.Lock()
		o.failure = errors.Join(o.failure, err)
		o.clear()
		o.mu.Unlock()
		return nil, err
	}
	profile, observedAt, observedErr = o.observe()
	o.mu.Lock()
	defer o.mu.Unlock()
	after, at, err := o.current(profile, observedAt, observedErr)
	if err != nil || o.permission != original || after != p || !at.Before(presentation.Deadline) || !at.Before(permissionEnd) || ctx.Err() != nil {
		clear(raw)
		return nil, errors.Join(errors.New("presentation authority changed"), err, ctx.Err())
	}
	return raw, nil
}

// Status is a defensive observation, never a grant or a mutable owner handle.
type Status struct {
	Accepted, Pending, Busy, Closed bool
	Remaining                       [3]uint32
	BootstrapRemaining              uint8
}

func (o *Owner) Status() Status {
	if o == nil {
		return Status{Closed: true}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	s := Status{Closed: o.closed, Busy: o.active != nil}
	if p := o.permission; p != nil {
		s.Accepted = p.HasAccepted()
		s.Pending = p.HasPending()
		s.BootstrapRemaining = p.BootstrapAllowance()
		for i := range s.Remaining {
			s.Remaining[i] = p.Remaining(uint8(i + 1))
		}
	}
	return s
}

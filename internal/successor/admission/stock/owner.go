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
	closed     bool
	failure    error
	floor      time.Time
	ownJournal bool
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

func (o *Owner) current() (admission.AuthorityFacts, time.Time, error) {
	if o.closed || o.failure != nil {
		return admission.AuthorityFacts{}, time.Time{}, errors.Join(errors.New("holder unavailable"), o.failure)
	}
	p, now, err := o.observe()
	if err != nil || now.IsZero() || now.Before(o.floor) || p.ValidateAt(now) != nil {
		return admission.AuthorityFacts{}, time.Time{}, errors.Join(errors.New("holder authority unavailable"), err)
	}
	o.floor = now
	return p, now, nil
}

func (o *Owner) Request(maxima [3]uint32) ([]byte, [32]byte, error) {
	if o == nil {
		return nil, [32]byte{}, errors.New("holder unavailable")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	p, now, err := o.current()
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
	if o == nil {
		return errors.New("holder unavailable")
	}
	grant, err := admission.DecodePermission(raw)
	if err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	p, now, err := o.current()
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
	defer o.mu.Unlock()
	if o.closed {
		return o.failure
	}
	o.closed = true
	o.clear()
	if o.ownJournal {
		o.failure = errors.Join(o.failure, o.journal.Close())
	}
	return o.failure
}

// Take irreversibly marks presentation before returning a token. Binding and
// currentness are checked again after journal I/O; refusal never restores stock.
func (o *Owner) Take(ctx context.Context, presentation Presentation, class uint8) ([]byte, error) {
	if o == nil || ctx == nil {
		return nil, errors.New("holder unavailable")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	p, now, err := o.current()
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil || presentation.NetworkID != p.NetworkID || presentation.StateGeneration != p.StateGeneration || presentation.StateDigest != p.StateDigest ||
		presentation.ProfileDigest != p.Digest || presentation.RecipientNodeID == [32]byte{} || presentation.RecipientDutyGeneration == 0 || presentation.ChannelNonce == [32]byte{} ||
		!now.Before(presentation.Deadline) || presentation.Deadline.After(p.NotAfter) {
		return nil, errors.New("presentation binding unavailable")
	}
	raw, err := o.permission.consumeToken(p, now, presentation, class)
	if err != nil {
		return nil, err
	}
	err = o.journal.Mark(raw, attempts.Attempt{Profile: p.Digest, Receiver: presentation.RecipientNodeID, Duty: presentation.RecipientDutyGeneration, Window: o.permission.accepted.NotBefore, Class: class, Nonce: presentation.ChannelNonce})
	if err != nil {
		clear(raw)
		o.failure = err
		o.clear()
		return nil, err
	}
	after, at, err := o.current()
	if err != nil || after != p || !at.Before(presentation.Deadline) || !at.Before(o.permission.accepted.NotAfter) || ctx.Err() != nil {
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

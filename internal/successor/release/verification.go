package release

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrClosed           = errors.New("release: verifier closed")
	ErrIncompatible     = errors.New("release: incompatible local binding")
	ErrTrustUnavailable = errors.New("release: established complete trust history unavailable")
)

const h3EvidenceNotice = "threshold and rebuild identities are project-controlled; no independent custody or builder claim"

// Verifier owns one durable history. A single admitted operation holds gate
// until all of its physical work completes; Close cancels it and joins gate
// before returning the lease. Waiting callers cannot extend original contexts.
type Verifier struct {
	mu       sync.Mutex
	gate     chan struct{}
	stop     chan struct{}
	done     chan struct{}
	closed   bool
	active   context.CancelFunc
	terminal error
	store    *floorStore
}

// Open acquires a native exclusive lease and confirms the validated history's
// durability. Missing retained pointers and unsupported platforms refuse.
func Open(path string) (*Verifier, error) {
	s, err := openFloorStore(path)
	if err != nil {
		return nil, err
	}
	return verifierForStore(s), nil
}

// OpenRetained requires an existing owned history with every metadata floor.
// It never creates a history root or bootstraps trust from candidate bytes.
func OpenRetained(path string) (*Verifier, error) {
	s, err := openFloorStoreMode(path, true)
	if err != nil {
		return nil, err
	}
	return verifierForStore(s), nil
}

func verifierForStore(s *floorStore) *Verifier {
	v := &Verifier{gate: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}), store: s}
	v.gate <- struct{}{}
	return v
}

func (v *Verifier) enter(ctx context.Context) (context.Context, func(), error) {
	if v == nil || v.gate == nil || ctx == nil {
		return nil, nil, ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-v.stop:
		return nil, nil, ErrClosed
	case <-v.gate:
	}
	v.mu.Lock()
	if v.closed || ctx.Err() != nil {
		v.mu.Unlock()
		v.gate <- struct{}{}
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		return nil, nil, ErrClosed
	}
	work, cancel := context.WithCancel(ctx)
	v.active = cancel
	v.mu.Unlock()
	return work, func() { cancel(); v.mu.Lock(); v.active = nil; v.mu.Unlock(); v.gate <- struct{}{} }, nil
}

// Close stops admission, cancels and joins the admitted operation, then releases
// the native lease. Repeated calls retain the same physical terminal result.
func (v *Verifier) Close() error {
	if v == nil || v.gate == nil {
		return ErrClosed
	}
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		<-v.done
		return v.terminal
	}
	v.closed = true
	close(v.stop)
	if v.active != nil {
		v.active()
	}
	v.mu.Unlock()
	<-v.gate
	v.terminal = v.store.Close()
	close(v.done)
	return v.terminal
}

// CurrentFloors is a detached observation and grants no target authority.
func (v *Verifier) CurrentFloors(ctx context.Context) (FloorSet, error) {
	work, leave, err := v.enter(ctx)
	if err != nil {
		return FloorSet{}, err
	}
	defer leave()
	f, err := v.store.ReadFloors()
	if err != nil {
		return FloorSet{}, err
	}
	if err = errors.Join(ctx.Err(), work.Err()); err != nil {
		return FloorSet{}, err
	}
	return cloneDecision(Decision{Floors: f}).Floors, nil
}

// Evaluate verifies a private snapshot of input and authorizes exact bytes.
// The caller must not mutate input during its initial bounded copy. A caller's
// cancellation after commit retains floors but refuses the final handoff.
func (v *Verifier) Evaluate(ctx context.Context, in Inputs) Decision {
	work, leave, err := v.enter(ctx)
	if err != nil {
		return reject(outcomeReleaseUnavailable, "evaluation unavailable", err)
	}
	defer leave()
	if in.Local.RefTime.IsZero() {
		return reject(outcomeReleaseInvalid, "reference time missing", nil)
	}
	if err = validateInputsEnvelope(in); err != nil {
		return reject(outcomeReleaseInvalid, "invalid envelope", err)
	}
	in = freezeInputs(in)
	decision := evaluate(work, in, &operationStore{floorStore: v.store, ctx: work, original: ctx})
	// Linearize handoff against Close under the same admission lock. Cancellation
	// after a durable effect retains floors but cannot expose an authorization.
	v.mu.Lock()
	if v.closed || ctx.Err() != nil || work.Err() != nil {
		err = errors.Join(ctx.Err(), work.Err(), func() error {
			if v.closed {
				return ErrClosed
			}
			return nil
		}())
		decision = reject(outcomeReleaseUnavailable, "evaluation retired before handoff", err)
	}
	v.mu.Unlock()
	return decision
}

func freezeInputs(in Inputs) Inputs {
	in.RootBytes = append([]byte(nil), in.RootBytes...)
	in.Artifact = append([]byte(nil), in.Artifact...)
	files := make(map[string][]byte, len(in.Files))
	for name, data := range in.Files {
		files[name] = append([]byte(nil), data...)
	}
	in.Files = files
	return in
}

type floorPersistence interface {
	ReadFloors() (FloorSet, error)
	CommitRoot(int64, []byte, [][]byte) error
	CommitFloors(FloorSet, [][]byte) error
}

type operationStore struct {
	*floorStore
	ctx      context.Context
	original context.Context
}

func (s *operationStore) CommitRoot(v int64, d []byte, chain [][]byte) error {
	if err := errors.Join(s.original.Err(), s.ctx.Err()); err != nil {
		return err
	}
	return s.floorStore.CommitRoot(v, d, chain)
}
func (s *operationStore) CommitFloors(f FloorSet, chain [][]byte) error {
	if err := errors.Join(s.original.Err(), s.ctx.Err()); err != nil {
		return err
	}
	return s.floorStore.CommitFloors(f, chain)
}

func reject(outcome Outcome, notice string, cause error) Decision {
	return Decision{Outcome: outcome, Notice: notice, EvidenceNotice: h3EvidenceNotice, cause: cause}
}

// Err retains the causal context or physical failure without exposing it in
// the bounded consumer report.
func (d Decision) Err() error           { return d.cause }
func detailInvalidMessage(error) string { return "release metadata refused" }

package systemd

import (
	"context"
	"os"
)

// SourceReplacement owns a distinct original root connection for replacement
// of the fixed Source unit. It retains that unit, observes complete native
// properties and permits one explicit reload. It cannot Start, Stop, Enable,
// choose a unit or reconnect. Installation alone admits each native effect.
type SourceReplacement struct{ original *sourceStopped }

func OpenSourceReplacement(ctx context.Context) (*SourceReplacement, error) {
	return openSourceReplacement(ctx, "")
}

// OpenSourceRecoveryReplacement matches the actual original broker/manager
// before any LoadUnit/RefUnit effect. The copied expectation grants no policy
// or recovery authority; Installation separately owns the genuine cut proof.
func OpenSourceRecoveryReplacement(ctx context.Context, expected string) (*SourceReplacement, error) {
	if expected == "" || len(expected) > 288 {
		return nil, ErrInput
	}
	return openSourceReplacement(ctx, expected)
}

func openSourceReplacement(ctx context.Context, expected string) (*SourceReplacement, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openSourceReference(ctx)
	if r == nil {
		return nil, err
	}
	s := &SourceReplacement{original: &sourceStopped{reference: &SourcePreflight{owner: r}}}
	if err != nil {
		return s, err
	}
	if expected != "" {
		actual, err := r.sourceManagerIdentity()
		if err != nil {
			return s, err
		}
		if actual != expected {
			return s, r.sourceResult(ErrObservation, ctx)
		}
	}
	if err := r.initializeSourceDeclaration(); err != nil {
		return s, err
	}
	if err := r.retainSourceControlUnit(); err != nil {
		return s, err
	}
	if _, err := r.sourceLoadedObservation(); err != nil {
		return s, err
	}
	return s, nil
}

func (s *SourceReplacement) Observe() (SourceObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil || s.original.reference.owner == nil {
		return SourceObservation{}, ErrInput
	}
	return s.original.reference.owner.sourceLoadedObservation()
}

func (s *SourceReplacement) Reload() error {
	if s == nil || s.original == nil {
		return ErrInput
	}
	// This private physical reload shares its original one-attempt latch,
	// cancellation and joined Close, not the initial consumer's absence policy.
	return (&SourceStopped{original: s.original}).Reload()
}

func (s *SourceReplacement) Close() error {
	if s == nil {
		return nil
	}
	return (&SourceStopped{original: s.original}).Close()
}

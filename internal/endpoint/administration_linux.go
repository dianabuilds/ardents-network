//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync"
	"unicode/utf8"

	applicationadministration "github.com/dianabuilds/ardents-network/internal/application/administration"
	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// administration owns one explicit snapshot publication and its terminal
// withdrawal. The participant supplies the separately authorized context;
// neither snapshot bytes nor the local socket supply authority or a Target.
type administration struct {
	context   *dutyContext
	mu        sync.Mutex
	closed    bool
	ending    bool
	pending   chan struct{}
	cancel    context.CancelFunc
	run       *publisherRun
	closeOnce sync.Once
	closeErr  error
}

func (owner *dutyContext) openAdministration() (*administration, error) {
	if owner == nil {
		return nil, errors.New("text Administration unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.liveLocked(owner.endpoint, broker.Administration) || owner.principal == [32]byte{} {
		return nil, errors.New("text Administration requires its own current authorization")
	}
	return &administration{context: owner}, nil
}

func (owner *administration) authorize(ctx context.Context) error {
	if owner == nil || owner.context == nil || ctx == nil || ctx.Err() != nil {
		return errors.New("text Administration unavailable")
	}
	endpoint := owner.context.endpoint
	capability, err := endpoint.Admit(owner.context.principal, broker.Administration)
	if err != nil {
		return err
	}
	_, err = endpoint.admission.Consume(capability, owner.context.principal, broker.Administration)
	return err
}

// Publish cannot invent a snapshot for the preceding bodyless operation.
func (owner *administration) Publish(context.Context) error {
	return errors.New("text publication requires an explicit snapshot")
}

// PublishSnapshot returns only after installed worker qualification and the
// actual Descriptor acknowledgement. Its completed caller does not own the
// retained publication lifetime. The worker receives its snapshot during INIT.
func (owner *administration) PublishSnapshot(ctx context.Context, snapshot []byte) (outcome error) {
	if err := owner.authorize(ctx); err != nil {
		return err
	}
	if len(snapshot) > applicationadministration.MaximumSnapshotBytes || !utf8.Valid(snapshot) {
		return errors.New("text publication snapshot invalid")
	}
	owner.mu.Lock()
	if owner.closed || owner.ending || owner.pending != nil || owner.run != nil {
		owner.mu.Unlock()
		return errors.New("text publication already owned or unavailable")
	}
	startup, cancel := context.WithCancel(ctx)
	pending := make(chan struct{})
	owner.pending, owner.cancel = pending, cancel
	owner.mu.Unlock()
	defer func() {
		cancel()
		owner.mu.Lock()
		owner.pending, owner.cancel = nil, nil
		close(pending)
		owner.mu.Unlock()
	}()
	run, err := owner.context.startPublisher(startup, snapshot)
	if err != nil {
		return err
	}
	owner.mu.Lock()
	accepted := !owner.closed && !owner.ending && startup.Err() == nil
	if accepted {
		owner.run = run
	}
	owner.mu.Unlock()
	if !accepted {
		return errors.Join(errors.New("text publication startup ended before handover"), run.Close())
	}
	return nil
}

// Withdraw remains reachable while startup runs. In that case it cancels and
// joins startup, preventing a late published result. A committed run uses the
// existing five-second drain; repeated calls cannot start another drain.
func (owner *administration) Withdraw(ctx context.Context) error {
	if err := owner.authorize(ctx); err != nil {
		owner.context.reportWithdrawalFailure("authorization")
		return err
	}
	owner.mu.Lock()
	if owner.closed || owner.ending || owner.pending == nil && owner.run == nil {
		owner.mu.Unlock()
		owner.context.reportWithdrawalFailure("publication-state")
		return errors.New("text publication already ending")
	}
	owner.ending = true
	pending, cancel, run := owner.pending, owner.cancel, owner.run
	owner.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if pending != nil {
		<-pending
	}
	if run == nil {
		owner.context.reportWithdrawalFailure("publication-handover")
		return errors.New("text publication was not committed")
	}
	return run.Withdraw(ctx)
}

// Close revokes the context and joins startup and the retained run. It is an
// abort used by participant shutdown, not a successful withdrawal receipt.
func (owner *administration) Close() error {
	if owner == nil {
		return nil
	}
	owner.closeOnce.Do(func() {
		owner.mu.Lock()
		owner.closed = true
		pending, cancel, run := owner.pending, owner.cancel, owner.run
		owner.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		owner.closeErr = owner.context.Close()
		if pending != nil {
			<-pending
		}
		if run != nil {
			owner.closeErr = errors.Join(owner.closeErr, run.Close())
		}
	})
	return owner.closeErr
}

var _ applicationadministration.Interface = (*administration)(nil)
var _ applicationadministration.SnapshotPublisher = (*administration)(nil)

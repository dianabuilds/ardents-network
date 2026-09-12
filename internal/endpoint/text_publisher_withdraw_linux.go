//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"
)

// Withdraw stops admission before network I/O. Previously admitted streams keep
// their original bounds, with an additional five-second maximum. Only the first
// withdrawal owns this transition; repeated calls cannot extend its deadline.
func (run *textPublisherRun) Withdraw(ctx context.Context) error {
	if run == nil || ctx == nil || ctx.Err() != nil {
		return errors.New("text publication withdrawal unavailable")
	}
	run.mu.Lock()
	if run.ending || run.withdrawDone != nil {
		run.mu.Unlock()
		return errors.New("text publication already ending")
	}
	owner := run.owner
	owner.mu.Lock()
	if owner.publicationDraining || owner.registration == nil || !owner.liveLocked(owner.endpoint, owner.surface) {
		owner.mu.Unlock()
		run.mu.Unlock()
		return errors.New("text publication withdrawal owner unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	abortDone := make(chan struct{})
	stopAbort := context.AfterFunc(bounded, func() { defer close(abortDone); run.cancel() })
	run.withdrawDone = make(chan struct{})
	withdrawn := run.withdrawDone
	owner.publicationDraining = true
	close(owner.publicationDrain)
	owner.signalTextRegistrationsLocked()
	owner.mu.Unlock()
	run.mu.Unlock()
	// Join an in-flight refresh before withdrawing its final selected registration.
	// The admission stop is already visible throughout this network operation.
	owner.stopTextRefresh()
	withdrawalErr := owner.withdrawTextIntroduction(bounded)
	if withdrawalErr != nil {
		run.cancel()
	}
	close(withdrawn)
	<-run.done
	if !stopAbort() {
		<-abortDone
	}
	boundedErr := bounded.Err()
	cancel()
	return errors.Join(withdrawalErr, run.err, boundedErr)
}

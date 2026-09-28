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
func (run *publisherRun) Withdraw(ctx context.Context) error {
	if run == nil {
		return errors.New("text publication withdrawal unavailable")
	}
	if ctx == nil || ctx.Err() != nil {
		run.owner.reportWithdrawalFailure("caller-context")
		return errors.New("text publication withdrawal unavailable")
	}
	run.mu.Lock()
	if run.ending || run.withdrawDone != nil {
		run.mu.Unlock()
		run.owner.reportWithdrawalFailure("publisher-ended")
		return errors.New("text publication already ending")
	}
	owner := run.owner
	owner.mu.Lock()
	failure := ""
	if owner.publication.pair.drainingLocked() {
		failure = "publication-draining"
	} else if owner.publication.pair.currentLocked() == nil {
		failure = "registration-absent"
	} else if !owner.liveLocked(owner.endpoint, owner.surface) {
		failure = "publisher-not-live"
	}
	if failure != "" {
		owner.mu.Unlock()
		run.mu.Unlock()
		owner.reportWithdrawalFailure(failure)
		return errors.New("text publication withdrawal owner unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	abortDone := make(chan struct{})
	stopAbort := context.AfterFunc(bounded, func() { defer close(abortDone); run.cancel() })
	run.withdrawDone = make(chan struct{})
	withdrawn := run.withdrawDone
	owner.publication.beginDrainLocked()
	owner.mu.Unlock()
	run.mu.Unlock()
	// Join an in-flight refresh before withdrawing its final selected registration.
	// The admission stop is already visible throughout this network operation.
	owner.publication.stopRefresh()
	withdrawalErr := owner.withdrawIntroduction(bounded)
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
	if withdrawalErr != nil {
		owner.reportWithdrawalFailure("registration")
	}
	if run.err != nil {
		owner.reportWithdrawalFailure("publisher-drain")
	}
	if boundedErr != nil {
		owner.reportWithdrawalFailure("deadline")
	}
	return errors.Join(withdrawalErr, run.err, boundedErr)
}

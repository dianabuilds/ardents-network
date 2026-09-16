//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func (worker *qualifiedTextWorker) runQualifiedStreams(ctx context.Context, streams []streamqualification.BoundStream) (report streamqualification.Report, outcome error) {
	bounded, cancel := context.WithCancel(ctx)
	stopped := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-bounded.Done():
				stopped <- nil
				return
			case <-ticker.C:
				if err := worker.replenishStreams(bounded); err != nil {
					stopped <- err
					cancel()
					return
				}
			}
		}
	}()
	attachment := &qualificationAttachment{ReadWriteCloser: worker.lifetime.attachment, sample: worker.job.qualificationStopSampling}
	report, outcome = streamqualification.RunConnections(bounded, attachment, *worker.job.qualification, streams, worker.job.qualificationObserve)
	cancel()
	outcome = errors.Join(outcome, <-stopped)
	return report, outcome
}

func (worker *qualifiedTextWorker) replenishStreams(ctx context.Context) error {
	owner := worker.job.owner
	owner.mu.Lock()
	if !owner.liveTextServiceJobLocked(worker.job, owner.surface) {
		owner.mu.Unlock()
		return errors.New("qualification refill job retired")
	}
	// Busy issuance delays inspection; it cannot spend another token or refill.
	if owner.issuance != nil {
		owner.mu.Unlock()
		return nil
	}
	prefixes := []*route.ClosedSourcePrefix{owner.prefix, owner.introduction.prefix, owner.responder.prefix}
	joins := make([]*route.ClosedJoinedStream, 0, len(worker.job.qualificationJoins))
	for joined := range worker.job.qualificationJoins {
		joins = append(joins, joined)
	}
	owner.mu.Unlock()
	present := func(hello route.ClosedHello, class uint8) ([]byte, error) {
		return owner.presentQualifiedRefill(ctx, worker.job, hello, class)
	}
	for _, prefix := range prefixes {
		if prefix != nil {
			if err := prefix.Replenish(ctx, present); err != nil {
				return err
			}
		}
	}
	for _, joined := range joins {
		if err := joined.Replenish(ctx, present); err != nil {
			owner.mu.Lock()
			_, retained := worker.job.qualificationJoins[joined]
			owner.mu.Unlock()
			// A retiring transport's Service owner retains its terminal cause.
			if retained {
				return err
			}
		}
	}
	return nil
}

func (owner *textContext) presentQualifiedRefill(ctx context.Context, job *textJobIdentity, hello route.ClosedHello, class uint8) ([]byte, error) {
	release, err := owner.acquireTextSourceOperation(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	owner.mu.Lock()
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil || class != 2 || !owner.liveTextServiceJobLocked(job, owner.surface) || owner.permission == nil ||
		hello.NetworkID != profile.NetworkID || hello.StateDigest != profile.StateDigest || hello.StateGeneration != profile.StateGeneration ||
		hello.ProfileDigest != profile.Digest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		owner.mu.Unlock()
		return nil, errors.New("qualification refill authority unavailable")
	}
	stocked := false
	for _, stock := range owner.permission.stock {
		if stock.challenge.ReceiverNodeID == hello.RecipientNodeID && stock.challenge.ReceiverDutyGeneration == hello.RecipientDutyGeneration &&
			stock.challenge.ProfileDigest == profile.Digest && stock.challenge.Class == 2 && stock.challenge.WindowStart == owner.permission.accepted.NotBefore && len(stock.tokens) != 0 {
			stocked = true
		}
	}
	owner.mu.Unlock()
	if !stocked {
		if err := owner.issueTextTokensForOpening(ctx, [][32]byte{hello.RecipientNodeID}, 2, nil, false); err != nil {
			return nil, err
		}
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	current, now, err := owner.textPermissionProfileLocked()
	if err != nil || current != profile || !owner.liveTextServiceJobLocked(job, owner.surface) {
		return nil, errors.New("qualification refill authority changed")
	}
	return owner.takeTextTokenLocked(current, now, hello, class, ctx)
}

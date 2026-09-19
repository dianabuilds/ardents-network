//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// Keep issuer admissions outside the next Introduction's ten-second wire
// lifetime. Refilling at this completed-stream boundary retains the exact
// permission and durable-spend rules without putting a large refill in the
// capsule's latency-critical path. Reader reserves are deliberately separated
// by nine tokens, so their three-token-per-opening paths do not recreate the
// refill herd that their opening phases avoid. Each refill is capped to eight
// tokens so one CPU can continue sampling every Node during issuance.
const qualificationIssuerReserve, qualificationIssuerRefill = 8, 8

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
	joins := make([]*route.ClosedJoinedStream, 0, len(worker.job.qualificationJoins))
	for joined := range worker.job.qualificationJoins {
		joins = append(joins, joined)
	}
	owner.mu.Unlock()
	issuerReserve := qualificationIssuerReserve
	if worker.job.qualification.Role == streamqualification.ReaderRole {
		issuerReserve += (3 - worker.qualificationReader) * 9
	}
	if err := owner.ensureQualificationIssuerReserve(ctx, issuerReserve); err != nil {
		return err
	}
	// Observe the prefixes after reserve inspection: a Source prefix retired
	// on its finite post-work interval may have just been reopened by that
	// inspection. Snapshotting before it would replenish a known-dead parent.
	owner.mu.Lock()
	prefixes := []*route.ClosedSourcePrefix{owner.prefix, owner.introduction.prefix, owner.responder.prefix}
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

func (owner *textContext) ensureQualificationIssuerReserve(ctx context.Context, minimum int) error {
	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	if err != nil || owner.permission == nil || ctx.Err() != nil {
		owner.mu.Unlock()
		return errors.Join(err, ctx.Err(), errors.New("qualification issuer reserve unavailable"))
	}
	ready := 0
	for _, stock := range owner.permission.stock {
		if stock.challenge.ReceiverNodeID == profile.IssuerNodeID &&
			stock.challenge.ReceiverDutyGeneration == profile.IssuerDutyGeneration &&
			stock.challenge.ProfileDigest == profile.Digest && stock.challenge.Class == 1 &&
			stock.challenge.WindowStart == owner.permission.accepted.NotBefore {
			ready += len(stock.tokens)
		}
	}
	remaining := owner.permission.accepted.Maxima[0] - owner.permission.reserved[0]
	prefixLive := owner.prefix != nil
	owner.mu.Unlock()
	if ready >= minimum || remaining == 0 {
		// No new admission is due here, so a Source prefix retired on its
		// finite post-work interval must not fail the retained connection set.
		return nil
	}
	if !prefixLive {
		// Fresh issuer stock is new private work: reopen the Source prefix
		// through the same retained-member bootstrap that opened it at job
		// start. A concurrent opening or issuance flight already owns the
		// refill; the next completed-stream boundary observes its result.
		if _, openErr := owner.openTextPrefix(ctx); openErr != nil {
			owner.mu.Lock()
			inProgress := owner.prefix != nil || owner.prefixOpening != nil || owner.issuance != nil
			owner.mu.Unlock()
			if !inProgress {
				return errors.Join(openErr, errors.New("qualification issuer prefix rebirth failed"))
			}
			return nil
		}
	}
	count := min(remaining, uint32(qualificationIssuerRefill))
	receivers := make([][32]byte, count)
	for index := range receivers {
		receivers[index] = profile.IssuerNodeID
	}
	return owner.issueTextTokens(ctx, receivers, 1)
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

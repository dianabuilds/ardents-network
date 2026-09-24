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
	attachment := &qualificationAttachment{ReadWriteCloser: worker.lifetime.attachment, sample: worker.job.qualification.stopSamples}
	report, outcome = streamqualification.RunConnections(bounded, attachment, worker.job.qualification.init, streams, worker.job.qualification.observe)
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
	joins := worker.job.qualification.joinedStreams()
	owner.mu.Unlock()
	issuerReserve := qualificationIssuerReserve
	if worker.job.qualification.init.Role == streamqualification.ReaderRole {
		issuerReserve += (3 - worker.qualificationReader) * 9
	}
	if err := owner.ensureQualificationIssuerReserve(ctx, issuerReserve); err != nil {
		return err
	}
	// Observe the prefixes after reserve inspection: a Source prefix retired
	// on its finite post-work interval may have just been reopened by that
	// inspection. Snapshotting before it would replenish a known-dead parent.
	owner.mu.Lock()
	source := owner.currentTextSourceLocked()
	introduction := owner.introduction.currentLocked()
	responder := owner.responder.currentLocked()
	owner.mu.Unlock()
	present := func(hello route.ClosedHello, class uint8) ([]byte, error) {
		return owner.presentQualifiedRefill(ctx, worker.job, hello, class)
	}
	if source != nil {
		if err := source.replenish(ctx, present); err != nil {
			return err
		}
	}
	if introduction != nil {
		if err := introduction.replenish(ctx, present); err != nil {
			return err
		}
	}
	if responder != nil {
		if err := responder.replenish(ctx, present); err != nil {
			return err
		}
	}
	for _, joined := range joins {
		if err := joined.Replenish(ctx, present); err != nil {
			retained := worker.job.qualification.retains(joined)
			// A retiring transport's Service owner retains its terminal cause.
			if retained {
				return err
			}
		}
	}
	return nil
}

func (owner *textContext) ensureQualificationTokenReserve(ctx context.Context, receiver [32]byte, class uint8, minimum int) error {
	if owner == nil || ctx == nil || ctx.Err() != nil || receiver == [32]byte{} || class < 1 || class > 3 || minimum < 1 || minimum > 32 {
		return errors.New("qualification token reserve unavailable")
	}
	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	if err != nil || owner.permission == nil {
		owner.mu.Unlock()
		return errors.Join(err, errors.New("qualification token reserve unavailable"))
	}
	ready := owner.permission.stockCountFor(profile.Digest, receiver, class)
	remaining := owner.permission.accepted.Maxima[class-1] - owner.permission.reserved[class-1]
	owner.mu.Unlock()
	missing := min(minimum-ready, int(remaining))
	if missing <= 0 {
		return nil
	}
	receivers := make([][32]byte, missing)
	for index := range receivers {
		receivers[index] = receiver
	}
	return owner.issueTextTokens(ctx, receivers, class)
}

func (owner *textContext) ensureQualificationIssuerReserve(ctx context.Context, minimum int) error {
	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	if err != nil || owner.permission == nil || ctx.Err() != nil {
		owner.mu.Unlock()
		return errors.Join(err, ctx.Err(), errors.New("qualification issuer reserve unavailable"))
	}
	ready := owner.permission.stockCountForDuty(profile.Digest, profile.IssuerNodeID, profile.IssuerDutyGeneration, 1)
	remaining := owner.permission.accepted.Maxima[0] - owner.permission.reserved[0]
	prefixLive := owner.currentTextSourceLocked() != nil
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
			inProgress := owner.currentTextSourceLocked() != nil || owner.source.openingInProgressLocked() || owner.issuance != nil
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
	stocked := owner.permission.stockCountForDuty(profile.Digest, hello.RecipientNodeID, hello.RecipientDutyGeneration, 2) != 0
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

//go:build linux

package endpoint

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/qualification"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// The qualification scenario owner lives in internal/qualification. This file
// is the Endpoint side of that boundary: the bounded, authorized participant
// operations behind permission checks that the scenario consumes. Every
// operation rechecks its own authority under the Context lock; private
// Context, Job, permission, Source, and worker state never crosses.

// Each refill is capped to eight tokens so one CPU can continue sampling
// every Node during issuance.
const qualificationIssuerRefill = 8

func (worker *qualifiedTextWorker) Run() *qualification.Run { return worker.job.qualification }

func (worker *qualifiedTextWorker) Now() time.Time { return worker.job.owner.endpoint.clock() }

func (worker *qualifiedTextWorker) Artifact() *qualification.Artifact {
	return worker.lifetime.qualificationArtifact()
}

func (worker *qualifiedTextWorker) Cgroup() string { return worker.lifetime.cgroup }

func (worker *qualifiedTextWorker) WorkerAttachment() io.ReadWriteCloser {
	return worker.lifetime.attachment
}

func (worker *qualifiedTextWorker) RefillSnapshot() qualification.Refill {
	owner := worker.job.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	snapshot := qualification.Refill{Live: owner.liveTextServiceJobLocked(worker.job, owner.surface)}
	if !snapshot.Live {
		return snapshot
	}
	snapshot.Busy = owner.issuance != nil
	if snapshot.Busy {
		return snapshot
	}
	snapshot.Joins = worker.job.qualification.JoinedStreams()
	return snapshot
}

func (worker *qualifiedTextWorker) EnsureIssuerReserve(ctx context.Context, minimum int) error {
	return worker.job.owner.ensureQualificationIssuerReserve(ctx, minimum)
}

func (worker *qualifiedTextWorker) PresentRefill(ctx context.Context, hello ardp.Hello, class uint8) ([]byte, error) {
	return worker.job.owner.presentQualifiedRefill(ctx, worker.job, hello, class)
}

func (worker *qualifiedTextWorker) ReplenishPrefixes(ctx context.Context, present client.ClosedTokenPresenter) error {
	owner := worker.job.owner
	owner.mu.Lock()
	source := owner.source.currentLocked()
	introduction := owner.introduction.currentLocked()
	responder := owner.responder.currentLocked()
	owner.mu.Unlock()
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
	return nil
}

// qualificationSession binds one measured run's launch authority to the
// authorized participant operations of its Context, surface, permission
// files, and worker.
type qualificationSession struct {
	*qualifiedTextWorker
	owner            *textContext
	surface          broker.Surface
	permission       TextPermissionFiles
	reportPermission func(context.Context, [32]byte) error
}

func (session *qualificationSession) BeginOperation(ctx context.Context) (context.Context, func(), error) {
	return session.qualifiedTextWorker.beginOperation(ctx, session.surface)
}

func (session *qualificationSession) ProvisionPermission(ctx context.Context) error {
	return session.owner.provisionTextPermission(ctx, session.permission.RequestPath, session.permission.ResponsePath, session.permission.Maxima, session.reportPermission)
}

func (session *qualificationSession) ResolveIntroduction(ctx context.Context, destination targetlink.Link) (reachability.Verified, error) {
	return session.owner.resolveTextIntroduction(ctx, session.job, destination)
}

func (session *qualificationSession) PrepareIntroduction(ctx context.Context, destination targetlink.Link, bounds [3]int64, verified reachability.Verified) (qualification.Preparation, error) {
	attempt, err := session.owner.prepareResolvedTextIntroduction(ctx, session.job, destination, bounds, verified)
	if err != nil {
		return nil, err
	}
	return qualificationPreparation{attempt: attempt}, nil
}

func (session *qualificationSession) OpenJoinedService(ctx context.Context, preparation qualification.Preparation, setupComplete func()) (connection.Stream, error) {
	prepared, ok := preparation.(qualificationPreparation)
	if !ok || prepared.attempt == nil {
		return nil, errors.New("qualification introduction preparation unavailable")
	}
	service, err := session.owner.openTextJoinedServiceAfterSetup(ctx, session.job, prepared.attempt, setupComplete)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func (session *qualificationSession) ReserveRecipients() (qualification.ReserveRecipients, error) {
	owner := session.owner
	owner.mu.Lock()
	prefix := owner.source.currentLocked()
	owner.mu.Unlock()
	if prefix == nil {
		return qualification.ReserveRecipients{}, errors.New("qualification Source prefix unavailable")
	}
	var recipients qualification.ReserveRecipients
	recipients.Join, _, _, recipients.JoinErr = prefix.dataJoinRecipient()
	recipients.Submission, recipients.SubmissionErr = prefix.submissionRecipient()
	return recipients, nil
}

func (session *qualificationSession) EnsureTokenReserve(ctx context.Context, receiver [32]byte, class uint8, minimum int) error {
	return session.owner.ensureQualificationTokenReserve(ctx, receiver, class, minimum)
}

func (session *qualificationSession) StartPublication(ctx context.Context) (qualification.Publication, error) {
	publication, err := session.qualifiedTextWorker.startPublication(ctx)
	if err != nil {
		return nil, err
	}
	return publication, nil
}

// qualificationPreparation is the opaque handle for one resolved Introduction
// preparation. Endpoint owns the attempt state; the scenario only hands it
// back to OpenJoinedService.
type qualificationPreparation struct {
	attempt *textIntroductionAttempt
}

func (qualificationPreparation) QualificationPreparation() {}

func (owner *textContext) streamConnectionLimitLocked() int {
	if owner.job != nil && owner.job.qualification != nil {
		schedule, err := owner.job.qualification.Init().Profile.Definition(owner.job.qualification.Init().Role)
		if err == nil {
			return int(schedule.OpenConnections)
		}
	}
	return 16
}

func (owner *textContext) streamExchangeLimitLocked() int {
	if owner.job != nil && owner.job.qualification != nil {
		// Retained transports plus finite simultaneous introduction/recovery work.
		return owner.streamConnectionLimitLocked() + 16
	}
	return 16
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
	remaining := owner.permission.remaining(class)
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
	remaining := owner.permission.remaining(1)
	prefixLive := owner.source.currentLocked() != nil
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
			inProgress := owner.source.currentLocked() != nil || owner.source.openingInProgressLocked() || owner.issuance != nil
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

func (owner *textContext) presentQualifiedRefill(ctx context.Context, job *textJobIdentity, hello ardp.Hello, class uint8) ([]byte, error) {
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

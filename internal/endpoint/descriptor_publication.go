//go:build linux

package endpoint

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	servicepublication "github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// publishDescriptor consumes only the Endpoint's accepted Instance and
// registered slot. It commits the real private proof at the resolution Node;
// no worker supplies a signer, recipient key, Target, or publication bytes.
// Descriptor acknowledgement alone does not imply working capsule delivery.
func (owner *dutyContext) publishDescriptor(ctx context.Context) (verified reachability.Verified, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return verified, errors.New("text publication unavailable")
	}
	endpoint := owner.endpoint
	endpoint.publisherMu.Lock()
	locked := true
	defer func() {
		if locked {
			endpoint.publisherMu.Unlock()
		}
	}()
	owner.mu.Lock()
	profile, now, err := owner.permissionProfileLocked()
	registered := owner.publication.pair.PublicationTargetLocked()
	acquisition := owner.source.AcquireResolutionLocked()
	reason := ""
	switch {
	case err != nil:
		reason = "permission profile: " + err.Error()
	case owner.surface != broker.Administration:
		reason = "context is not Administration"
	case registered == nil:
		reason = "Introduction registration is absent"
	case owner.publication.pair.DrainingLocked():
		reason = "Introduction registration is draining"
	case owner.publication.pair.WithdrawalInProgressLocked():
		reason = "Introduction registration withdrawal is in progress"
	case owner.publication.pair.OpeningInProgressLocked():
		reason = "Introduction registration opening is in progress"
	case owner.tokens.Permission == nil:
		reason = "Permission is absent"
	case owner.resolution.BusyLocked():
		reason = "resolution flight is active"
	case acquisition == nil:
		reason = "Source is absent"
	case endpoint.publisherBinding == nil:
		reason = "Publisher binding is absent"
	case endpoint.publications == nil:
		reason = "Publication owner is absent"
	case endpoint.publisherOwner != nil && endpoint.publisherOwner != owner:
		reason = "another Publisher context owns the publication"
	}
	if reason != "" {
		if acquisition != nil {
			acquisition.Release()
		}
		owner.mu.Unlock()
		return verified, fmt.Errorf("text publication owner unavailable: %s", reason)
	}
	if registered.Ended() {
		acquisition.Release()
		owner.mu.Unlock()
		return verified, errors.New("text registration ended")
	}
	flight := owner.resolution.BeginLocked(owner.lease.Context(), ctx, acquisition)
	if flight == nil {
		owner.mu.Unlock()
		return verified, errors.New("text publication owner unavailable: resolution flight is active")
	}
	attempt := flight.context
	owner.mu.Unlock()
	defer func() { outcome = owner.finishResolution(flight, outcome, ctx) }()
	binding := endpoint.publisherBinding
	lease, err := owner.acquirePublication(attempt, registered, binding, now)
	if err != nil {
		return verified, err
	}
	endpoint.publisherOwner = owner
	defer func() { outcome = errors.Join(outcome, lease.Close()) }()
	current := lease.Current()
	owner.mu.Lock()
	live, at, err := owner.permissionProfileLocked()
	if err != nil || live != profile || owner.publication.pair.PublicationTargetLocked() != registered || owner.publication.pair.DrainingLocked() || !owner.liveLocked(endpoint, broker.Administration) || attempt.Err() != nil {
		owner.mu.Unlock()
		return verified, errors.New("text publication authority changed")
	}
	if registered.Ended() {
		owner.mu.Unlock()
		return verified, errors.New("text registration ended")
	}
	if !registered.HasPrivateProofLocked() {
		at = at.UTC().Truncate(time.Second)
		revision, expiry := registered.RegistrationWindow()
		recipient, err := binding.NewPrivateRecipient(revision, at, expiry)
		if err != nil {
			owner.mu.Unlock()
			return verified, err
		}
		raw, _, err := reachability.IssuePrivate(reachability.PrivateIssueInput{Current: current, ProfileDigest: profile.Digest, InstanceSigner: binding,
			Introduction: registered.PrivateIntroduction(recipient.Public(at), at)})
		if err != nil {
			owner.mu.Unlock()
			return verified, errors.Join(err, recipient.Close())
		}
		joined := make(chan struct{})
		registered.AttachPrivateProofLocked(recipient, raw, joined)
		go func() { defer close(joined); <-registered.DoneSignal(); _ = recipient.Close() }()
	}
	raw := registered.CopyDescriptorLocked()
	owner.mu.Unlock()
	// The exact context retains exclusive Instance ownership while the flight
	// waits on the network. Legacy start/withdraw cannot acquire this owner;
	// context shutdown cancels and joins this flight before retiring the binding.
	endpoint.publisherMu.Unlock()
	locked = false
	defer clear(raw)
	// An exact retry retains the same signed bytes and key. A new profile or
	// revision requires explicit registration replacement, never a silent edit.
	verified, err = reachability.VerifyPrivate(raw, current.Credential.Target, profile.NetworkID, profile.Digest, at)
	if err != nil {
		return reachability.Verified{}, err
	}
	receiver, err := flight.source.ResolutionRecipient()
	if err != nil {
		return reachability.Verified{}, err
	}
	flight.receiver = receiver
	if err := owner.prepareSourceReopen(attempt, flight); err != nil {
		return reachability.Verified{}, errors.Join(errors.New("text publication Source token preparation failed"), err)
	}
	if err := owner.ensureResolutionStock(flight); err != nil {
		return reachability.Verified{}, errors.Join(errors.New("text publication resolution token preparation failed"), err)
	}
	status, _, err := flight.source.ExchangeDescriptor(attempt, func(hello ardp.Hello, class uint8) ([]byte, error) {
		return owner.presentResolutionToken(flight, hello, class)
	}, [32]byte{}, raw)
	if err != nil || status != 0 {
		return reachability.Verified{}, errors.Join(fmt.Errorf("text Descriptor publication refused: status=%d", status), err)
	}
	endpoint.publisherMu.Lock()
	locked = true
	owner.mu.Lock()
	defer owner.mu.Unlock()
	live, at, err = owner.permissionProfileLocked()
	if err != nil || live != profile || owner.publication.pair.PublicationTargetLocked() != registered || owner.publication.pair.DrainingLocked() || !owner.resolution.CurrentSourceLocked(flight, &owner.source) ||
		endpoint.publisherOwner != owner || endpoint.publisherBinding != binding || !endpoint.publicationLive ||
		!owner.liveLocked(endpoint, broker.Administration) || attempt.Err() != nil || ctx.Err() != nil || registered.RecipientPublicLocked(at) == [32]byte{} {
		return reachability.Verified{}, errors.New("text Descriptor acknowledgement outlived its owner")
	}
	if registered.Ended() {
		return reachability.Verified{}, errors.New("text registration ended before acknowledgement")
	}
	verified, err = reachability.VerifyPrivate(raw, current.Credential.Target, profile.NetworkID, profile.Digest, at)
	if err == nil {
		wasPublished := registered.PublishedLocked()
		if err := owner.publication.pair.CommitAcknowledgedLocked(ctx, registered, at); err != nil {
			return reachability.Verified{}, err
		}
		if !wasPublished {
			owner.publication.signalRegistrationsLocked()
		}
		owner.startRefreshLocked(registered)
	}
	return verified, err
}

// publisherMu serializes this existing publication/Instance ownership with
// legacy start and withdrawal. Local publication proof is distinct from the
// later resolution acknowledgement and eventual protected Service readiness.
func (owner *dutyContext) acquirePublication(ctx context.Context, registered *introduction.Registration, binding *instance.Binding, now time.Time) (*servicepublication.Lease, error) {
	endpoint := owner.endpoint
	credential := binding.Credential()
	if err := validateCredential(credential, endpoint.authority, endpoint.network, now, publishCapability|connectCapability); err != nil {
		return nil, err
	}
	if lease, err := endpoint.publications.AcquireAt(ctx, now); err == nil {
		if lease.Current().Credential != credential {
			return nil, errors.Join(errors.New("text Instance differs from live publication"), lease.Close())
		}
		endpoint.publisherOwner, endpoint.publicationLive = owner, true
		if err := binding.CommitPublished(credential.Generation); err != nil {
			return nil, errors.Join(err, lease.Close())
		}
		return lease, nil
	}
	floor, err := endpoint.publications.Floor()
	if err != nil || floor >= credential.Generation {
		return nil, errors.Join(errors.New("text publication successor required"), err)
	}
	// Claim cleanup before a durable commit can outlive caller cancellation.
	endpoint.publisherOwner = owner
	_, err = endpoint.publications.PublishAfterReadiness(ctx, servicepublication.PublishInput{Credential: credential, InstanceSigner: binding, At: now}, func(ctx context.Context) ([]byte, error) {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		if ctx.Err() != nil || owner.publication.pair.PublicationTargetLocked() != registered || owner.publication.pair.DrainingLocked() || !owner.liveLocked(endpoint, broker.Administration) {
			return nil, errors.New("text registration owner changed")
		}
		if registered.Ended() {
			return nil, errors.New("text registration ended")
		}
		receipt := registered.AckReceipt()
		if receipt == [32]byte{} {
			return nil, errors.New("text registration has no acknowledgement")
		}
		if err := binding.CommitPublished(credential.Generation); err != nil {
			return nil, err
		}
		return receipt[:], nil
	})
	return owner.finishPublicationCommit(ctx, registered, binding, now, err)
}

// Every commit outcome retains cleanup ownership before a cancellable handover.
func (owner *dutyContext) finishPublicationCommit(ctx context.Context, registered *introduction.Registration, binding *instance.Binding, now time.Time, commitErr error) (*servicepublication.Lease, error) {
	endpoint := owner.endpoint
	endpoint.publisherOwner = owner
	if commitErr != nil {
		registered.Cancel()
		channelErr := registered.Close()
		withdrawErr := binding.Withdraw()
		if withdrawErr == nil {
			endpoint.publisherBinding = nil
		}
		cleanup := errors.Join(channelErr, withdrawErr)
		if cleanup != nil {
			owner.mu.Lock()
			owner.closeErr = errors.Join(owner.closeErr, cleanup)
			owner.closed = true
			endpoint.failDutyContexts(cleanup)
			owner.mu.Unlock()
		}
		return nil, errors.Join(commitErr, cleanup)
	}
	endpoint.publicationLive = true
	return endpoint.publications.AcquireAt(ctx, now)
}

// Context shutdown calls this only after its flights and worker have joined.
// The accepted Instance cannot transfer to a different local context after
// first publication; a new owner requires an explicit successor binding.
func (owner *dutyContext) retirePublication() error {
	endpoint := owner.endpoint
	endpoint.publisherMu.Lock()
	defer endpoint.publisherMu.Unlock()
	if endpoint.publisherOwner != owner || endpoint.publisherBinding == nil {
		return nil
	}
	if endpoint.publicationLive {
		if err := endpoint.publications.Unpublish(context.Background()); err != nil {
			return err
		}
		endpoint.publicationLive = false
	}
	if err := endpoint.publisherBinding.Withdraw(); err != nil {
		return err
	}
	endpoint.publisherBinding = nil
	return nil
}

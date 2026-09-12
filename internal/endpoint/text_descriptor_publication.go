//go:build linux

package endpoint

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// publishTextDescriptor consumes only the Endpoint's accepted Instance and
// registered slot. It commits the real private proof at the resolution Node;
// no worker supplies a signer, recipient key, Target, or publication bytes.
// Descriptor acknowledgement alone does not imply working capsule delivery.
func (owner *textContext) publishTextDescriptor(ctx context.Context) (verified reachability.Verified, outcome error) {
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
	profile, now, err := owner.textPermissionProfileLocked()
	registered := owner.registration
	if err != nil || owner.surface != broker.Administration || registered == nil || owner.withdrawal != nil ||
		owner.registrationOpening != nil || owner.permission == nil || owner.resolution != nil || owner.prefix == nil ||
		endpoint.publisherBinding == nil || endpoint.publications == nil || endpoint.publisherSession != nil || endpoint.textPublisherOwner != nil && endpoint.textPublisherOwner != owner {
		owner.mu.Unlock()
		return verified, errors.New("text publication owner unavailable")
	}
	select {
	case <-registered.channel.Done():
		owner.mu.Unlock()
		return verified, errors.New("text registration ended")
	default:
	}
	attempt, cancel := context.WithCancel(owner.lease.Context())
	flight := &textResolutionFlight{context: attempt, cancel: cancel, done: make(chan struct{}), prefix: owner.prefix}
	owner.resolution = flight
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	defer func() {
		cancel()
		if !stop() {
			<-interrupted
		}
		outcome = errors.Join(outcome, ctx.Err())
		owner.finishTextResolution(flight, outcome)
	}()
	binding := endpoint.publisherBinding
	lease, err := owner.acquireTextPublication(attempt, registered, binding, now)
	if err != nil {
		return verified, err
	}
	endpoint.textPublisherOwner = owner
	defer func() { outcome = errors.Join(outcome, lease.Close()) }()
	current := lease.Current()
	owner.mu.Lock()
	live, at, err := owner.textPermissionProfileLocked()
	if err != nil || live != profile || owner.registration != registered || !owner.liveLocked(endpoint, broker.Administration) || attempt.Err() != nil {
		owner.mu.Unlock()
		return verified, errors.New("text publication authority changed")
	}
	select {
	case <-registered.channel.Done():
		owner.mu.Unlock()
		return verified, errors.New("text registration ended")
	default:
	}
	if len(registered.descriptor) == 0 {
		at = at.UTC().Truncate(time.Second)
		recipient, err := binding.NewPrivateRecipient(registered.request.Revision, at, registered.request.Expiry)
		if err != nil {
			owner.mu.Unlock()
			return verified, err
		}
		raw, _, err := reachability.IssuePrivate(reachability.PrivateIssueInput{Current: current, ProfileDigest: profile.Digest, InstanceSigner: binding,
			Introduction: reachability.PrivateIntroduction{Revision: registered.request.Revision, NodeID: registered.node,
				Slot: registered.request.Slot, RecipientKey: recipient.Public(at), NotBefore: at, NotAfter: registered.request.Expiry}})
		if err != nil {
			owner.mu.Unlock()
			return verified, errors.Join(err, recipient.Close())
		}
		registered.recipient, registered.descriptor = recipient, raw
		joined := make(chan struct{})
		registered.recipientDone = joined
		go func() { defer close(joined); <-registered.channel.Done(); _ = recipient.Close() }()
	}
	raw := append([]byte(nil), registered.descriptor...)
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
	receiver, err := flight.prefix.ResolutionRecipient()
	if err != nil {
		return reachability.Verified{}, err
	}
	flight.receiver = receiver
	if err := owner.prepareTextSourceReopen(attempt, flight); err != nil {
		return reachability.Verified{}, errors.Join(errors.New("text publication Source token preparation failed"), err)
	}
	if err := owner.ensureTextResolutionStock(flight); err != nil {
		return reachability.Verified{}, errors.Join(errors.New("text publication resolution token preparation failed"), err)
	}
	status, _, err := flight.prefix.ExchangeDescriptor(attempt, func(hello route.ClosedHello, class uint8) ([]byte, error) {
		return owner.presentTextResolutionToken(flight, hello, class)
	}, [32]byte{}, raw)
	if err != nil || status != 0 {
		return reachability.Verified{}, errors.Join(fmt.Errorf("text Descriptor publication refused: status=%d", status), err)
	}
	endpoint.publisherMu.Lock()
	locked = true
	owner.mu.Lock()
	defer owner.mu.Unlock()
	live, at, err = owner.textPermissionProfileLocked()
	if err != nil || live != profile || owner.registration != registered || owner.prefix != flight.prefix ||
		endpoint.textPublisherOwner != owner || endpoint.publisherBinding != binding || !endpoint.textPublicationLive || endpoint.publisherSession != nil ||
		!owner.liveLocked(endpoint, broker.Administration) || attempt.Err() != nil || ctx.Err() != nil || registered.recipient.Public(at) == [32]byte{} {
		return reachability.Verified{}, errors.New("text Descriptor acknowledgement outlived its owner")
	}
	select {
	case <-registered.channel.Done():
		return reachability.Verified{}, errors.New("text registration ended before acknowledgement")
	default:
	}
	verified, err = reachability.VerifyPrivate(raw, current.Credential.Target, profile.NetworkID, profile.Digest, at)
	if err == nil {
		if !registered.published {
			if previous := owner.previousRegistration; previous != nil {
				switchAt := at.UTC().Truncate(time.Second)
				until := switchAt.Add(60 * time.Second)
				if previous.request.Expiry.Before(until) {
					until = previous.request.Expiry
				}
				if switchAt.Before(until) {
					if err := previous.recipient.RetainPredecessor(switchAt, until); err != nil {
						return reachability.Verified{}, err
					}
				}
				owner.previousUntil = until
			}
			registered.publishedAt = at
			registered.published = true
			owner.signalTextRegistrationsLocked()
		}
		owner.startTextRefreshLocked(registered)
	}
	return verified, err
}

// publisherMu serializes this existing publication/Instance ownership with
// legacy start and withdrawal. Local publication proof is distinct from the
// later resolution acknowledgement and eventual protected Service readiness.
func (owner *textContext) acquireTextPublication(ctx context.Context, registered *textIntroductionRegistration, binding *instance.Binding, now time.Time) (*publication.Lease, error) {
	endpoint := owner.endpoint
	credential := binding.Credential()
	if err := validateCredential(credential, endpoint.authority, endpoint.network, now, publishCapability|connectCapability); err != nil {
		return nil, err
	}
	if lease, err := endpoint.publications.AcquireAt(ctx, now); err == nil {
		if lease.Current().Credential != credential {
			return nil, errors.Join(errors.New("text Instance differs from live publication"), lease.Close())
		}
		endpoint.textPublisherOwner, endpoint.textPublicationLive = owner, true
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
	endpoint.textPublisherOwner = owner
	_, err = endpoint.publications.PublishAfterReadiness(ctx, publication.PublishInput{Credential: credential, InstanceSigner: binding, At: now}, func(ctx context.Context) ([]byte, error) {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		if ctx.Err() != nil || owner.registration != registered || !owner.liveLocked(endpoint, broker.Administration) {
			return nil, errors.New("text registration owner changed")
		}
		select {
		case <-registered.channel.Done():
			return nil, errors.New("text registration ended")
		default:
		}
		receipt := registered.channel.Receipt()
		if receipt == [32]byte{} {
			return nil, errors.New("text registration has no acknowledgement")
		}
		if err := binding.CommitPublished(credential.Generation); err != nil {
			return nil, err
		}
		return receipt[:], nil
	})
	return owner.finishTextPublicationCommit(ctx, registered, binding, now, err)
}

// Every commit outcome retains cleanup ownership before a cancellable handover.
func (owner *textContext) finishTextPublicationCommit(ctx context.Context, registered *textIntroductionRegistration, binding *instance.Binding, now time.Time, commitErr error) (*publication.Lease, error) {
	endpoint := owner.endpoint
	endpoint.textPublisherOwner = owner
	if commitErr != nil {
		registered.cancel()
		channelErr := registered.close()
		withdrawErr := binding.Withdraw()
		if withdrawErr == nil {
			endpoint.publisherBinding = nil
		}
		cleanup := errors.Join(channelErr, withdrawErr)
		if cleanup != nil {
			owner.mu.Lock()
			owner.closeErr = errors.Join(owner.closeErr, cleanup)
			owner.closed = true
			endpoint.failTextContexts(cleanup)
			owner.mu.Unlock()
		}
		return nil, errors.Join(commitErr, cleanup)
	}
	endpoint.textPublicationLive = true
	return endpoint.publications.AcquireAt(ctx, now)
}

// Context shutdown calls this only after its flights and worker have joined.
// The accepted Instance cannot transfer to a different local context after
// first publication; a new owner requires an explicit successor binding.
func (owner *textContext) retireTextPublication() error {
	endpoint := owner.endpoint
	endpoint.publisherMu.Lock()
	defer endpoint.publisherMu.Unlock()
	if endpoint.textPublisherOwner != owner || endpoint.publisherBinding == nil {
		return nil
	}
	if endpoint.textPublicationLive {
		if err := endpoint.publications.Unpublish(context.Background()); err != nil {
			return err
		}
		endpoint.textPublicationLive = false
	}
	if err := endpoint.publisherBinding.Withdraw(); err != nil {
		return err
	}
	endpoint.publisherBinding = nil
	return nil
}

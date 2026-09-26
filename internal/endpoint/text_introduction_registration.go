//go:build linux

package endpoint

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	introductioncapsule "github.com/dianabuilds/ardents-network/internal/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

const textRegistrationRefreshDelay = 300 * time.Second

type textRegistrationFlight struct {
	previous *textIntroductionRegistration
	context  context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	prefix   *textIntroductionPrefixHandle
	receiver [32]byte
}

func (flight *textRegistrationFlight) stop() {
	if flight != nil {
		flight.cancel()
	}
}

func (flight *textRegistrationFlight) join() {
	if flight != nil {
		<-flight.done
	}
}

type textIntroductionRegistration struct {
	createdAt     time.Time
	refreshAt     time.Time
	published     bool
	publishedAt   time.Time // First verified ACK transition, retained across exact retries.
	recipient     *instance.PrivateRecipient
	recipientDone <-chan struct{}
	descriptor    []byte
	channel       *route.ClosedIntroductionRegistration
	node          [32]byte
	request       terminal.RegistrationRequest
	cancel        context.CancelFunc
}

// registerTextIntroduction owns the fresh random slot and spends a real
// Publication token on the separate admitted Introduction tree. Registration
// supplies no Service authority and is not Descriptor publication readiness.
func (owner *textContext) registerTextIntroduction(ctx context.Context, revision uint64, expiry time.Time) (*textIntroductionRegistration, error) {
	return owner.openTextRegistration(ctx, revision, expiry, nil)
}

func (owner *textContext) openTextRegistration(ctx context.Context, revision uint64, expiry time.Time, previous *textIntroductionRegistration) (registered *textIntroductionRegistration, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || revision == 0 {
		return nil, errors.New("text Publisher registration unavailable")
	}
	owner.mu.Lock()
	profile, now, err := owner.textPermissionProfileLocked()
	if previous == nil {
		if ended := owner.publication.evictEndedTargetLocked(); ended != nil {
			cleanup := ended.close()
			ended.cancel()
			owner.signalTextRegistrationsLocked()
			if cleanup != nil {
				owner.closeErr = errors.Join(owner.closeErr, cleanup)
				owner.closed = true
				owner.endpoint.failTextContexts(cleanup)
				err = errors.Join(err, cleanup)
			}
		}
	}
	prefix := owner.introduction.currentLocked()
	prior, _ := owner.publication.previousLocked()
	if err != nil || owner.surface != broker.Administration || prefix == nil || owner.introduction.openingInProgressLocked() || previous != nil && (prior != nil || !owner.refresh.matchesContext(ctx) || previous.recipient == nil || revision <= previous.request.Revision) || owner.permission == nil || !now.Before(expiry) || expiry.After(now.Add(600*time.Second)) {
		owner.mu.Unlock()
		return nil, errors.New("text Publisher registration owner unavailable")
	}
	attempt, cancel := context.WithCancel(owner.lease.Context())
	flight := &textRegistrationFlight{previous: previous, context: attempt, cancel: cancel, done: make(chan struct{}), prefix: prefix}
	if !owner.publication.beginOpeningLocked(flight, previous) {
		owner.mu.Unlock()
		cancel()
		return nil, errors.New("text Publisher registration owner unavailable")
	}
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	var channel *route.ClosedIntroductionRegistration
	defer func() {
		if !stop() {
			<-interrupted
		}
		registered, outcome = owner.finishTextRegistration(ctx, flight, registered, channel, outcome)
	}()
	receiver, until, err := flight.prefix.introductionRecipient()
	if err != nil || expiry.After(until) {
		return nil, errors.Join(err, errors.New("text Publisher registration expiry exceeds current duty"))
	}
	flight.receiver = receiver
	owner.mu.Lock()
	ready := !owner.permission.hasPending() &&
		owner.permission.stockCountFor(profile.Digest, receiver, 3) != 0
	owner.mu.Unlock()
	if !ready {
		if err := owner.issueTextTokens(attempt, [][32]byte{receiver}, 3); err != nil {
			return nil, err
		}
	}
	request := terminal.RegistrationRequest{Revision: revision, Expiry: expiry}
	if _, err := rand.Read(request.Slot[:]); err != nil {
		return nil, err
	}
	createdAt := owner.endpoint.clock().UTC().Truncate(time.Second)
	channel, err = flight.prefix.register(attempt, func(hello ardp.Hello, class uint8) ([]byte, error) {
		return owner.presentTextRegistrationToken(flight, hello, class)
	}, request)
	if err != nil {
		return nil, err
	}
	select {
	case <-channel.Done():
		return nil, errors.New("text Publisher registration ended before handover")
	default:
	}
	return &textIntroductionRegistration{createdAt: createdAt, channel: channel, node: receiver, request: request, cancel: cancel}, nil
}

func (owner *textContext) finishTextRegistration(ctx context.Context, flight *textRegistrationFlight, registered *textIntroductionRegistration, channel *route.ClosedIntroductionRegistration, outcome error) (*textIntroductionRegistration, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer close(flight.done)
	current := owner.publication.finishOpeningLocked(flight)
	if outcome != nil || ctx.Err() != nil || flight.context.Err() != nil || !current || !flight.prefix.currentLocked(&owner.introduction) ||
		!owner.publication.openingBaseLocked(flight.previous) || !owner.liveLocked(owner.endpoint, broker.Administration) {
		flight.cancel()
		cleanup := channel.Close()
		if errors.Is(outcome, route.ErrClosedSourceCleanup) || cleanup != nil {
			owner.closeErr = errors.Join(owner.closeErr, outcome, cleanup)
			owner.closed = true
			owner.endpoint.failTextContexts(owner.closeErr)
		}
		return nil, errors.Join(outcome, ctx.Err(), cleanup, errors.New("text Publisher registration did not complete"))
	}
	// Registration alone cannot switch published readiness or shorten its
	// predecessor. The verified Descriptor ACK establishes the overlap.
	if !owner.publication.installLocked(flight.previous, registered) {
		flight.cancel()
		cleanup := channel.Close()
		if cleanup != nil {
			owner.closeErr = errors.Join(owner.closeErr, cleanup)
			owner.closed = true
			owner.endpoint.failTextContexts(cleanup)
		}
		return nil, errors.Join(cleanup, errors.New("text Publisher registration owner changed before install"))
	}
	owner.signalTextRegistrationsLocked()
	return registered, nil
}

func (owner *textContext) presentTextRegistrationToken(flight *textRegistrationFlight, hello ardp.Hello, class uint8) ([]byte, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil || owner.surface != broker.Administration || !owner.publication.openingCurrentLocked(flight) || !flight.prefix.currentLocked(&owner.introduction) || flight.context.Err() != nil || owner.permission == nil ||
		class != 3 || hello.Purpose != ardp.PurposeIntroduction || hello.RecipientNodeID != flight.receiver || hello.NetworkID != profile.NetworkID || hello.ProfileDigest != profile.Digest ||
		hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, errors.New("text Publisher registration token authority unavailable")
	}
	receiver, _, err := flight.prefix.introductionRecipient()
	if err != nil || receiver != flight.receiver {
		return nil, errors.New("text Publisher registration recipient changed")
	}
	return owner.takeTextTokenLocked(profile, now, hello, class, flight.context)
}

func (owner *textContext) withdrawTextIntroduction(ctx context.Context) error {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return errors.New("text Publisher registration unavailable")
	}
	owner.mu.Lock()
	registered := owner.publication.publicationTargetLocked()
	if registered == nil || owner.resolution != nil || owner.publication.openingInProgressLocked() || owner.publication.withdrawalInProgressLocked() || !owner.liveLocked(owner.endpoint, broker.Administration) {
		owner.mu.Unlock()
		return errors.New("text Publisher registration absent or ending")
	}
	attempt, cancel := context.WithCancel(owner.lease.Context())
	flight := &textOperationFlight{context: attempt, cancel: cancel, done: make(chan struct{})}
	if !owner.publication.reserveWithdrawalLocked(flight) {
		cancel()
		owner.mu.Unlock()
		return errors.New("text Publisher registration absent or ending")
	}
	owner.mu.Unlock()
	owner.stopTextRefresh()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	err := registered.withdraw(attempt)
	cancel()
	if !stop() {
		<-interrupted
	}
	registered.cancel()
	cleanup := registered.close()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	previous := owner.publication.completeWithdrawalLocked(flight, registered)
	owner.signalTextRegistrationsLocked()
	if previous != nil {
		previous.cancel()
		cleanup = errors.Join(cleanup, previous.close())
	}
	if cleanup != nil {
		owner.closeErr = errors.Join(owner.closeErr, cleanup)
		owner.closed = true
		owner.endpoint.failTextContexts(cleanup)
	}
	close(flight.done)
	return errors.Join(err, cleanup)
}

func (registered *textIntroductionRegistration) close() error {
	if registered == nil {
		return nil
	}
	err := registered.channel.Close()
	if registered.recipientDone != nil {
		<-registered.recipientDone
	}
	return err
}

// ended reports whether the registration channel has already terminated.
func (registered *textIntroductionRegistration) ended() bool {
	select {
	case <-registered.channel.Done():
		return true
	default:
		return false
	}
}

// endReason returns the fixed local terminal category of the channel.
func (registered *textIntroductionRegistration) endReason() route.ClosedIntroductionEndReason {
	return registered.channel.EndReason()
}

// endedStage names the fixed local refresh failure stage of a terminated
// channel. It never serializes a peer, route, or authority fact.
func (registered *textIntroductionRegistration) endedStage() string {
	return "registration-ended-" + string(registered.channel.EndReason())
}

// doneSignal lets one waiter join channel termination without transport access.
func (registered *textIntroductionRegistration) doneSignal() <-chan struct{} {
	return registered.channel.Done()
}

// takeDelivery consumes the next buffered capsule delivery.
func (registered *textIntroductionRegistration) takeDelivery(ctx context.Context) (*route.ClosedIntroductionDelivery, error) {
	return registered.channel.TakeDelivery(ctx)
}

// deliverySignals exposes the channel wake-up and termination signals.
func (registered *textIntroductionRegistration) deliverySignals() (ready, done <-chan struct{}) {
	return registered.channel.DeliveryAvailable(), registered.channel.Done()
}

// withdraw requests the terminal channel withdrawal of this registration.
func (registered *textIntroductionRegistration) withdraw(ctx context.Context) error {
	return registered.channel.Withdraw(ctx)
}

// ackReceipt returns the channel acknowledgement receipt, zero until ACK.
func (registered *textIntroductionRegistration) ackReceipt() [32]byte {
	return registered.channel.Receipt()
}

// expiry returns the registration validity end.
func (registered *textIntroductionRegistration) expiry() time.Time {
	return registered.request.Expiry
}

// matchesRequest compares the immutable slot and revision identity.
func (registered *textIntroductionRegistration) matchesRequest(slot [32]byte, revision uint64) bool {
	return registered != nil && registered.request.Slot == slot && registered.request.Revision == revision
}

// acceptingNowLocked reports the verified ACK and a live private recipient.
func (registered *textIntroductionRegistration) acceptingNowLocked() bool {
	return registered != nil && registered.published && registered.recipient != nil
}

// publishedLocked reports the ACK-committed publication state.
func (registered *textIntroductionRegistration) publishedLocked() bool {
	return registered != nil && registered.published
}

// recipientPublicLocked returns the current public recipient key, zero when
// the recipient is absent.
func (registered *textIntroductionRegistration) recipientPublicLocked(at time.Time) [32]byte {
	if registered == nil || registered.recipient == nil {
		return [32]byte{}
	}
	return registered.recipient.Public(at)
}

// openCapsuleLocked decrypts one submitted capsule against the private
// recipient.
func (registered *textIntroductionRegistration) openCapsuleLocked(capsule introductioncapsule.Capsule, profile [32]byte, at time.Time) (introductioncapsule.Plaintext, [32]byte, error) {
	return introductioncapsule.Open(capsule, profile, registered.recipient, at)
}

// verifyDescriptorLocked verifies the signed Descriptor against live
// publication facts.
func (registered *textIntroductionRegistration) verifyDescriptorLocked(target, network, profile [32]byte, at time.Time) (reachability.Verified, error) {
	return reachability.VerifyPrivate(registered.descriptor, target, network, profile, at)
}

// registrationWindow returns the immutable revision and expiry for recipient
// issuance.
func (registered *textIntroductionRegistration) registrationWindow() (revision uint64, expiry time.Time) {
	return registered.request.Revision, registered.request.Expiry
}

// privateIntroduction assembles the Descriptor introduction facts for one
// issuance.
func (registered *textIntroductionRegistration) privateIntroduction(recipientKey [32]byte, at time.Time) reachability.PrivateIntroduction {
	return reachability.PrivateIntroduction{Revision: registered.request.Revision, NodeID: registered.node,
		Slot: registered.request.Slot, RecipientKey: recipientKey, NotBefore: at, NotAfter: registered.request.Expiry}
}

// commitPublicationLocked records the first verified ACK transition instant.
func (registered *textIntroductionRegistration) commitPublicationLocked(at time.Time) {
	registered.publishedAt = at
	registered.published = true
}

// attachPrivateProofLocked records the one issued recipient and its signed
// Descriptor.
func (registered *textIntroductionRegistration) attachPrivateProofLocked(recipient *instance.PrivateRecipient, descriptor []byte, done <-chan struct{}) {
	registered.recipient, registered.descriptor, registered.recipientDone = recipient, descriptor, done
}

// hasPrivateProofLocked reports whether the signed Descriptor was issued.
func (registered *textIntroductionRegistration) hasPrivateProofLocked() bool {
	return len(registered.descriptor) != 0
}

// copyDescriptorLocked clones the signed Descriptor bytes for one issuance.
func (registered *textIntroductionRegistration) copyDescriptorLocked() []byte {
	return append([]byte(nil), registered.descriptor...)
}

// scheduleRefreshLocked sets the one-time refresh instant from creation. An
// exact retry never moves it.
func (registered *textIntroductionRegistration) scheduleRefreshLocked() {
	if registered.refreshAt.IsZero() {
		registered.refreshAt = registered.createdAt.Add(textRegistrationRefreshDelay)
	}
}

// refreshScheduleLocked returns the current refresh instant and expiry.
func (registered *textIntroductionRegistration) refreshScheduleLocked() (refreshAt, expiry time.Time) {
	return registered.refreshAt, registered.request.Expiry
}

// linkVisibleAtLocked reports whether the committed registration is visible to
// a canonical Link projection now.
func (registered *textIntroductionRegistration) linkVisibleAtLocked(now time.Time) bool {
	return registered.published && !registered.refreshAt.IsZero() && now.Before(registered.request.Expiry)
}

// revisionExhausted reports that no successor revision exists.
func (registered *textIntroductionRegistration) revisionExhausted() bool {
	return registered.request.Revision == ^uint64(0)
}

// nextRevision names the successor revision for rotation.
func (registered *textIntroductionRegistration) nextRevision() uint64 {
	return registered.request.Revision + 1
}

// retainPredecessorLocked shortens the predecessor recipient window during an
// acknowledged switch.
func (registered *textIntroductionRegistration) retainPredecessorLocked(at, until time.Time) error {
	if registered.recipient == nil {
		return errors.New("text publication predecessor recipient unavailable")
	}
	return registered.recipient.RetainPredecessor(at, until)
}

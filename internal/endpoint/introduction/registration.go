//go:build linux

package introduction

import (
	"context"
	"errors"
	"time"

	introductioncapsule "github.com/dianabuilds/ardents-network/internal/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

const refreshDelay = 300 * time.Second

// Registration is one admitted Introduction registration: its route channel,
// private recipient proof, signed Descriptor bytes, refresh schedule, and
// publication acknowledgement state. The root opens, installs, and withdraws
// registrations through the PairLifecycle; this entity only carries the
// per-registration facts.
type Registration struct {
	createdAt     time.Time
	refreshAt     time.Time
	published     bool
	publishedAt   time.Time // First verified ACK transition, retained across exact retries.
	recipient     *instance.PrivateRecipient
	recipientDone <-chan struct{}
	descriptor    []byte
	channel       *client.ClosedIntroductionRegistration
	node          [32]byte
	request       terminal.RegistrationRequest
	cancel        context.CancelFunc
}

// NewRegistration records one freshly registered channel. The cancel belongs
// to the root opening flight lifetime; Cancel and Close are separate so the
// pair retirement can interrupt under the lock and join afterwards.
func NewRegistration(createdAt time.Time, channel *client.ClosedIntroductionRegistration,
	node [32]byte, request terminal.RegistrationRequest, cancel context.CancelFunc) *Registration {
	return &Registration{createdAt: createdAt, channel: channel, node: node,
		request: request, cancel: cancel}
}

// Cancel interrupts the registration attempt lifetime.
func (registered *Registration) Cancel() {
	if registered != nil && registered.cancel != nil {
		registered.cancel()
	}
}

// Close terminates the route channel and joins the private recipient.
func (registered *Registration) Close() error {
	if registered == nil {
		return nil
	}
	err := registered.channel.Close()
	if registered.recipientDone != nil {
		<-registered.recipientDone
	}
	return err
}

// Ended reports whether the registration channel has already terminated.
func (registered *Registration) Ended() bool {
	select {
	case <-registered.channel.Done():
		return true
	default:
		return false
	}
}

// EndReason returns the fixed local terminal category of the channel.
func (registered *Registration) EndReason() client.ClosedIntroductionEndReason {
	return registered.channel.EndReason()
}

// EndedStage names the fixed local refresh failure stage of a terminated
// channel. It never serializes a peer, route, or authority fact.
func (registered *Registration) EndedStage() string {
	return "registration-ended-" + string(registered.channel.EndReason())
}

// DoneSignal lets one waiter join channel termination without transport access.
func (registered *Registration) DoneSignal() <-chan struct{} {
	return registered.channel.Done()
}

// TakeDelivery consumes the next buffered capsule delivery.
func (registered *Registration) TakeDelivery(ctx context.Context) (*client.ClosedIntroductionDelivery, error) {
	return registered.channel.TakeDelivery(ctx)
}

// DeliverySignals exposes the channel wake-up and termination signals.
func (registered *Registration) DeliverySignals() (ready, done <-chan struct{}) {
	return registered.channel.DeliveryAvailable(), registered.channel.Done()
}

// Withdraw requests the terminal channel withdrawal of this registration.
func (registered *Registration) Withdraw(ctx context.Context) error {
	return registered.channel.Withdraw(ctx)
}

// AckReceipt returns the channel acknowledgement receipt, zero until ACK.
func (registered *Registration) AckReceipt() [32]byte {
	return registered.channel.Receipt()
}

// Expiry returns the registration validity end.
func (registered *Registration) Expiry() time.Time {
	return registered.request.Expiry
}

// MatchesRequest compares the immutable slot and revision identity.
func (registered *Registration) MatchesRequest(slot [32]byte, revision uint64) bool {
	return registered != nil && registered.request.Slot == slot && registered.request.Revision == revision
}

// Revision returns the immutable registration revision.
func (registered *Registration) Revision() uint64 {
	return registered.request.Revision
}

// AcceptingNowLocked reports the verified ACK and a live private recipient.
func (registered *Registration) AcceptingNowLocked() bool {
	return registered != nil && registered.published && registered.recipient != nil
}

// PublishedLocked reports the ACK-committed publication state.
func (registered *Registration) PublishedLocked() bool {
	return registered != nil && registered.published
}

// HasRecipientLocked reports whether a private recipient was ever attached.
func (registered *Registration) HasRecipientLocked() bool {
	return registered != nil && registered.recipient != nil
}

// RecipientPublicLocked returns the current public recipient key, zero when
// the recipient is absent.
func (registered *Registration) RecipientPublicLocked(at time.Time) [32]byte {
	if registered == nil || registered.recipient == nil {
		return [32]byte{}
	}
	return registered.recipient.Public(at)
}

// OpenCapsuleLocked decrypts one submitted capsule against the private
// recipient.
func (registered *Registration) OpenCapsuleLocked(capsule introductioncapsule.Capsule, profile [32]byte, at time.Time) (introductioncapsule.Plaintext, [32]byte, error) {
	return introductioncapsule.Open(capsule, profile, registered.recipient, at)
}

// VerifyDescriptorLocked verifies the signed Descriptor against live
// publication facts.
func (registered *Registration) VerifyDescriptorLocked(target, network, profile [32]byte, at time.Time) (reachability.Verified, error) {
	return reachability.VerifyPrivate(registered.descriptor, target, network, profile, at)
}

// RegistrationWindow returns the immutable revision and expiry for recipient
// issuance.
func (registered *Registration) RegistrationWindow() (revision uint64, expiry time.Time) {
	return registered.request.Revision, registered.request.Expiry
}

// PrivateIntroduction assembles the Descriptor introduction facts for one
// issuance.
func (registered *Registration) PrivateIntroduction(recipientKey [32]byte, at time.Time) reachability.PrivateIntroduction {
	return reachability.PrivateIntroduction{Revision: registered.request.Revision, NodeID: registered.node,
		Slot: registered.request.Slot, RecipientKey: recipientKey, NotBefore: at, NotAfter: registered.request.Expiry}
}

// CommitPublicationLocked records the first verified ACK transition instant.
func (registered *Registration) CommitPublicationLocked(at time.Time) {
	registered.publishedAt = at
	registered.published = true
}

// AttachPrivateProofLocked records the one issued recipient and its signed
// Descriptor.
func (registered *Registration) AttachPrivateProofLocked(recipient *instance.PrivateRecipient, descriptor []byte, done <-chan struct{}) {
	registered.recipient, registered.descriptor, registered.recipientDone = recipient, descriptor, done
}

// HasPrivateProofLocked reports whether the signed Descriptor was issued.
func (registered *Registration) HasPrivateProofLocked() bool {
	return len(registered.descriptor) != 0
}

// CopyDescriptorLocked clones the signed Descriptor bytes for one issuance.
func (registered *Registration) CopyDescriptorLocked() []byte {
	return append([]byte(nil), registered.descriptor...)
}

// ScheduleRefreshLocked sets the one-time refresh instant from creation. An
// exact retry never moves it.
func (registered *Registration) ScheduleRefreshLocked() {
	if registered.refreshAt.IsZero() {
		registered.refreshAt = registered.createdAt.Add(refreshDelay)
	}
}

// RefreshScheduleLocked returns the current refresh instant and expiry.
func (registered *Registration) RefreshScheduleLocked() (refreshAt, expiry time.Time) {
	return registered.refreshAt, registered.request.Expiry
}

// LinkVisibleAtLocked reports whether the committed registration is visible to
// a canonical Link projection now.
func (registered *Registration) LinkVisibleAtLocked(now time.Time) bool {
	return registered.published && !registered.refreshAt.IsZero() && now.Before(registered.request.Expiry)
}

// RevisionExhausted reports that no successor revision exists.
func (registered *Registration) RevisionExhausted() bool {
	return registered.request.Revision == ^uint64(0)
}

// NextRevision names the successor revision for rotation.
func (registered *Registration) NextRevision() uint64 {
	return registered.request.Revision + 1
}

// RetainPredecessorLocked shortens the predecessor recipient window during an
// acknowledged switch.
func (registered *Registration) RetainPredecessorLocked(at, until time.Time) error {
	if registered.recipient == nil {
		return errors.New("text publication predecessor recipient unavailable")
	}
	return registered.recipient.RetainPredecessor(at, until)
}

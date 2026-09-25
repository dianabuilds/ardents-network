//go:build linux

package endpoint

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// textServiceBinding retains the exact job and independently verified
// publication. It cannot be reconstructed by a local Application from a
// supplied "established" flag or copied wire nonce.
type textServiceBinding struct {
	owner              *textContext
	job                *textJobIdentity
	credential         publication.Credential
	facts              nativeconnection.ProtectedContextInput
	logical            [32]byte
	candidateView      [32]byte
	destinationBinding [32]byte
	// introduction is the already verified recipient selected for this
	// Target-Link Connection. It remains client-local and finite.
	introduction reachability.PrivateIntroduction
	recovery     *textIntroductionRecoveryOwner
}

// newTextServiceBinding is the Initiator's local owner operation after
// destination authorization and verified reachability. The local Connection
// context and its salt never leave this Endpoint.
func (owner *textContext) newTextServiceBinding(job *textJobIdentity, destination targetlink.Link, current publication.Current,
	bounds [3]int64) (*textServiceBinding, error) {
	if owner == nil {
		return nil, errors.New("text Service context unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.liveTextServiceJobLocked(job, broker.Connection) {
		return nil, errors.New("text Service reader job unavailable")
	}
	profile, _, err := owner.textPermissionProfileLocked()
	if err != nil {
		return nil, err
	}
	spelling, err := targetlink.Encode(destination)
	if err != nil || destination.Network != owner.endpoint.network {
		return nil, errors.New("text Service destination unavailable")
	}
	// This is the existing private ConnectionContext construction. Only its
	// independently salted per-Connection commitment enters the shared tuple.
	destinationBinding := sha256.Sum256([]byte(spelling))
	local, err := nativeconnection.Context(nativeconnection.ContextInput{
		Network: destination.Network, Target: destination.Target, InstancePublic: current.Credential.InstancePublic,
		InstanceGeneration: current.Credential.Generation, PublicationDigest: current.Digest,
		CandidateView: profile.StateDigest, IsolationContext: job.nonce, DestinationBinding: destinationBinding,
		WorkSafetyNotAfter: bounds[0], WorkSafetyMaximum: bounds[1], NoNewRecoveryAfter: bounds[2]})
	if err != nil {
		return nil, err
	}
	var salt, nonce [32]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return nil, err
	}
	defer clear(salt[:])
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	encoded := []byte("ardents-initiator-binding-v3\x00")
	encoded = append(encoded, salt[:]...)
	encoded = append(encoded, local[:]...)
	defer clear(encoded)
	facts := nativeconnection.ProtectedContextInput{
		Network: destination.Network, Target: destination.Target,
		InstancePublic: current.Credential.InstancePublic, InstanceGeneration: current.Credential.Generation,
		PublicationDigest: current.Digest, ProfileDigest: profile.Digest, ConnectionNonce: nonce,
		InitiatorBinding: sha256.Sum256(encoded), WorkSafetyNotAfter: bounds[0], WorkSafetyMaximum: bounds[1], NoNewRecoveryAfter: bounds[2]}
	return owner.bindTextServiceLocked(job, current, facts)
}

func (owner *textContext) bindTextServiceLocked(job *textJobIdentity, current publication.Current,
	facts nativeconnection.ProtectedContextInput) (*textServiceBinding, error) {
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil {
		return nil, err
	}
	verified, err := publication.Decode(current.Record, ed25519.PublicKey(current.Credential.AuthorityPublic[:]), owner.endpoint.network, now)
	if err != nil || verified.Credential != current.Credential || verified.Digest != current.Digest {
		return nil, errors.New("text Service publication unavailable")
	}
	if facts.Network != owner.endpoint.network || facts.Target != verified.Credential.Target ||
		facts.InstancePublic != verified.Credential.InstancePublic || facts.InstanceGeneration != verified.Credential.Generation ||
		facts.PublicationDigest != verified.Digest || facts.ProfileDigest != profile.Digest ||
		facts.WorkSafetyNotAfter <= now.Unix() || facts.NoNewRecoveryAfter <= now.Unix() ||
		facts.WorkSafetyMaximum > verified.Credential.NotAfter || facts.WorkSafetyMaximum > profile.NotAfter.Unix() {
		return nil, &textIntroductionRefusal{cause: errors.New("text Service publication or authority bounds unavailable")}
	}
	if owner.surface == broker.Administration {
		if owner.endpoint.publications == nil {
			return nil, errors.New("text Service Publisher publication unavailable")
		}
		lease, err := owner.endpoint.publications.AcquireAt(job.context, now)
		if err != nil {
			return nil, errors.New("text Service Publisher publication unavailable")
		}
		published := lease.Current()
		closeErr := lease.Close()
		if closeErr != nil {
			return nil, errors.Join(errors.New("text Service Publisher publication unavailable"), closeErr)
		}
		if published.Credential != verified.Credential || published.Digest != verified.Digest {
			return nil, &textIntroductionRefusal{cause: errors.New("text Service Publisher publication differs")}
		}
	}
	if deadline, ok := job.context.Deadline(); ok && time.Unix(facts.WorkSafetyMaximum, 0).After(deadline) {
		return nil, &textIntroductionRefusal{cause: errors.New("text Service bounds exceed local job authority")}
	}
	logical, err := nativeconnection.ProtectedContext(facts)
	if err != nil {
		return nil, &textIntroductionRefusal{cause: err}
	}
	spelling, err := targetlink.Encode(targetlink.Link{Network: facts.Network, Target: facts.Target})
	if err != nil {
		return nil, &textIntroductionRefusal{cause: errors.New("text Service destination binding unavailable")}
	}
	return &textServiceBinding{owner: owner, job: job, credential: verified.Credential, facts: facts, logical: logical,
		candidateView: profile.StateDigest, destinationBinding: sha256.Sum256([]byte(spelling))}, nil
}

func (owner *textContext) liveTextServiceJobLocked(job *textJobIdentity, surface broker.Surface) bool {
	return owner.liveLocked(owner.endpoint, surface) && job != nil && job.owner == owner && owner.job == job &&
		owner.verifiedJob == job && !job.retired && job.bound && job.workerGrant != nil && job.context.Err() == nil
}

func (binding *textServiceBinding) current() error {
	if binding == nil || binding.owner == nil {
		return errors.New("text Service binding unavailable")
	}
	owner := binding.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.liveTextServiceJobLocked(binding.job, owner.surface) {
		return errors.New("text Service job retired")
	}
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil || profile.Digest != binding.facts.ProfileDigest || profile.StateDigest != binding.candidateView ||
		!now.Before(time.Unix(binding.facts.WorkSafetyNotAfter, 0)) ||
		!now.Before(time.Unix(binding.credential.NotAfter, 0)) {
		return errors.New("text Service binding expired or changed")
	}
	return nil
}

func (binding *textServiceBinding) matchesPublication(current publication.Current) bool {
	return binding != nil && current.Credential == binding.credential && current.Digest == binding.facts.PublicationDigest &&
		len(current.Record) != 0 && sha256.Sum256(current.Record) == current.Digest
}

// servesJob reports the exact immutable Context and job ownership.
func (binding *textServiceBinding) servesJob(owner *textContext, job *textJobIdentity) bool {
	return binding != nil && binding.owner == owner && binding.job == job
}

// servesOwnerJob reports the Context ownership of a present job identity.
func (binding *textServiceBinding) servesOwnerJob(owner *textContext) bool {
	return binding != nil && binding.owner == owner && binding.job != nil
}

// jobIdentity returns the immutable job bound at construction.
func (binding *textServiceBinding) jobIdentity() *textJobIdentity {
	return binding.job
}

// connectionNonce returns the immutable per-Connection nonce commitment.
func (binding *textServiceBinding) connectionNonce() [32]byte {
	return binding.facts.ConnectionNonce
}

// target returns the immutable publication Target.
func (binding *textServiceBinding) target() [32]byte {
	return binding.facts.Target
}

// profileDigest returns the immutable permission profile digest.
func (binding *textServiceBinding) profileDigest() [32]byte {
	return binding.facts.ProfileDigest
}

// publicationDigest returns the immutable publication digest.
func (binding *textServiceBinding) publicationDigest() [32]byte {
	return binding.facts.PublicationDigest
}

// workSafetyNotAfter returns the immutable work-safety deadline in Unix
// seconds.
func (binding *textServiceBinding) workSafetyNotAfter() int64 {
	return binding.facts.WorkSafetyNotAfter
}

// protectedFacts copies the full immutable shared authority tuple.
func (binding *textServiceBinding) protectedFacts() nativeconnection.ProtectedContextInput {
	return binding.facts
}

// sameAuthorityAs compares every immutable authority fact of two bindings.
func (binding *textServiceBinding) sameAuthorityAs(other *textServiceBinding) bool {
	return binding != nil && other != nil && binding.logical == other.logical &&
		binding.facts == other.facts && binding.credential == other.credential &&
		binding.candidateView == other.candidateView
}

// dispatchRecoveryLocked returns the recovery slot only for the binding that
// serves the exact Context and job.
func (binding *textServiceBinding) dispatchRecoveryLocked(owner *textContext, job *textJobIdentity) *textIntroductionRecoveryOwner {
	if !binding.servesJob(owner, job) {
		return nil
	}
	return binding.recovery
}

// ownsRecoveryLocked reports whether this binding retains exactly that
// recovery owner.
func (binding *textServiceBinding) ownsRecoveryLocked(recovery *textIntroductionRecoveryOwner) bool {
	return binding != nil && recovery != nil && binding.recovery == recovery
}

// hasRecoveryLocked reports an occupied recovery slot.
func (binding *textServiceBinding) hasRecoveryLocked() bool {
	return binding != nil && binding.recovery != nil
}

// claimRecoveryLocked creates the one recovery owner slot of this binding.
// The shared Context lock makes creation and dispatcher registration one
// transition.
func (binding *textServiceBinding) claimRecoveryLocked() *textIntroductionRecoveryOwner {
	if binding == nil || binding.recovery != nil {
		return nil
	}
	recovery := &textIntroductionRecoveryOwner{binding: binding, generation: 2,
		delivery: make(chan textIntroductionRoutedDelivery, 1)}
	binding.recovery = recovery
	return recovery
}

// bindIntroductionLocked records the verified Descriptor recipient for later
// capsule issuance.
func (binding *textServiceBinding) bindIntroductionLocked(recipient reachability.PrivateIntroduction) {
	binding.introduction = recipient
}

// introductionLocked returns the current Descriptor recipient facts.
func (binding *textServiceBinding) introductionLocked() reachability.PrivateIntroduction {
	return binding.introduction
}

func (binding *textServiceBinding) textServiceRecovery() nativeconnection.Recovery {
	if binding == nil || binding.owner == nil || binding.job == nil {
		return nativeconnection.Recovery{}
	}
	return nativeconnection.Recovery{NetworkID: binding.facts.Network, CandidateView: binding.candidateView,
		IsolationContext: binding.job.nonce, DestinationBinding: binding.destinationBinding, RouteProfile: nativeconnection.Profile,
		WorkSafetyNotAfter: binding.facts.WorkSafetyNotAfter, WorkSafetyMaximum: binding.facts.WorkSafetyMaximum,
		NoNewRecoveryAfter: binding.facts.NoNewRecoveryAfter}
}

func (binding *textServiceBinding) validateTextServiceRecovery(request nativeconnection.Recovery) error {
	expected := binding.textServiceRecovery()
	role := "client"
	if binding != nil && binding.owner != nil && binding.owner.surface == broker.Administration {
		role = "publisher"
	}
	if request.Generation <= 1 || request.Deadline.IsZero() || request.NetworkID != expected.NetworkID ||
		request.CandidateView != expected.CandidateView || request.IsolationContext != expected.IsolationContext ||
		request.DestinationBinding != expected.DestinationBinding || request.RouteProfile != expected.RouteProfile ||
		request.Role != role ||
		request.WorkSafetyNotAfter != expected.WorkSafetyNotAfter || request.WorkSafetyMaximum != expected.WorkSafetyMaximum ||
		request.NoNewRecoveryAfter != expected.NoNewRecoveryAfter {
		return errors.New("text Service recovery changed immutable authority")
	}
	return binding.current()
}

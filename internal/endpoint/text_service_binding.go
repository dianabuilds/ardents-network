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
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// textServiceBinding retains the exact job and independently verified
// publication. It cannot be reconstructed by a local Application from a
// supplied "established" flag or copied wire nonce.
type textServiceBinding struct {
	owner      *textContext
	job        *textJobIdentity
	credential publication.Credential
	facts      nativeconnection.ProtectedContextInput
	logical    [32]byte
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
	local, err := nativeconnection.Context(nativeconnection.ContextInput{
		Network: destination.Network, Target: destination.Target, InstancePublic: current.Credential.InstancePublic,
		InstanceGeneration: current.Credential.Generation, PublicationDigest: current.Digest,
		CandidateView: profile.StateDigest, IsolationContext: job.nonce, DestinationBinding: sha256.Sum256([]byte(spelling)),
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
	if deadline, ok := job.context.Deadline(); ok && time.Unix(facts.WorkSafetyMaximum, 0).After(deadline) {
		return nil, &textIntroductionRefusal{cause: errors.New("text Service bounds exceed local job authority")}
	}
	logical, err := nativeconnection.ProtectedContext(facts)
	if err != nil {
		return nil, &textIntroductionRefusal{cause: err}
	}
	return &textServiceBinding{owner: owner, job: job, credential: verified.Credential, facts: facts, logical: logical}, nil
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
	if err != nil || profile.Digest != binding.facts.ProfileDigest || !now.Before(time.Unix(binding.facts.WorkSafetyNotAfter, 0)) ||
		!now.Before(time.Unix(binding.credential.NotAfter, 0)) {
		return errors.New("text Service binding expired or changed")
	}
	return nil
}

func (binding *textServiceBinding) matchesPublication(current publication.Current) bool {
	return binding != nil && current.Credential == binding.credential && current.Digest == binding.facts.PublicationDigest &&
		len(current.Record) != 0 && sha256.Sum256(current.Record) == current.Digest
}

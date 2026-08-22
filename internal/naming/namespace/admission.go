package namespace

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"runtime"
)

// NewAdmission creates one boot-scoped gate only for the accepted O1b profile.
func NewAdmission(node, network [32]byte, epoch uint64, bootSecret [32]byte) (*Admission, error) {
	if node == [32]byte{} || network == [32]byte{} || epoch == 0 || bootSecret == [32]byte{} {
		return nil, errors.New("naming admission configuration is not the accepted profile")
	}
	copyProfiles := acceptedProfiles()
	surfaces := make(map[string]*admissionSurface, len(copyProfiles))
	for _, profile := range copyProfiles {
		surfaces[profile.Surface] = &admissionSurface{spent: make(map[[32]byte]int64, profile.MaximumSpent),
			inflight: make(chan struct{}, profile.MaximumInFlight)}
	}
	return &Admission{node: node, network: network, epoch: epoch, bootSecret: bootSecret,
		profiles: copyProfiles, surfaces: surfaces}, nil
}

// Issue authenticates one fresh bounded challenge. now and expiry are fixed
// Node-clock milliseconds; the verifier owns their expiry interpretation.
func (admission *Admission) Issue(now int64, surface string, operation, isolation [32]byte,
	expires int64, nonce [16]byte,
) (Challenge, error) {
	profile, ok := admission.profile(surface)
	if !ok || now < 0 || operation == [32]byte{} || isolation == [32]byte{} || nonce == [16]byte{} ||
		expires <= now || expires-now > 30_000 {
		return Challenge{}, errors.New("naming admission request is invalid")
	}
	state := admission.surfaces[surface]
	state.mu.Lock()
	if state.nextExpiry != 0 && state.nextExpiry <= now {
		admission.removeExpired(state, now)
	}
	full := len(state.spent) >= profile.MaximumSpent
	state.mu.Unlock()
	if full {
		return Challenge{}, errors.New("naming admission capacity is full")
	}
	challenge := Challenge{Node: admission.node, Network: admission.network, Epoch: admission.epoch,
		Surface: surface, OperationDigest: operation,
		IsolationBinding: isolationBinding(admission.node, operation, isolation, nonce),
		IssuedAt:         now, ExpiresAt: expires, Nonce: nonce, WorkBits: profile.WorkBits}
	challenge.AuthenticationTag = admission.authenticate(challenge)
	return challenge, nil
}

// Verify consumes one valid proof exactly once and rejects full capacity or
// in-flight state immediately without eviction or queueing.
func (admission *Admission) Verify(now int64, proof Proof) (bool, string) {
	state, ok := admission.surfaces[proof.Challenge.Surface]
	if !ok {
		return false, "invalid-scope"
	}
	select {
	case state.inflight <- struct{}{}:
		defer func() { <-state.inflight }()
	default:
		return false, "busy"
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	challenge := proof.Challenge
	profile, ok := admission.profile(challenge.Surface)
	if !ok || challenge.Node != admission.node || challenge.Network != admission.network ||
		challenge.Epoch != admission.epoch || challenge.WorkBits != profile.WorkBits || challenge.IssuedAt < 0 ||
		now < challenge.IssuedAt || now >= challenge.ExpiresAt || challenge.ExpiresAt-challenge.IssuedAt > 30_000 ||
		challenge.OperationDigest == [32]byte{} || challenge.IsolationBinding == [32]byte{} || challenge.Nonce == [16]byte{} {
		return false, "invalid-scope"
	}
	tag := admission.authenticate(challenge)
	if subtle.ConstantTimeCompare(tag[:], challenge.AuthenticationTag[:]) != 1 {
		return false, "invalid-challenge"
	}
	if state.nextExpiry != 0 && state.nextExpiry <= now {
		admission.removeExpired(state, now)
	}
	digest := challengeDigest(challenge)
	if _, exists := state.spent[digest]; exists {
		return false, "replay"
	}
	if len(state.spent) >= profile.MaximumSpent {
		return false, "capacity"
	}
	if !validWork(challenge, proof.WorkNonce) {
		return false, "insufficient-work"
	}
	state.spent[digest] = challenge.ExpiresAt
	if state.nextExpiry == 0 || challenge.ExpiresAt < state.nextExpiry {
		state.nextExpiry = challenge.ExpiresAt
	}
	return true, ""
}

func (admission *Admission) removeExpired(state *admissionSurface, now int64) {
	next, removed := int64(0), 0
	for digest, expiry := range state.spent {
		if expiry <= now {
			delete(state.spent, digest)
			removed++
			if removed%64 == 0 {
				runtime.Gosched()
			}
		} else if next == 0 || expiry < next {
			next = expiry
		}
	}
	state.nextExpiry = next
}

func (admission *Admission) authenticate(challenge Challenge) [32]byte {
	mac := hmac.New(sha256.New, admission.bootSecret[:])
	_, _ = mac.Write(challengeBytes(challenge, false))
	var result [32]byte
	copy(result[:], mac.Sum(nil))
	return result
}

func (admission *Admission) profile(surface string) (profile, bool) {
	for _, profile := range admission.profiles {
		if profile.Surface == surface {
			return profile, true
		}
	}
	return profile{}, false
}

func acceptedProfiles() []profile {
	return []profile{
		{"resolution", 16, 4096, 64},
		{"renewal-update", 16, 2048, 32},
		{"policy-recovery", 17, 1024, 16},
		{"root-claim", 18, 1024, 8},
	}
}

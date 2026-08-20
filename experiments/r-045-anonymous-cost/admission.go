//go:build ignore

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"sync"
)

type Surface string

const (
	Resolution Surface = "resolution"
	Update     Surface = "renewal-update"
	Recovery   Surface = "policy-recovery"
	Claim      Surface = "root-claim"
)

type Outcome string

const (
	Admitted        Outcome = "admitted"
	AdmissionDenied Outcome = "admission-denied"
)

type Profile struct {
	Surface     Surface
	WorkBits    uint8
	MaxSpent    int
	MaxInFlight int
}

type Config struct {
	Node         [32]byte
	Network      [32]byte
	Epoch        uint64
	BootSecret   [32]byte
	MaxTTLMillis int64
	Profiles     []Profile
}

type Request struct {
	Surface          Surface
	OperationDigest  [32]byte
	IsolationContext [32]byte
	ExpiresAt        int64
	Nonce            [16]byte
}

type Challenge struct {
	Node              [32]byte
	Network           [32]byte
	Epoch             uint64
	Surface           Surface
	OperationDigest   [32]byte
	IsolationContext  [32]byte
	IssuedAt          int64
	ExpiresAt         int64
	Nonce             [16]byte
	WorkBits          uint8
	AuthenticationTag [32]byte
}

type Proof struct {
	Challenge Challenge
	WorkNonce uint64
}

type Result struct {
	Outcome Outcome
	Reason  string
}

type Admission struct {
	node         [32]byte
	network      [32]byte
	epoch        uint64
	bootSecret   [32]byte
	maxTTLMillis int64
	profiles     []Profile
	mu           sync.Mutex
	spent        map[Surface]map[[32]byte]int64
	inflight     map[Surface]chan struct{}
}

func NewAdmission(config Config) (*Admission, error) {
	if config.Node == [32]byte{} || config.Network == [32]byte{} || config.Epoch == 0 ||
		config.BootSecret == [32]byte{} ||
		config.MaxTTLMillis < 1 || config.MaxTTLMillis > 60_000 ||
		len(config.Profiles) == 0 || len(config.Profiles) > 4 {
		return nil, errors.New("admission configuration is invalid")
	}
	profiles := append([]Profile(nil), config.Profiles...)
	for i, profile := range profiles {
		if !validSurface(profile.Surface) || profile.WorkBits == 0 || profile.WorkBits > 30 ||
			profile.MaxSpent < 1 || profile.MaxSpent > 65_536 ||
			profile.MaxInFlight < 1 || profile.MaxInFlight > 1_024 {
			return nil, errors.New("admission profile is invalid")
		}
		for j := 0; j < i; j++ {
			if profiles[j].Surface == profile.Surface {
				return nil, errors.New("admission surface is duplicated")
			}
		}
	}
	spent := make(map[Surface]map[[32]byte]int64, len(profiles))
	inflight := make(map[Surface]chan struct{}, len(profiles))
	for _, profile := range profiles {
		spent[profile.Surface] = make(map[[32]byte]int64, profile.MaxSpent)
		inflight[profile.Surface] = make(chan struct{}, profile.MaxInFlight)
	}
	return &Admission{node: config.Node, network: config.Network, epoch: config.Epoch,
		bootSecret:   config.BootSecret,
		maxTTLMillis: config.MaxTTLMillis, profiles: profiles,
		spent: spent, inflight: inflight}, nil
}

func (admission *Admission) Issue(now int64, request Request) (Challenge, error) {
	bits, ok := admission.workBits(request.Surface)
	if !ok || now < 0 || request.OperationDigest == [32]byte{} || request.IsolationContext == [32]byte{} ||
		request.Nonce == [16]byte{} || request.ExpiresAt <= now || request.ExpiresAt-now > admission.maxTTLMillis {
		return Challenge{}, errors.New("admission request is invalid")
	}
	challenge := Challenge{Node: admission.node, Network: admission.network, Epoch: admission.epoch,
		Surface: request.Surface, OperationDigest: request.OperationDigest,
		IsolationContext: request.IsolationContext, IssuedAt: now, ExpiresAt: request.ExpiresAt,
		Nonce: request.Nonce, WorkBits: bits}
	challenge.AuthenticationTag = admission.authenticate(challenge)
	return challenge, nil
}

func (admission *Admission) Verify(now int64, proof Proof) Result {
	limiter, ok := admission.inflight[proof.Challenge.Surface]
	if !ok {
		return Result{Outcome: AdmissionDenied, Reason: "invalid-scope"}
	}
	select {
	case limiter <- struct{}{}:
		defer func() { <-limiter }()
	default:
		return Result{Outcome: AdmissionDenied, Reason: "busy"}
	}
	admission.mu.Lock()
	defer admission.mu.Unlock()
	challenge := proof.Challenge
	bits, ok := admission.workBits(challenge.Surface)
	if !ok || challenge.Node != admission.node || challenge.Network != admission.network ||
		challenge.Epoch != admission.epoch || challenge.WorkBits != bits || challenge.IssuedAt < 0 ||
		now < challenge.IssuedAt || now >= challenge.ExpiresAt ||
		challenge.ExpiresAt-challenge.IssuedAt > admission.maxTTLMillis ||
		challenge.OperationDigest == [32]byte{} || challenge.IsolationContext == [32]byte{} ||
		challenge.Nonce == [16]byte{} {
		return Result{Outcome: AdmissionDenied, Reason: "invalid-scope"}
	}
	wantTag := admission.authenticate(challenge)
	if subtle.ConstantTimeCompare(wantTag[:], challenge.AuthenticationTag[:]) != 1 {
		return Result{Outcome: AdmissionDenied, Reason: "invalid-challenge"}
	}
	spent := admission.spent[challenge.Surface]
	for digest, expiry := range spent {
		if expiry <= now {
			delete(spent, digest)
		}
	}
	digest := challengeDigest(challenge)
	if _, replay := spent[digest]; replay {
		return Result{Outcome: AdmissionDenied, Reason: "replay"}
	}
	if len(spent) >= admission.maxSpent(challenge.Surface) {
		return Result{Outcome: AdmissionDenied, Reason: "capacity"}
	}
	if !validWork(challenge, proof.WorkNonce) {
		return Result{Outcome: AdmissionDenied, Reason: "insufficient-work"}
	}
	spent[digest] = challenge.ExpiresAt
	return Result{Outcome: Admitted}
}

func Solve(challenge Challenge) (Proof, uint64) {
	for nonce := uint64(0); ; nonce++ {
		if validWork(challenge, nonce) {
			return Proof{Challenge: challenge, WorkNonce: nonce}, nonce + 1
		}
	}
}

func (admission *Admission) authenticate(challenge Challenge) [32]byte {
	mac := hmac.New(sha256.New, admission.bootSecret[:])
	_, _ = mac.Write(challengeBytes(challenge, false))
	var result [32]byte
	copy(result[:], mac.Sum(nil))
	return result
}

func challengeDigest(challenge Challenge) [32]byte {
	return sha256.Sum256(challengeBytes(challenge, true))
}

func (admission *Admission) workBits(surface Surface) (uint8, bool) {
	for _, profile := range admission.profiles {
		if profile.Surface == surface {
			return profile.WorkBits, true
		}
	}
	return 0, false
}

func (admission *Admission) maxSpent(surface Surface) int {
	for _, profile := range admission.profiles {
		if profile.Surface == surface {
			return profile.MaxSpent
		}
	}
	return 0
}

func validSurface(surface Surface) bool {
	return surface == Resolution || surface == Update || surface == Recovery || surface == Claim
}

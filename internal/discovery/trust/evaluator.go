// Package trust owns discovery publication trust evaluation. It does not own
// identity authentication or policy enforcement.
package trust

import (
	"bytes"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	discoveryrecord "ardents/internal/discovery/records"
	identitytrust "ardents/internal/identity/trust"
)

type Result struct {
	Outcome string
	Valid   bool
	Trusted bool
	Usable  bool
	Reason  string
}

type cachedVerification struct {
	valid  bool
	reason string
}

const maxVerificationCacheEntries = 1024

type Evaluator struct {
	mu                sync.Mutex
	state             string
	last              Result
	registry          *identitytrust.Registry
	verified          map[string]cachedVerification
	verifiedOrder     []string
	now               func() time.Time
	verificationCount uint64
}

func NewEvaluator(configured *identitytrust.Registry) *Evaluator {
	registry := emptyRegistry()
	if configured != nil {
		registry = configured
	}
	return &Evaluator{
		state:    "new",
		registry: registry,
		verified: make(map[string]cachedVerification),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func emptyRegistry() *identitytrust.Registry {
	registry, err := identitytrust.NewRegistry(nil)
	if err != nil {
		panic(err)
	}
	return registry
}

// ReplaceRegistry atomically changes the complete trust view. Signature cache
// entries remain valid; trust is recomputed against the new generation.
func (s *Evaluator) ReplaceRegistry(registry *identitytrust.Registry) {
	if registry == nil {
		registry = emptyRegistry()
	}
	s.mu.Lock()
	s.registry = registry
	s.mu.Unlock()
}

func (s *Evaluator) Evaluate(record discoveryrecord.Record) Result {
	result, _ := s.EvaluateWithEvidence(record)
	return result
}

// EvaluateWithEvidence verifies immutable record authority at most once per
// signed record in this process. Freshness and current purpose-scoped trust are
// evaluated on every call and are never cached.
func (s *Evaluator) EvaluateWithEvidence(record discoveryrecord.Record) (Result, discoveryrecord.VerificationEvidence) {
	return s.evaluateAtWithEvidence(record, s.now(), false)
}

func (s *Evaluator) EvaluateAtWithEvidence(record discoveryrecord.Record, now time.Time) (Result, discoveryrecord.VerificationEvidence) {
	return s.evaluateAtWithEvidence(record, now, false)
}

// VerifyRetained rechecks a persisted record's signature even if the same
// fingerprint already exists in memory. Persisted evidence is never an
// authority shortcut against database tampering.
func (s *Evaluator) VerifyRetained(record discoveryrecord.Record) (discoveryrecord.VerificationEvidence, error) {
	result, evidence := s.evaluateAtWithEvidence(record, s.now(), true)
	if evidence.Version == 0 {
		return discoveryrecord.VerificationEvidence{}, errors.New(result.Reason)
	}
	return evidence, nil
}

func (s *Evaluator) evaluateAtWithEvidence(record discoveryrecord.Record, now time.Time, force bool) (Result, discoveryrecord.VerificationEvidence) {
	canonicalDigest, signatureDigest, signer, err := discoveryrecord.Fingerprint(record)
	if err != nil {
		return unverified(err.Error()), discoveryrecord.VerificationEvidence{}
	}
	cacheKey := canonicalDigest + "." + signatureDigest

	s.mu.Lock()
	defer s.mu.Unlock()
	verification, ok := s.verified[cacheKey]
	if force {
		ok = false
	}
	if !ok {
		s.verificationCount++
		err := discoveryrecord.ValidateRetained(record)
		verification = cachedVerification{
			valid: err == nil,
		}
		if err != nil {
			verification.reason = err.Error()
		}
		s.rememberVerificationLocked(cacheKey, verification)
	}
	if !verification.valid {
		return unverified(verification.reason), discoveryrecord.VerificationEvidence{}
	}

	trusted := false
	if expected, ok := s.registry.Lookup(identitytrust.PurposeDiscoveryPublish, signer); ok {
		actual, decodeErr := base64.StdEncoding.DecodeString(record.PublicKeyText())
		trusted = decodeErr == nil && bytes.Equal(expected, actual)
	}
	evidence := discoveryrecord.VerificationEvidence{
		Version:         discoveryrecord.EvidenceVersion,
		CanonicalDigest: canonicalDigest,
		SignatureDigest: signatureDigest,
		Signer:          signer,
		TrustGeneration: s.registry.Generation().String(),
		Trusted:         trusted,
	}
	return evaluateFreshness(record, now, trusted), evidence
}

func (s *Evaluator) rememberVerificationLocked(key string, verification cachedVerification) {
	if _, exists := s.verified[key]; exists {
		s.verified[key] = verification
		return
	}
	if len(s.verifiedOrder) == maxVerificationCacheEntries {
		delete(s.verified, s.verifiedOrder[0])
		copy(s.verifiedOrder, s.verifiedOrder[1:])
		s.verifiedOrder = s.verifiedOrder[:len(s.verifiedOrder)-1]
	}
	s.verified[key] = verification
	s.verifiedOrder = append(s.verifiedOrder, key)
}

func evaluateFreshness(record discoveryrecord.Record, now time.Time, trusted bool) Result {
	if now.Before(record.IssuedAt) {
		return Result{Outcome: "not_yet_valid", Reason: "record is not yet valid"}
	}
	if !record.ExpiresAt.After(now) {
		return Result{Outcome: "expired", Reason: "record expired"}
	}
	result := Result{Valid: true, Outcome: "verified", Reason: "record verified"}
	if trusted {
		result.Trusted = true
		result.Usable = true
		result.Outcome = "usable"
		result.Reason = "record verified and trusted"
	}
	return result
}

func unverified(reason string) Result {
	return Result{Outcome: "unverified", Reason: reason}
}

func (s *Evaluator) Remember(result Result) {
	s.mu.Lock()
	s.last = result
	s.state = stateForResult(result)
	s.mu.Unlock()
}

func (s *Evaluator) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Evaluator) Last() Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

func stateForResult(result Result) string {
	if result.Valid {
		return "ready"
	}
	if result.Outcome == "" && result.Reason == "" {
		return "new"
	}
	return "degraded"
}

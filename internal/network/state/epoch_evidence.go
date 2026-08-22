package state

import (
	"errors"
	"fmt"
)

// VerifyEvidence authenticates every retained generation and fully verifies
// the current Epoch decision through the canonical Epoch implementation.
func verifyEpochEvidence(policy epochPolicy, current string, generations map[string][]byte, inputs, materializations [][]byte) (verifiedEpochDecision, error) {
	indexed, err := indexEvidence(current, generations, inputs, materializations)
	if err != nil {
		return verifiedEpochDecision{}, err
	}
	tip, previous, seen, err := authenticateEvidenceChain(policy, indexed, current)
	if err != nil {
		return verifiedEpochDecision{}, err
	}
	if len(seen) != len(indexed) {
		return verifiedEpochDecision{}, errors.New("generation evidence contains an unrelated member")
	}
	policy.Previous = previous
	decision, err := verifyEpochDecision(policy, indexed[current], inputs, materializations, true)
	if err != nil {
		return verifiedEpochDecision{}, err
	}
	if decision.Snapshot.Digest != tip.digest || decision.Snapshot.Generation != current {
		return verifiedEpochDecision{}, errors.New("current generation disagrees with authenticated evidence")
	}
	return decision, nil
}

func indexEvidence(current string, generations map[string][]byte, inputs, materializations [][]byte) (map[string][]byte, error) {
	if !canonicalGeneration(current) || len(generations) == 0 ||
		len(generations) > maximumEpochChain || len(inputs) > 64 || len(materializations) > 64 {
		return nil, errors.New("epoch evidence exceeds its canonical finite shape")
	}
	indexed := make(map[string][]byte, len(generations))
	for name, raw := range generations {
		if !canonicalGeneration(name) || len(raw) == 0 || len(raw) > maximumEpochBytes {
			return nil, errors.New("epoch generation evidence is invalid")
		}
		indexed[name] = raw
	}
	return indexed, nil
}

func authenticateEvidenceChain(policy epochPolicy, generations map[string][]byte, current string) (epochEnvelope, *epochVerificationSnapshot, map[string]bool, error) {
	seen := make(map[string]bool)
	var load func(string, bool) (epochEnvelope, *epochVerificationSnapshot, error)
	load = func(name string, tip bool) (epochEnvelope, *epochVerificationSnapshot, error) {
		if !canonicalGeneration(name) || seen[name] || len(seen) >= maximumEpochChain {
			return epochEnvelope{}, nil, errors.New("generation chain is cyclic or exceeds its bound")
		}
		raw, exists := generations[name]
		if !exists {
			return epochEnvelope{}, nil, errors.New("generation chain member is missing")
		}
		seen[name] = true
		parsed, err := parseEpoch(raw)
		if err != nil || parsed.digestString() != name {
			return epochEnvelope{}, nil, errors.Join(errors.New("generation chain member is invalid"), err)
		}
		checkAt := parsed.validFrom
		if tip {
			checkAt = policy.Now
		}
		if err := authenticateEnvelope(policy, parsed, checkAt); err != nil {
			return epochEnvelope{}, nil, err
		}
		if parsed.number == 1 {
			if parsed.previous != [32]byte{} {
				return epochEnvelope{}, nil, errors.New("genesis previous digest is not zero")
			}
			return parsed, nil, nil
		}
		prior, _, err := load(fmt.Sprintf("%x", parsed.previous), false)
		if err != nil || prior.number+1 != parsed.number || prior.digest != parsed.previous {
			return epochEnvelope{}, nil, errors.Join(errors.New("epoch transition is invalid"), err)
		}
		previous := snapshotFor(prior)
		return parsed, &previous, nil
	}
	tip, previous, err := load(current, true)
	return tip, previous, seen, err
}

func canonicalGeneration(name string) bool {
	if len(name) != 64 {
		return false
	}
	for _, current := range []byte(name) {
		if current < '0' || current > '9' {
			if current < 'a' || current > 'f' {
				return false
			}
		}
	}
	return true
}

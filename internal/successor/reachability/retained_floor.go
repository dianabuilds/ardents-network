package reachability

import (
	"errors"
	"time"
)

// Outcome describes a proof/floor transition; only durable Store completion
// permits an accepting publication acknowledgement.
type Outcome string

const (
	Accepted       Outcome = "accepted"
	AlreadyCurrent Outcome = "already-current"
	Stale          Outcome = "stale"
	Conflicting    Outcome = "conflicting"
	Invalid        Outcome = "invalid"
)

type retainedFloor struct {
	descriptor                            Descriptor
	publicationConflict, revisionConflict bool
}

// historyFloor carries monotonic facts only. Local history cannot retain proof
// bytes for offline use; receiving storage separately retains the signed record.
type historyFloor struct {
	target, publication, descriptor       [32]byte
	generation, revision                  uint64
	notBefore, notAfter                   time.Time
	publicationConflict, revisionConflict bool
}

func descriptorFloor(proof Descriptor) historyFloor {
	credential := proof.Publication.Delegation()
	return historyFloor{target: proof.Target, publication: proof.PublicationDigest,
		descriptor: proof.Digest(), generation: credential.Generation,
		revision: proof.Introduction.Revision, notBefore: credential.NotBefore,
		notAfter: credential.NotAfter}
}

// compareFloor proposes a replacement without mutating either retained value.
// Its caller commits before publishing the replacement, including conflicts.
func compareFloor(prior, candidate retainedFloor) (Outcome, *retainedFloor, error) {
	old, next := descriptorFloor(prior.descriptor), descriptorFloor(candidate.descriptor)
	old.publicationConflict, old.revisionConflict = prior.publicationConflict, prior.revisionConflict
	outcome, replacement, err := compareHistoryFloor(old, next)
	if replacement == nil {
		return outcome, nil, err
	}
	if replacement.descriptor == next.descriptor {
		prior = candidate
	}
	prior.publicationConflict, prior.revisionConflict = replacement.publicationConflict, replacement.revisionConflict
	return outcome, &prior, err
}

// compareHistoryFloor is the one shared transition rule. Its owner commits the
// proposal at its own lifetime/durability boundary before exposing the result.
func compareHistoryFloor(prior, candidate historyFloor) (Outcome, *historyFloor, error) {
	if prior.target != candidate.target {
		return Invalid, nil, errors.New("reachability compared different Targets")
	}
	if candidate.generation < prior.generation {
		return Stale, nil, errors.New("reachability generation stale")
	}
	if candidate.generation > prior.generation {
		if candidate.notBefore.Before(prior.notAfter) {
			return Invalid, nil, errors.New("reachability generations overlap")
		}
		return Accepted, &candidate, nil
	}
	if prior.publicationConflict {
		if candidate.notAfter.After(prior.notAfter) {
			candidate.publicationConflict = true
			return Stale, &candidate, errors.New("reachability publication remains conflicting")
		}
		return Stale, nil, errors.New("reachability publication remains conflicting")
	}
	if prior.publication != candidate.publication {
		if candidate.notAfter.After(prior.notAfter) {
			prior = candidate
		}
		prior.publicationConflict = true
		return Conflicting, &prior, errors.New("reachability publication conflicts")
	}
	if candidate.revision < prior.revision {
		return Stale, nil, errors.New("reachability revision stale")
	}
	if candidate.revision > prior.revision {
		return Accepted, &candidate, nil
	}
	if prior.revisionConflict {
		return Conflicting, nil, errors.New("reachability revision remains conflicting")
	}
	if prior.descriptor == candidate.descriptor {
		return AlreadyCurrent, nil, nil
	}
	prior.revisionConflict = true
	return Conflicting, &prior, errors.New("reachability revision conflicts")
}

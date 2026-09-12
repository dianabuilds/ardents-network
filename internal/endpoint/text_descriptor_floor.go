//go:build linux

package endpoint

import (
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// Each authorized context retains at most the receiving Store's 128 Targets.
// Expired entries retain their floors until context retirement; capacity never
// evicts an older floor or silently moves private history to another context.
const maximumTextDescriptorTargets = 128

type textDescriptorFloor struct {
	generation, revision                  uint64
	publication, descriptor               [32]byte
	notAfter                              int64
	publicationConflict, revisionConflict bool
}

// Called under owner.mu after the actual resolution flight rechecks its live
// authority. Verify raw bytes here so no caller-assembled Verified can poison
// the floor. Returned proof slices have no aliases to retained cache state.
func (owner *textContext) acceptTextDescriptorLocked(raw []byte, target, network, profile [32]byte, at time.Time) (reachability.Verified, error) {
	verified, err := reachability.VerifyPrivate(raw, target, network, profile, at)
	if err != nil {
		return reachability.Verified{}, err
	}
	credential := verified.Current.Credential
	candidate := textDescriptorFloor{generation: credential.Generation, revision: verified.Descriptor.Private.Revision,
		publication: verified.Current.Digest, descriptor: sha256.Sum256(raw), notAfter: credential.NotAfter}
	prior, exists := owner.descriptorFloors[target]
	if exists {
		if candidate.generation < prior.generation {
			return reachability.Verified{}, errors.New("text Descriptor publication is stale")
		}
		if candidate.generation > prior.generation {
			if credential.NotBefore < prior.notAfter {
				return reachability.Verified{}, errors.New("text Descriptor publication overlaps retained authority")
			}
		} else {
			if candidate.publication != prior.publication {
				prior.publicationConflict = true
				// A conflicting branch cannot shorten the terminal authority interval
				// against which every future Instance generation must be checked.
				if candidate.notAfter > prior.notAfter {
					prior.notAfter = candidate.notAfter
				}
				owner.descriptorFloors[target] = prior
			}
			if prior.publicationConflict {
				return reachability.Verified{}, errors.New("text Descriptor publication remains conflicting")
			}
			if candidate.revision < prior.revision {
				return reachability.Verified{}, errors.New("text Descriptor revision is stale")
			}
			if candidate.revision == prior.revision {
				if candidate.descriptor != prior.descriptor {
					prior.revisionConflict = true
					owner.descriptorFloors[target] = prior
				}
				if prior.revisionConflict {
					return reachability.Verified{}, errors.New("text Descriptor revision remains conflicting")
				}
			}
		}
	} else if len(owner.descriptorFloors) >= maximumTextDescriptorTargets {
		return reachability.Verified{}, errors.New("text Descriptor context capacity exhausted")
	}
	if owner.descriptorFloors == nil {
		owner.descriptorFloors = make(map[[32]byte]textDescriptorFloor)
	}
	owner.descriptorFloors[target] = candidate
	return verified, nil
}

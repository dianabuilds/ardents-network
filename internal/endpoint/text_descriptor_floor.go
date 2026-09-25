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

// textDescriptorHistory owns one context's private publication and revision
// floors. The Context lock protects every method and retirement clears it.
type textDescriptorHistory struct {
	floors map[[32]byte]textDescriptorFloor
}

func (history *textDescriptorHistory) canAdmit(target [32]byte) bool {
	_, retained := history.floors[target]
	return retained || len(history.floors) < maximumTextDescriptorTargets
}

func (history *textDescriptorHistory) matches(target, publication [32]byte, revision uint64) bool {
	floor, retained := history.floors[target]
	return retained && !floor.publicationConflict && !floor.revisionConflict &&
		floor.publication == publication && floor.revision == revision
}

func (history *textDescriptorHistory) clear() {
	clear(history.floors)
	history.floors = nil
}

// Called under the Context lock after the actual resolution flight rechecks its live
// authority. Verify raw bytes here so no caller-assembled Verified can poison
// the floor. Returned proof slices have no aliases to retained cache state.
func (history *textDescriptorHistory) accept(raw []byte, target, network, profile [32]byte, at time.Time) (reachability.Verified, error) {
	verified, err := reachability.VerifyPrivate(raw, target, network, profile, at)
	if err != nil {
		return reachability.Verified{}, err
	}
	credential := verified.Current.Credential
	candidate := textDescriptorFloor{generation: credential.Generation, revision: verified.Descriptor.Private.Revision,
		publication: verified.Current.Digest, descriptor: sha256.Sum256(raw), notAfter: credential.NotAfter}
	prior, exists := history.floors[target]
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
				history.floors[target] = prior
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
					history.floors[target] = prior
				}
				if prior.revisionConflict {
					return reachability.Verified{}, errors.New("text Descriptor revision remains conflicting")
				}
			}
		}
	} else if len(history.floors) >= maximumTextDescriptorTargets {
		return reachability.Verified{}, errors.New("text Descriptor context capacity exhausted")
	}
	if history.floors == nil {
		history.floors = make(map[[32]byte]textDescriptorFloor)
	}
	history.floors[target] = candidate
	return verified, nil
}

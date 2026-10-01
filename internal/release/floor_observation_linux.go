//go:build linux

package release

import "errors"

// CurrentFloors observes the exclusively held durable trust state for the
// installed Linux successor consumer. An empty result grants no target trust.
// Returned digest slices do not alias the store's owned observations.
func (verifier *Verifier) CurrentFloors() (FloorSet, error) {
	if verifier == nil || verifier.store == nil {
		return FloorSet{}, errors.New("release: verifier is nil")
	}
	floors, err := verifier.store.ReadFloors()
	if err != nil {
		return FloorSet{}, err
	}
	return cloneDecision(Decision{Floors: floors}).Floors, nil
}

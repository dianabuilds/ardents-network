package state

import "github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"

// OpenRetained is the selected native Source's already-initialized opening.
// It cannot initialize missing State/participation histories. Signed acceptance,
// conflict, time, serving and joined failure rules remain identical to Open.
func OpenRetained(input Config) (*networkState, error) {
	resolved, err := validateConfig(input)
	if err != nil {
		return nil, err
	}
	resolved.retainedHistory = true
	if resolved.permitWork != nil {
		if err = resolved.permitWork(); err != nil {
			return nil, err
		}
	}
	storage, err := durable.OpenRetained(resolved.root, storageLimits())
	if err != nil {
		return nil, err
	}
	return activateState(resolved, storage, nil, nil)
}

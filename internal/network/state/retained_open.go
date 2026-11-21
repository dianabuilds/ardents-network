package state

import "github.com/dianabuilds/ardents-network/internal/network/state/durable"

// OpenRetained opens already-initialized State history.
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

package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// nodePreparationStateConfig constructs an offline State configuration. It
// authenticates no State and opens no root; role callers validate their purpose.
func nodePreparationStateConfig(n deployment.NodeNetworkInputs, rolePaths []string, invalid, binding error) (state.Config, error) {
	if n.NetworkID == [32]byte{} || n.ProfileAuthority == [32]byte{} || n.Threshold < 1 || n.Threshold > len(n.Authorities) || len(n.Authorities) > 16 {
		return state.Config{}, invalid
	}
	roots := append([]string{n.Root, n.ClockObservationFile}, rolePaths...)
	for i, path := range roots {
		if !sourceControlPath(path) {
			return state.Config{}, invalid
		}
		for _, previous := range roots[:i] {
			if path == previous || strings.HasPrefix(path, previous+"/") || strings.HasPrefix(previous, path+"/") {
				return state.Config{}, binding
			}
		}
	}
	pins := make(map[[32]byte]ed25519.PublicKey, len(n.Authorities))
	for _, public := range n.Authorities {
		id := sha256.Sum256(public[:])
		if public == [32]byte{} || pins[id] != nil {
			return state.Config{}, binding
		}
		pins[id] = append(ed25519.PublicKey(nil), public[:]...)
	}
	return state.Config{Root: n.Root, NetworkID: n.NetworkID, Authorities: pins, Threshold: n.Threshold, ClosedProfileAuthority: append(ed25519.PublicKey(nil), n.ProfileAuthority[:]...), AcceptedProfile: "ardents-route-v3", Clock: time.Now, ClockObservationFile: n.ClockObservationFile}, nil
}

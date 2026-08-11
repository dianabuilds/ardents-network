package networkstate

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

func validateConfig(input Config) (config, error) {
	if input.Root == "" {
		return config{}, errors.New("state root is required")
	}
	root, err := filepath.Abs(input.Root)
	if err != nil {
		return config{}, fmt.Errorf("resolve state root: %w", err)
	}
	if input.Threshold < 1 || input.Threshold > len(input.Authorities) {
		return config{}, errors.New("authority threshold is outside the authority set")
	}
	if len(input.Authorities) > 16 {
		return config{}, errors.New("authority set exceeds 16 keys")
	}
	if input.Now.IsZero() && input.Clock == nil {
		return config{}, errors.New("verification time is required")
	}
	clock := input.Clock
	if clock == nil {
		fixed := input.Now.UTC()
		clock = func() time.Time { return fixed }
	}
	authorities := make(map[[32]byte]ed25519.PublicKey, len(input.Authorities))
	for id, public := range input.Authorities {
		if len(public) != ed25519.PublicKeySize {
			return config{}, errors.New("authority public key has invalid length")
		}
		if sha256.Sum256(public) != id {
			return config{}, errors.New("authority identifier does not match its public key")
		}
		authorities[id] = append(ed25519.PublicKey(nil), public...)
	}
	initial := clock().UTC()
	resolved := config{
		root: root, networkID: input.NetworkID, authorities: authorities,
		threshold: input.Threshold, now: initial, clock: clock,
		material: input.SourceMaterializationIndex, observation: input.ClockObservation.UTC(),
		orderSeed: input.SourceOrderSeed, anchorWall: initial, anchorMono: time.Now(),
	}
	if err := configureDistribution(&resolved, input); err != nil {
		return config{}, err
	}
	return resolved, nil
}

package state

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
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
	observe := input.ObserveClock
	if observe != nil && input.ClockObservationFile != "" {
		return config{}, errors.New("clock observation has multiple owners")
	}
	if input.ClockObservationFile != "" {
		observationPath, pathErr := filepath.Abs(input.ClockObservationFile)
		if pathErr != nil {
			return config{}, errors.New("resolve clock observation file")
		}
		observe = fileClockObserver(observationPath)
	}
	if observe == nil {
		fixed := input.ClockObservation.UTC()
		observe = func() time.Time { return fixed }
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
	sourcePlan, sourceInfo, err := source.New(input.Source, authorities)
	if err != nil {
		return config{}, err
	}
	resolved := config{
		root: root, networkID: input.NetworkID, authorities: authorities,
		threshold: input.Threshold, now: initial, clock: clock,
		source: sourcePlan, sourceInfo: sourceInfo, observation: input.ClockObservation.UTC(), observe: observe,
		automatic: input.AutomaticRefreshInterval, profile: input.RuntimeProfile,
		anchorWall: initial, anchorMono: time.Now(),
	}
	if resolved.profile != "" && resolved.profile != "h3-s-v1" {
		return config{}, errors.New("runtime profile is not supported")
	}
	if resolved.automatic < 0 || resolved.automatic > time.Minute || resolved.automatic > 0 && resolved.automatic < 100*time.Millisecond {
		return config{}, errors.New("automatic refresh interval is invalid")
	}
	if resolved.automatic > 0 && input.ObserveClock == nil && input.ClockObservationFile == "" {
		return config{}, errors.New("automatic refresh requires live clock observations")
	}
	if resolved.automatic > 0 && !resolved.sourceInfo.Configured {
		return config{}, errors.New("automatic refresh requires a finite source plan")
	}
	return resolved, nil
}

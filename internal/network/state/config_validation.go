package state

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

func validateConfig(input Config) (config, error) {
	if input.Root == "" {
		return config{}, errors.New("state root is required")
	}
	if !pathgrammar.Strict(input.Root) {
		return config{}, errors.New("state root must be an absolute clean path")
	}
	root := input.Root
	if input.Threshold < 1 || input.Threshold > len(input.Authorities) {
		return config{}, errors.New("authority threshold is outside the authority set")
	}
	if len(input.Authorities) > 16 {
		return config{}, errors.New("authority set exceeds 16 keys")
	}
	if input.Now.IsZero() && input.Clock == nil {
		return config{}, errors.New("verification time is required")
	}
	if !input.Now.IsZero() && input.Clock != nil {
		return config{}, errors.New("verification time has multiple owners")
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
		if !pathgrammar.Strict(input.ClockObservationFile) {
			return config{}, errors.New("clock observation file must be an absolute clean path")
		}
		observe = fileClockObserver(input.ClockObservationFile)
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
	if input.Source.VerificationClock != nil {
		return config{}, errors.New("source verification clock is owned by Network State")
	}
	sourceInput := input.Source
	sourceInput.VerificationClock = clock
	sourcePlan, sourceInfo, err := source.New(sourceInput, authorities)
	if err != nil {
		return config{}, err
	}
	if (sourceInfo.Configured || sourceInfo.Serving) && input.LocalRoleStateRoot == "" {
		return config{}, errors.New("direct Source work requires local role state")
	}
	localRoles := input.LocalRoleStateRoot
	if localRoles != "" && !pathgrammar.Strict(localRoles) {
		return config{}, errors.New("local role state root must be an absolute clean path")
	}
	acceptedProfile := input.AcceptedProfile
	if acceptedProfile == "" {
		acceptedProfile = epoch.ProfileClosedRoute
	}
	if acceptedProfile != epoch.ProfileClosedRoute {
		return config{}, errors.New("network state requires the selected closed profile")
	}
	closedProfileAuthority := append(ed25519.PublicKey(nil), input.ClosedProfileAuthority...)
	if len(closedProfileAuthority) != ed25519.PublicKeySize {
		return config{}, errors.New("closed Route profile authority is required")
	}
	if _, exists := authorities[sha256.Sum256(closedProfileAuthority)]; !exists {
		return config{}, errors.New("closed Route profile authority is not pinned by State")
	}
	resolved := config{
		root: root, networkID: input.NetworkID, authorities: authorities,
		threshold: input.Threshold, closedProfileAuthority: closedProfileAuthority, acceptedProfile: acceptedProfile, now: initial, clock: clock,
		source: sourcePlan, fetchSourceOperation: sourcePlan.Fetch, sourceInfo: sourceInfo, observation: input.ClockObservation.UTC(), observe: observe,
		automatic: input.AutomaticRefreshInterval, permitWork: input.PermitWork,
		localRoles: localRoles,
		anchorWall: initial, anchorMono: time.Now(),
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

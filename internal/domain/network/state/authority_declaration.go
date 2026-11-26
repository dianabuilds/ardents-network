package state

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

// AuthorityDeclaration names the local pins and storage needed to open State.
// The declaration grants no verified authority.
type AuthorityDeclaration struct {
	Root                 string     `json:"root"`
	NetworkID            [32]byte   `json:"network_id"`
	Authorities          [][32]byte `json:"authorities"`
	Threshold            int        `json:"threshold"`
	ProfileAuthority     [32]byte   `json:"profile_authority"`
	ClockObservationFile string     `json:"clock_observation_file"`
}

// ValidShape checks bounded local pins without opening State.
func (plan *AuthorityDeclaration) ValidShape() bool {
	return plan != nil && pathgrammar.Strict(plan.Root) && pathgrammar.Strict(plan.ClockObservationFile) &&
		plan.NetworkID != [32]byte{} && plan.ProfileAuthority != [32]byte{} &&
		len(plan.Authorities) > 0 && len(plan.Authorities) <= 16
}

// Config copies a detached declaration for the State owner to authenticate.
func (plan *AuthorityDeclaration) Config() (Config, error) {
	if !plan.ValidShape() {
		return Config{}, errors.New("invalid Network owner plan")
	}
	authorities := make(map[[32]byte]ed25519.PublicKey, len(plan.Authorities))
	for _, key := range plan.Authorities {
		id := sha256.Sum256(key[:])
		if _, duplicate := authorities[id]; duplicate || key == [32]byte{} {
			return Config{}, errors.New("invalid Network authority pins")
		}
		authorities[id] = append(ed25519.PublicKey(nil), key[:]...)
	}
	return Config{Root: plan.Root, NetworkID: plan.NetworkID, Authorities: authorities,
		Threshold: plan.Threshold, ClosedProfileAuthority: append(ed25519.PublicKey(nil), plan.ProfileAuthority[:]...),
		AcceptedProfile: "ardents-route-v3", Clock: time.Now, ClockObservationFile: plan.ClockObservationFile}, nil
}

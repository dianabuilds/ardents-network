package preparation

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
)

var errSourcePreparation = errors.New("invalid initial Source State preparation")

// StateConfig projects independently selected public pins and
// independent paths. State authenticates the Epoch separately.
func StateConfig(plan servingdeclaration.ServingDeclaration) (state.Config, error) {
	n := plan.Network
	if n == nil || n.NetworkID == [32]byte{} || n.ProfileAuthority == [32]byte{} ||
		n.Threshold < 1 || n.Threshold > len(n.Authorities) || len(n.Authorities) > 16 {
		return state.Config{}, errSourcePreparation
	}
	paths := []string{n.Root, n.ClockObservationFile, plan.ParticipationRoot}
	for i, path := range paths {
		if path == "/" || len(path) > 4096 || !pathcheck.IsCleanAbsolute(path) || strings.ContainsRune(path, 0) {
			return state.Config{}, errSourcePreparation
		}
		for _, previous := range paths[:i] {
			if path == previous || strings.HasPrefix(path, previous+"/") || strings.HasPrefix(previous, path+"/") {
				return state.Config{}, errSourcePreparation
			}
		}
	}
	pins := make(map[[32]byte]ed25519.PublicKey, len(n.Authorities))
	for _, public := range n.Authorities {
		id := sha256.Sum256(public[:])
		if public == [32]byte{} || pins[id] != nil {
			return state.Config{}, errSourcePreparation
		}
		pins[id] = append(ed25519.PublicKey(nil), public[:]...)
	}
	return state.Config{
		Root: n.Root, NetworkID: n.NetworkID, Authorities: pins, Threshold: n.Threshold,
		ClosedProfileAuthority: append(ed25519.PublicKey(nil), n.ProfileAuthority[:]...),
		AcceptedProfile:        epoch.ProfileClosedRoute, Clock: time.Now, ClockObservationFile: n.ClockObservationFile,
	}, nil
}

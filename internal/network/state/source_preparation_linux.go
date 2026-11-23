package state

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

var errSourcePreparation = errors.New("invalid initial Source State preparation")

// SourcePreparationConfig projects independently selected public pins and
// independent paths. State authenticates the Epoch separately.
func SourcePreparationConfig(plan servingdeclaration.ServingDeclaration) (Config, error) {
	n := plan.Network
	if n == nil || n.NetworkID == [32]byte{} || n.ProfileAuthority == [32]byte{} ||
		n.Threshold < 1 || n.Threshold > len(n.Authorities) || len(n.Authorities) > 16 {
		return Config{}, errSourcePreparation
	}
	paths := []string{n.Root, n.ClockObservationFile, plan.ParticipationRoot}
	for i, path := range paths {
		if path == "/" || len(path) > 4096 || !pathgrammar.Strict(path) || strings.ContainsRune(path, 0) {
			return Config{}, errSourcePreparation
		}
		for _, previous := range paths[:i] {
			if path == previous || strings.HasPrefix(path, previous+"/") || strings.HasPrefix(previous, path+"/") {
				return Config{}, errSourcePreparation
			}
		}
	}
	pins := make(map[[32]byte]ed25519.PublicKey, len(n.Authorities))
	for _, public := range n.Authorities {
		id := sha256.Sum256(public[:])
		if public == [32]byte{} || pins[id] != nil {
			return Config{}, errSourcePreparation
		}
		pins[id] = append(ed25519.PublicKey(nil), public[:]...)
	}
	return Config{
		Root: n.Root, NetworkID: n.NetworkID, Authorities: pins, Threshold: n.Threshold,
		ClosedProfileAuthority: append(ed25519.PublicKey(nil), n.ProfileAuthority[:]...),
		AcceptedProfile:        epoch.ProfileClosedRoute, Clock: time.Now, ClockObservationFile: n.ClockObservationFile,
	}, nil
}

// prepare authenticates the first Epoch before initializing empty participation.
func (preparation sourceStatePreparation) prepare(ctx context.Context, check func() error) (result error) {
	config := preparation.config
	config.PermitWork = check
	if err := check(); err != nil {
		return err
	}
	owner, err := OpenFresh(config)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, owner.Close(), check()) }()
	if _, err := owner.Current(); !errors.Is(err, ErrNoCurrentGeneration) {
		return errors.Join(errSourcePreparation, err)
	}
	if _, err := owner.AcceptEpoch(ctx, preparation.bundle.Epoch, preparation.bundle.Inputs, preparation.bundle.Materials); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	if err := duty.Initialize(ctx, duty.Config{Root: preparation.participation, Clock: time.Now}); err != nil {
		return err
	}
	return check()
}

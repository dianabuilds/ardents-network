package command

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// offlineDeclaration contains detached inputs for one local State operation.
type offlineDeclaration struct {
	Config    state.Config
	Epoch     []byte
	Inputs    [][]byte
	Materials [][]byte
	Profile   []byte
}

// decodeDeclaration checks the operation-specific inputs before opening State.
func decodeDeclaration(raw []byte, operation string) (offlineDeclaration, error) {
	var input struct {
		Network   *state.AuthorityDeclaration `json:"network"`
		Epoch     []byte                      `json:"epoch,omitempty"`
		Inputs    [][]byte                    `json:"inputs,omitempty"`
		Materials [][]byte                    `json:"materials,omitempty"`
		Profile   []byte                      `json:"profile,omitempty"`
	}
	if (operation != "accept-epoch" && operation != "accept-profile" && operation != "inspect") || artifacts.DecodeObject(raw, &input) != nil ||
		(operation != "accept-epoch" && (len(input.Epoch) != 0 || len(input.Inputs) != 0 || len(input.Materials) != 0)) ||
		(operation != "accept-profile" && len(input.Profile) != 0) {
		return offlineDeclaration{}, errors.New("invalid offline State declaration")
	}
	config, err := input.Network.Config()
	if err != nil {
		return offlineDeclaration{}, err
	}
	return offlineDeclaration{Config: config, Epoch: input.Epoch, Inputs: input.Inputs, Materials: input.Materials, Profile: input.Profile}, nil
}

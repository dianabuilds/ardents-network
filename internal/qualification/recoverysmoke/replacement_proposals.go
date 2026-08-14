package recoverysmoke

import (
	"encoding/json"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func replacementProposals(raw []byte, mode string) ([]replacementProposal, error) {
	expected, err := replacementProposalCount(mode)
	if err != nil {
		return nil, err
	}
	result := make([]replacementProposal, 0, expected)
	for _, line := range splitLines(raw) {
		var value struct {
			Kind                     string            `json:"kind"`
			Role                     string            `json:"role"`
			Terminal                 string            `json:"terminal"`
			Attachment               uint32            `json:"attachment"`
			Positions                []route.Position  `json:"positions"`
			ExcludedIdentities       [][32]byte        `json:"excluded_identities"`
			IntroductionSetupReceipt [32]byte          `json:"introduction_setup_receipt"`
			IntroductionSetup        introductionProof `json:"introduction_setup"`
		}
		if decodeErr := json.Unmarshal(line, &value); decodeErr != nil {
			return nil, errors.Join(decodeErr, errors.New("decode client replacement proposal evidence"))
		}
		if value.Kind != "complete" || value.Role != "client" {
			continue
		}
		if len(value.Positions) != len(replacementRoles) || value.Attachment != uint32(len(result)+1) ||
			value.Terminal != "success" && value.Terminal != "error" {
			return nil, errors.New("replacement proposal evidence is malformed")
		}
		proposal := replacementProposal{Attachment: value.Attachment,
			ExcludedIdentities: append([][32]byte(nil), value.ExcludedIdentities...), Terminal: value.Terminal,
			Committed: value.Terminal == "success", IntroductionReceipt: value.IntroductionSetupReceipt,
			IntroductionProof: value.IntroductionSetup}
		for index, position := range value.Positions {
			proposal.NodeIDs[index], proposal.PublicKeys[index] = position.NodeID, position.PublicKey
		}
		result = append(result, proposal)
	}
	if len(result) != expected {
		return nil, errors.New("replacement proposal evidence count is incomplete")
	}
	for index := range result {
		wantCommitted := mode != "isolated-rendezvous" || index != 1
		if result[index].Committed != wantCommitted || (index == 2) != (result[index].IntroductionReceipt != [32]byte{}) {
			return nil, errors.New("replacement proposal outcome or Introduction setup is inconsistent")
		}
	}
	return result, nil
}

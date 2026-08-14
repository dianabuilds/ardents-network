package recoverysmoke

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

type selectedRoute map[string]route.Position

var replacementRoles = [...]string{"initiator", "introduction", "rendezvous", "responder"}

func replacementSelections(candidates []route.Position) ([]selectedRoute, error) {
	byRole := make(map[string][]route.Position, len(replacementRoles))
	identities := make(map[[32]byte]bool, len(candidates))
	for _, candidate := range candidates {
		if !replacementRole(candidate.Role) || candidate.NodeID == [32]byte{} ||
			candidate.PublicKey == [32]byte{} || candidate.Endpoint == "" || identities[candidate.NodeID] {
			return nil, errors.New("finite alternate candidate set is invalid")
		}
		identities[candidate.NodeID] = true
		byRole[candidate.Role] = append(byRole[candidate.Role], candidate)
	}
	for _, role := range replacementRoles {
		if len(byRole[role]) != 3 {
			return nil, errors.New("exactly three finite candidates per Route role are required")
		}
	}
	indexes := [][4]int{{0, 0, 0, 0}, {1, 1, 0, 1}, {2, 2, 2, 2}, {1, 1, 2, 1}}
	result := make([]selectedRoute, 0, len(indexes))
	for _, generation := range indexes {
		selected := make(selectedRoute, len(replacementRoles))
		for index, role := range replacementRoles {
			selected[role] = byRole[role][generation[index]]
		}
		result = append(result, selected)
	}
	return result, nil
}

func replacementRole(wanted string) bool {
	for _, role := range replacementRoles {
		if wanted == role {
			return true
		}
	}
	return false
}

func replacementProposalCount(mode string) (int, error) {
	switch mode {
	case "isolated-initiator", "isolated-introduction", "isolated-responder":
		return 2, nil
	case "isolated-rendezvous":
		return 3, nil
	case "sequential-three":
		return 4, nil
	default:
		return 0, errors.New("replacement mode is outside the bounded proposal policy")
	}
}

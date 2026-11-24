//go:build linux || darwin

package serving

import (
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

// ValidDeclaration checks the detached operational input shape.
func ValidDeclaration(plan servingdeclaration.ServingDeclaration) bool {
	pins := (*state.AuthorityDeclaration)(plan.Network)
	if plan.ProcessProfile != processresource.SourceProfile || !pins.ValidShape() ||
		!pathgrammar.Strict(plan.ParticipationRoot) || pins.Threshold <= 0 || pins.Threshold > len(pins.Authorities) {
		return false
	}
	split := string(filepath.Separator)
	if pins.Root == plan.ParticipationRoot || strings.HasPrefix(pins.Root, plan.ParticipationRoot+split) ||
		strings.HasPrefix(plan.ParticipationRoot, pins.Root+split) {
		return false
	}
	return pathgrammar.Strict(plan.Certificate) && pathgrammar.Strict(plan.PrivateKey) &&
		pathgrammar.Strict(plan.ClientRoot) && len(plan.ClientPins) > 0 && len(plan.ClientPins) <= 3 && plan.Address != ""
}

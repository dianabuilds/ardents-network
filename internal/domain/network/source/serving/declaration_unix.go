//go:build linux || darwin

package serving

import (
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
	"github.com/dianabuilds/ardents-network/internal/platform/processguard"
)

// ValidDeclaration checks the detached operational input shape.
func ValidDeclaration(plan servingdeclaration.ServingDeclaration) bool {
	pins := (*state.AuthorityDeclaration)(plan.Network)
	if plan.ProcessProfile != processguard.SourceProfile || !pins.ValidShape() ||
		!pathcheck.IsCleanAbsolute(plan.ParticipationRoot) || pins.Threshold <= 0 || pins.Threshold > len(pins.Authorities) {
		return false
	}
	split := string(filepath.Separator)
	if pins.Root == plan.ParticipationRoot || strings.HasPrefix(pins.Root, plan.ParticipationRoot+split) ||
		strings.HasPrefix(plan.ParticipationRoot, pins.Root+split) {
		return false
	}
	return pathcheck.IsCleanAbsolute(plan.Certificate) && pathcheck.IsCleanAbsolute(plan.PrivateKey) &&
		pathcheck.IsCleanAbsolute(plan.ClientRoot) && len(plan.ClientPins) > 0 && len(plan.ClientPins) <= 3 && plan.Address != ""
}

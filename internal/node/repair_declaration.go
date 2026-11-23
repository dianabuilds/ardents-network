package node

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/processresource"
)

var errRepairDeclaration = errors.New("invalid node repair declaration")

// RepairDeclaration projects the original scope of a fixed Node producer.
// State separately authenticates the retained original and its repair proof.
func RepairDeclaration(role string, raw []byte) (Network, []string, error) {
	var core nodePlanCore
	var profile, extraRoot string
	var err error
	switch role {
	case "forwarding":
		var plan ForwardingPlan
		plan, err = DecodeForwardingPlan(raw)
		core, profile = plan.nodePlanCore, processresource.Profile
	case "join":
		var plan JoinPlan
		plan, err = DecodeJoinPlan(raw)
		core, profile = plan.nodePlanCore, processresource.JoinProfile
	case "descriptor":
		var plan DescriptorPlan
		plan, err = DecodeDescriptorPlan(raw)
		core, profile, extraRoot = plan.nodePlanCore, processresource.DescriptorProfile, plan.DescriptorRoot
	case "introduction":
		var plan IntroductionPlan
		plan, err = DecodeIntroductionPlan(raw)
		core, profile, extraRoot = plan.nodePlanCore, processresource.IntroductionProfile, plan.IntroductionRoot
	default:
		return Network{}, nil, errRepairDeclaration
	}
	if err != nil || core.Network == nil || core.ProcessProfile != profile {
		return Network{}, nil, errRepairDeclaration
	}
	roots := []string{core.Network.Root, core.ParticipationRoot, core.SpendRoot, core.HostingRoot}
	if role == "descriptor" || role == "introduction" {
		roots = append(roots, extraRoot)
	}
	if !separateRoots(roots...) {
		return Network{}, nil, errRepairDeclaration
	}
	return *core.Network, roots, nil
}

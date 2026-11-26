package node

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/pathcheck"
	"github.com/dianabuilds/ardents-network/internal/platform/processguard"
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
		if err != nil {
			return Network{}, nil, errRepairDeclaration
		}
		core, profile = plan.nodePlanCore, processguard.ForwardingProfile
	case "join":
		var plan JoinPlan
		plan, err = DecodeJoinPlan(raw)
		if err != nil {
			return Network{}, nil, errRepairDeclaration
		}
		core, profile = plan.nodePlanCore, processguard.JoinProfile
	case "descriptor":
		var plan DescriptorPlan
		plan, err = DecodeDescriptorPlan(raw)
		if err != nil {
			return Network{}, nil, errRepairDeclaration
		}
		core, profile, extraRoot = plan.nodePlanCore, processguard.DescriptorProfile, plan.DescriptorRoot
	case "introduction":
		var plan IntroductionPlan
		plan, err = DecodeIntroductionPlan(raw)
		if err != nil {
			return Network{}, nil, errRepairDeclaration
		}
		core, profile, extraRoot = plan.nodePlanCore, processguard.IntroductionProfile, plan.IntroductionRoot
	default:
		return Network{}, nil, errRepairDeclaration
	}
	if core.Network == nil || core.ProcessProfile != profile {
		return Network{}, nil, errRepairDeclaration
	}
	roots := []string{core.Network.Root, core.ParticipationRoot, core.SpendRoot, core.HostingRoot}
	if role == "descriptor" || role == "introduction" {
		roots = append(roots, extraRoot)
	}
	if !pathcheck.DisjointRoots(roots...) {
		return Network{}, nil, errRepairDeclaration
	}
	return *core.Network, roots, nil
}

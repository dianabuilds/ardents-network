package node

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/domain/route/role"
)

// ReceivingPlan is the runtime start seam for the four non-Issuer plans.
type ReceivingPlan interface {
	ForwardingPlan | DescriptorPlan | IntroductionPlan | JoinPlan
	ValidInvocation() bool
	InvocationTarget() (*Network, string, [32]byte, uint32)
	Start(context.Context, role.Authority, bool) (RuntimeInvocation, error)
}

// StartReceivingRole binds a typed plan to the current duty of the original
// State. The caller retains that State through joined completion.
func StartReceivingRole[P ReceivingPlan](ctx context.Context, owner *state.State, plan P, retained bool) (RuntimeInvocation, error) {
	_, _, nodeID, _ := plan.InvocationTarget()
	authority, err := originalRoleAuthority(owner, nodeID)
	if err != nil {
		return nil, err
	}
	return plan.Start(ctx, authority, retained)
}

// StartIssuingRole binds the Issuer to its original State duty and exact
// prepared issuance declaration.
func StartIssuingRole(ctx context.Context, owner *state.State, plan IssuerPlan, retained bool, issuance issuer.Plan) (RuntimeInvocation, error) {
	authority, err := originalRoleAuthority(owner, plan.NodeID)
	if err != nil {
		return nil, err
	}
	return plan.Start(ctx, authority, retained, issuance)
}

func originalRoleAuthority(owner *state.State, nodeID [32]byte) (role.Authority, error) {
	view, err := owner.CurrentRuntime()
	if err != nil {
		return role.Authority{}, err
	}
	duty, err := view.RetainDuty(nodeID, view.ObservedAt())
	if err != nil {
		return role.Authority{}, err
	}
	return role.Authority{Current: owner.CurrentRuntime, Duty: duty, Profile: view.ProfileBinding()}, nil
}

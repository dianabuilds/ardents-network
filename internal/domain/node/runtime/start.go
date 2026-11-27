package runtime

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	nodeplan "github.com/dianabuilds/ardents-network/internal/domain/node/plan"
	"github.com/dianabuilds/ardents-network/internal/domain/route/role"
)

// StartReceivingRole binds a typed plan to the current duty of the original
// State. The caller retains that State through joined completion.
func StartReceivingRole[P nodeplan.Receiving](ctx context.Context, owner *state.State, plan P, retained bool) (RuntimeInvocation, error) {
	_, _, nodeID, _ := plan.InvocationTarget()
	authority, err := originalRoleAuthority(owner, nodeID)
	if err != nil {
		return nil, err
	}
	switch declared := any(plan).(type) {
	case nodeplan.ForwardingPlan:
		return startForwarding(ctx, authority, declared, retained)
	case nodeplan.DescriptorPlan:
		return startDescriptor(ctx, authority, declared, retained)
	case nodeplan.IntroductionPlan:
		return startIntroduction(ctx, authority, declared, retained)
	case nodeplan.JoinPlan:
		return startJoin(ctx, authority, declared, retained)
	}
	return nil, errors.New("original Node receiving declaration unavailable")
}

// StartIssuingRole binds the Issuer to its original State duty and exact
// prepared issuance declaration.
func StartIssuingRole(ctx context.Context, owner *state.State, plan nodeplan.IssuerPlan, retained bool, issuance issuer.Plan) (RuntimeInvocation, error) {
	authority, err := originalRoleAuthority(owner, plan.NodeID)
	if err != nil {
		return nil, err
	}
	return startIssuer(ctx, authority, plan, retained, issuance)
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

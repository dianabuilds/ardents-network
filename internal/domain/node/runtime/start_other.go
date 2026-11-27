//go:build !linux && !darwin

package runtime

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	nodeplan "github.com/dianabuilds/ardents-network/internal/domain/node/plan"
	"github.com/dianabuilds/ardents-network/internal/domain/route/role"
)

var errNativeRuntimeUnavailable = errors.New("native Node runtime unavailable")

func startForwarding(context.Context, role.Authority, nodeplan.ForwardingPlan, bool) (RuntimeInvocation, error) {
	return nil, errNativeRuntimeUnavailable
}

func startIssuer(context.Context, role.Authority, nodeplan.IssuerPlan, bool, issuer.Plan) (RuntimeInvocation, error) {
	return nil, errNativeRuntimeUnavailable
}

func startDescriptor(context.Context, role.Authority, nodeplan.DescriptorPlan, bool) (RuntimeInvocation, error) {
	return nil, errNativeRuntimeUnavailable
}

func startIntroduction(context.Context, role.Authority, nodeplan.IntroductionPlan, bool) (RuntimeInvocation, error) {
	return nil, errNativeRuntimeUnavailable
}

func startJoin(context.Context, role.Authority, nodeplan.JoinPlan, bool) (RuntimeInvocation, error) {
	return nil, errNativeRuntimeUnavailable
}

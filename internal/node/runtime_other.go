//go:build !linux

package node

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

var errNativeRuntimeUnavailable = errors.New("native Node runtime unavailable")

func (ForwardingPlan) Start(context.Context, role.Authority, bool) (RuntimeInvocation, error) {
	return nil, errNativeRuntimeUnavailable
}

func (IssuerPlan) Start(context.Context, role.Authority, bool, issuer.Plan) (RuntimeInvocation, error) {
	return nil, errNativeRuntimeUnavailable
}

func (DescriptorPlan) Start(context.Context, role.Authority, bool) (RuntimeInvocation, error) {
	return nil, errNativeRuntimeUnavailable
}

func (IntroductionPlan) Start(context.Context, role.Authority, bool) (RuntimeInvocation, error) {
	return nil, errNativeRuntimeUnavailable
}

func (JoinPlan) Start(context.Context, role.Authority, bool) (RuntimeInvocation, error) {
	return nil, errNativeRuntimeUnavailable
}

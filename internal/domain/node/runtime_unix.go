//go:build linux || darwin

package node

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/route/role"
)

// Start invokes the original Forwarding runtime for this declaration.
func (plan ForwardingPlan) Start(ctx context.Context, authority role.Authority, retained bool) (RuntimeInvocation, error) {
	listen := ListenForwarding
	if retained {
		listen = ListenForwardingRetained
	}
	return listen(ctx, ForwardingConfig{
		Authority:         authority,
		ParticipationRoot: plan.ParticipationRoot,
		SpendRoot:         plan.SpendRoot,
		HostingRoot:       plan.HostingRoot,
		Certificate:       plan.Certificate,
		PrivateKey:        plan.PrivateKey,
		Work:              plan.Work,
		Termination:       plan.Termination,
	})
}

// Start invokes the original Issuer runtime for this declaration.
func (plan IssuerPlan) Start(ctx context.Context, authority role.Authority, retained bool, issuancePlan issuer.Plan) (RuntimeInvocation, error) {
	listen := ListenIssuer
	if retained {
		listen = ListenIssuerRetained
	}
	return listen(ctx, IssuerConfig{
		Authority:         authority,
		ParticipationRoot: plan.ParticipationRoot,
		SpendRoot:         plan.SpendRoot,
		HostingRoot:       plan.HostingRoot,
		Certificate:       plan.Certificate,
		PrivateKey:        plan.PrivateKey,
		Issuance:          issuancePlan,
		Work:              plan.Work,
		BootstrapWork:     plan.BootstrapWork,
		Termination:       plan.Termination,
	})
}

// Start invokes the original Descriptor runtime for this declaration.
func (plan DescriptorPlan) Start(ctx context.Context, authority role.Authority, retained bool) (RuntimeInvocation, error) {
	listen := ListenDescriptor
	if retained {
		listen = ListenDescriptorRetained
	}
	return listen(ctx, DescriptorConfig{
		Authority:         authority,
		ParticipationRoot: plan.ParticipationRoot,
		SpendRoot:         plan.SpendRoot,
		HostingRoot:       plan.HostingRoot,
		DescriptorRoot:    plan.DescriptorRoot,
		Certificate:       plan.Certificate,
		PrivateKey:        plan.PrivateKey,
		Work:              plan.Work,
		Termination:       plan.Termination,
	})
}

// Start invokes the original Introduction runtime for this declaration.
func (plan IntroductionPlan) Start(ctx context.Context, authority role.Authority, retained bool) (RuntimeInvocation, error) {
	listen := ListenIntroduction
	if retained {
		listen = ListenIntroductionRetained
	}
	return listen(ctx, IntroductionConfig{
		Authority:         authority,
		ParticipationRoot: plan.ParticipationRoot,
		SpendRoot:         plan.SpendRoot,
		HostingRoot:       plan.HostingRoot,
		IntroductionRoot:  plan.IntroductionRoot,
		Certificate:       plan.Certificate,
		PrivateKey:        plan.PrivateKey,
		Work:              plan.Work,
		Termination:       plan.Termination,
	})
}

// Start invokes the original Join runtime for this declaration.
func (plan JoinPlan) Start(ctx context.Context, authority role.Authority, retained bool) (RuntimeInvocation, error) {
	listen := ListenJoin
	if retained {
		listen = ListenJoinRetained
	}
	return listen(ctx, JoinConfig{
		Authority:         authority,
		ParticipationRoot: plan.ParticipationRoot,
		SpendRoot:         plan.SpendRoot,
		HostingRoot:       plan.HostingRoot,
		Certificate:       plan.Certificate,
		PrivateKey:        plan.PrivateKey,
		Work:              plan.Work,
		Termination:       plan.Termination,
	})
}

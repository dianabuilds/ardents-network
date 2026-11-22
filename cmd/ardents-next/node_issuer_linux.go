//go:build linux

package main

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

// State is owned here; Node joins its original dependents before State closes.
func runNodeIssuer(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	retained := len(args) == 2 && args[0] == "--retained-config"
	if retained {
		args = []string{"--config", args[1]}
	}
	if ctx == nil || len(args) != 2 || args[0] != "--config" {
		return 2
	}
	raw, err := readBounded(args[1], 16<<10)
	if err != nil {
		return 2
	}
	plan, err := node.DecodeIssuerPlan(raw)
	if err != nil {
		return 2
	}
	network := (*networkAuthorityPlan)(plan.Network)
	if plan.Network == nil || plan.ProcessProfile != processresource.IssuerProfile || plan.RetirementWaitMS == 0 || plan.RetirementWaitMS > 5000 {
		return 2
	}
	prepared, err := decodeIssuanceResultPlan(plan.Issuance, "initialize-results")
	if err != nil || !validAdmissionAuthority("", network, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, prepared.Admission.Root, prepared.Keys.Root, prepared.Root) || !independentRouteRoots(network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, prepared.Admission.Root, prepared.Keys.Root, prepared.Root) {
		return 2
	}
	config, err := networkStateConfig(network)
	if err != nil {
		return 2
	}
	config.LocalRoleStateRoot = plan.ParticipationRoot
	listen := node.ListenIssuer
	if retained {
		listen = node.ListenIssuerRetained
	}
	return runNodeWithState(ctx, out, diagnostic, "node.issuer", config, plan.NodeID, plan.RetirementWaitMS, func(authority role.Authority) (*node.Issuer, error) {
		return listen(ctx, node.IssuerConfig{
			Authority:         authority,
			ParticipationRoot: plan.ParticipationRoot,
			SpendRoot:         plan.SpendRoot,
			HostingRoot:       plan.HostingRoot,
			Certificate:       plan.Certificate,
			PrivateKey:        plan.PrivateKey,
			Issuance: issuer.Plan{
				AdmissionRoot:    prepared.Admission.Root,
				AdmissionBinding: prepared.Admission.Binding,
				KeyRoot:          prepared.Keys.Root,
				KeyBinding:       prepared.Keys.Binding,
				ResultRoot:       prepared.Root,
			},
			Work:          plan.Work,
			BootstrapWork: plan.BootstrapWork,
			Termination:   plan.Termination,
		})
	})
}

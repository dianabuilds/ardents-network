//go:build linux

package main

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/node"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// State is owned here; Node joins its original dependents before State closes.
func runNodeIssuer(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	var plan struct {
		Network           *networkAuthorityPlan `json:"network"`
		NodeID            [32]byte              `json:"node_id"`
		ProcessProfile    string                `json:"process_profile"`
		ParticipationRoot string                `json:"participation_root"`
		SpendRoot         string                `json:"spend_root"`
		HostingRoot       string                `json:"hosting_root"`
		Certificate       string                `json:"certificate"`
		PrivateKey        string                `json:"private_key"`
		Issuance          json.RawMessage       `json:"issuance"`
		Work              hosting.Traffic       `json:"work"`
		BootstrapWork     hosting.Traffic       `json:"bootstrap_work"`
		Termination       hosting.Traffic       `json:"termination"`
		RetirementWaitMS  uint32                `json:"retirement_wait_ms"`
	}
	if ctx == nil || admissionConfig(args, &plan) != nil || plan.Network == nil || plan.ProcessProfile != processresource.IssuerProfile || plan.RetirementWaitMS == 0 || plan.RetirementWaitMS > 5000 {
		return 2
	}
	prepared, err := decodeIssuanceResultPlan(plan.Issuance, "initialize-results")
	if err != nil || !validAdmissionAuthority("", plan.Network, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, prepared.Admission.Root, prepared.Keys.Root, prepared.Root) || !independentRouteRoots(plan.Network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, prepared.Admission.Root, prepared.Keys.Root, prepared.Root) {
		return 2
	}
	config, err := networkStateConfig(plan.Network)
	if err != nil {
		return 2
	}
	config.LocalRoleStateRoot = plan.ParticipationRoot
	owner, err := state.Open(config)
	if err != nil {
		_ = nodeIssuerReport(out, "refused", "state-unavailable", "", "")
		return 1
	}
	refuse := func(cause error) int {
		_ = owner.Close()
		if nodeIssuerReport(out, "refused", "startup-unavailable", "", "", encodeNodeFailureDiagnostic(node.StartupFailure(cause))) != nil {
			return 2
		}
		return 1
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		return refuse(err)
	}
	duty, err := view.RetainDuty(plan.NodeID, view.ObservedAt())
	if err != nil {
		return refuse(err)
	}
	original, err := node.ListenIssuer(ctx, node.IssuerConfig{
		Authority:         role.Authority{Current: owner.CurrentRuntime, Duty: duty, Profile: view.Profile().ProfileBinding},
		ParticipationRoot: plan.ParticipationRoot, SpendRoot: plan.SpendRoot, HostingRoot: plan.HostingRoot,
		Certificate: plan.Certificate, PrivateKey: plan.PrivateKey,
		Issuance: issuer.Plan{AdmissionRoot: prepared.Admission.Root, AdmissionBinding: prepared.Admission.Binding, KeyRoot: prepared.Keys.Root, KeyBinding: prepared.Keys.Binding, ResultRoot: prepared.Root},
		Work:     plan.Work, BootstrapWork: plan.BootstrapWork, Termination: plan.Termination,
	})
	if err != nil {
		return refuse(err)
	}
	outputErr := nodeIssuerReport(out, "listening", "original-duty", "", "")
	if outputErr == nil {
		select {
		case <-ctx.Done():
		case <-original.Done():
		}
	}
	original.Stop()
	type completion struct {
		outcome, first, cleanup string
		stateError              error
	}
	joined := make(chan completion, 1)
	go func() {
		_ = original.Close()
		first, cleanup := original.JoinedFailureOwners()
		joined <- completion{original.JoinedOutcome(), first, cleanup, owner.Close()}
	}()
	timer := time.NewTimer(time.Duration(plan.RetirementWaitMS) * time.Millisecond)
	defer timer.Stop()
	var result completion
	timedOut := false
	select {
	case result = <-joined:
	case <-timer.C:
		timedOut = true
		_ = nodeIssuerReport(diagnostic, "retiring", "completion-unavailable", "", "")
		// Preserve original roots/keys/producer/State while physical work joins.
		// Reporting a bounded failure never exits the process ahead of its users.
		result = <-joined
	}
	if outputErr != nil {
		return 2
	}
	if timedOut || result.outcome != "canceled" || result.stateError != nil {
		if result.stateError != nil {
			result.cleanup = "network"
		}
		_ = nodeIssuerReport(diagnostic, "joined", "failed", result.first, result.cleanup, encodeNodeFailureDiagnostic(original.FirstFailure()))
		return 1
	}
	if nodeIssuerReport(diagnostic, "joined", "canceled", "", "") != nil {
		return 2
	}
	return 130
}

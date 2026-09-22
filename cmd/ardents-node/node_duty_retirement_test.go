package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNodeCommandRefusesOldDutyReservationsBeforeEffects(t *testing.T) {
	for _, test := range []struct {
		name       string
		selectDuty func(*nodePlan)
	}{
		{name: "rendezvous", selectDuty: func(plan *nodePlan) { plan.Rendezvous = &rendezvousPlan{} }},
		{name: "initiator", selectDuty: func(plan *nodePlan) { plan.Initiator = &initiatorPlan{} }},
		{name: "introduction", selectDuty: func(plan *nodePlan) { plan.Introduction = &introductionPlan{} }},
		{name: "responder", selectDuty: func(plan *nodePlan) { plan.Responder = &responderPlan{} }},
		{name: "transit issuer", selectDuty: func(plan *nodePlan) { plan.TransitIssuer = &transitIssuerPlan{} }},
		{name: "old and closed", selectDuty: func(plan *nodePlan) {
			plan.Rendezvous = &rendezvousPlan{}
			plan.ClosedForwarding = &closedForwardingPlan{}
			plan.ClosedProfileAuthority = plan.AuthorityPublic[0]
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := oldDutyRetirementPlan(t)
			test.selectDuty(&plan)
			path := writeForwardingNodePlan(t, plan)

			err := runNode(t.Context(), path, new(bytes.Buffer))
			if !errors.Is(err, errOldNodeDutyRetired) {
				t.Fatalf("old duty error = %v", err)
			}
			for _, root := range []string{plan.StateRoot, plan.LocalRoleStateRoot} {
				if _, statErr := os.Stat(root); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("retired duty changed root %q: %v", root, statErr)
				}
			}
		})
	}
}

func oldDutyRetirementPlan(t *testing.T) nodePlan {
	t.Helper()
	plan := forwardingNodePlan(t)
	missing := filepath.Join(t.TempDir(), "missing")
	plan.StateRoot = filepath.Join(missing, "state")
	plan.LocalRoleStateRoot = filepath.Join(missing, "role")
	plan.ClockObservationFile = filepath.Join(missing, "clock")
	plan.SourceClientCertificate = filepath.Join(missing, "source.pem")
	plan.SourceClientKey = filepath.Join(missing, "source.key")
	plan.IdentityKey = filepath.Join(missing, "identity.key")
	plan.ServerCertificate = filepath.Join(missing, "node.pem")
	plan.ServerKey = filepath.Join(missing, "node.key")
	for index := range plan.Sources {
		plan.Sources[index].RootCA = filepath.Join(missing, "source-root.pem")
	}
	plan.ClosedProfileAuthority = ""
	plan.ClosedForwarding = nil
	return plan
}

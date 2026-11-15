package main

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/introduction"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/join"
)

func TestNodeStatePreparationRejectsForeignRole(t *testing.T) {
	p := forwardingStatePlanFixture()
	joinPlan := deployment.JoinPlan(p)
	if _, err := joinPreparationStateConfig(joinPlan); !errors.Is(err, join.ErrInput) {
		t.Fatalf("JOIN accepted foreign role: %v", err)
	}
	descriptorPlan := deployment.DescriptorPlan{Schema: p.Schema, ProcessProfile: p.ProcessProfile, NodeID: p.NodeID, Network: p.Network}
	if _, err := descriptorPreparationStateConfig(descriptorPlan); !errors.Is(err, descriptor.ErrInput) {
		t.Fatalf("Descriptor accepted foreign role: %v", err)
	}
	introductionPlan := deployment.IntroductionPlan{Schema: p.Schema, ProcessProfile: p.ProcessProfile, NodeID: p.NodeID, Network: p.Network}
	if _, err := introductionPreparationStateConfig(introductionPlan); !errors.Is(err, introduction.ErrInput) {
		t.Fatalf("Introduction accepted foreign role: %v", err)
	}
}

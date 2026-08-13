package main

import "testing"

func TestRolePlansRejectCrossRoleFieldsBeforeActorConstruction(t *testing.T) {
	tests := []actorPlan{
		{Role: "initiator", StateRoot: "forbidden"},
		{Role: "introduction", PublisherPin: "forbidden"},
		{Role: "rendezvous", ServiceKey: "forbidden"},
		{Role: "publisher", Seed: "forbidden"},
		{Role: "publisher", Next: "forbidden"},
		{Role: "client", Listen: "forbidden"},
		{Role: "client", ServiceCertificate: "forbidden"},
	}
	for _, input := range tests {
		if err := input.validateRoleLocal(); err == nil {
			t.Fatalf("cross-role plan was accepted: %+v", input)
		}
	}
}

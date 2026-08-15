package route

import "testing"

func TestValidateRejectsIncompleteOrRepeatedRoutePositions(t *testing.T) {
	valid := Plan{NetworkID: [32]byte{1}, Generation: "generation", Epoch: 1, Digest: [32]byte{2},
		Profile: "h3-route-tracer-v1", ViewRoot: [32]byte{3}, Seed: [32]byte{4}, SelectionAt: 1,
		Positions: []Position{
			{Role: "initiator", Domain: "initiator", NodeID: [32]byte{1}, PublicKey: [32]byte{11}, Family: "a", Endpoint: "127.0.0.1:1", Capacity: 1},
			{Role: "introduction", Domain: "introduction", NodeID: [32]byte{2}, PublicKey: [32]byte{12}, Family: "b", Endpoint: "127.0.0.1:2", Capacity: 1},
			{Role: "rendezvous", Domain: "rendezvous", NodeID: [32]byte{3}, PublicKey: [32]byte{13}, Family: "c", Endpoint: "127.0.0.1:3", Capacity: 1},
			{Role: "responder", Domain: "responder", NodeID: [32]byte{4}, PublicKey: [32]byte{14}, Family: "d", Endpoint: "127.0.0.1:4", Capacity: 1},
		}}
	if err := Validate(valid); err != nil {
		t.Fatalf("valid Route rejected: %v", err)
	}
	tests := []struct {
		name   string
		change func(*Plan)
	}{
		{"short path", func(plan *Plan) { plan.Positions = plan.Positions[:3] }},
		{"wrong domain", func(plan *Plan) { plan.Positions[1].Domain = "other" }},
		{"duplicate identity", func(plan *Plan) { plan.Positions[1].NodeID = plan.Positions[0].NodeID }},
		{"duplicate family", func(plan *Plan) { plan.Positions[1].Family = plan.Positions[0].Family }},
		{"duplicate endpoint", func(plan *Plan) { plan.Positions[1].Endpoint = plan.Positions[0].Endpoint }},
		{"non literal endpoint", func(plan *Plan) { plan.Positions[1].Endpoint = "node.example:4102" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := valid
			plan.Positions = append([]Position(nil), valid.Positions...)
			test.change(&plan)
			if err := Validate(plan); err == nil {
				t.Fatal("invalid Route validated")
			}
		})
	}
}

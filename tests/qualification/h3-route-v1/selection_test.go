package h3route_test

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClientSelectionRejectsIdentityFamilyDomainAndShortPaths(t *testing.T) {
	fixture := newProcessFixture(t)
	plan, err := route.Select(fixture.snapshot, route.Selection{Seed: fixture.selectionSeed, At: fixture.now})
	if err != nil || len(plan.Positions) != 4 {
		t.Fatalf("complete authenticated Route was not selected: %+v err=%v", plan, err)
	}
	tests := []struct {
		name   string
		change func(int)
	}{
		{"wrong domain", func(index int) { fixture.snapshot.Candidates[index].Domain = "other" }},
		{"duplicate identity", func(index int) { fixture.snapshot.Candidates[index].NodeID = fixture.snapshot.Candidates[0].NodeID }},
		{"duplicate family", func(index int) { fixture.snapshot.Candidates[index].Family = fixture.snapshot.Candidates[0].Family }},
		{"duplicate endpoint", func(index int) { fixture.snapshot.Candidates[index].Endpoint = fixture.snapshot.Candidates[0].Endpoint }},
		{"non literal endpoint", func(index int) { fixture.snapshot.Candidates[index].Endpoint = "node.example:4102" }},
	}
	base := fixture.snapshot.Candidates
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture.snapshot.Candidates = base
			test.change(1)
			if _, err := route.Select(fixture.snapshot, route.Selection{Seed: fixture.selectionSeed, At: fixture.now}); err == nil {
				t.Fatal("invalid authenticated Candidate set produced a Route")
			}
		})
	}
	fixture.snapshot.Candidates = base
	if _, err := route.Select(fixture.snapshot, route.Selection{Seed: fixture.selectionSeed, At: fixture.now,
		ExcludedIdentities: [][32]byte{fixture.snapshot.Candidates[0].NodeID}}); err == nil {
		t.Fatal("identity exclusion produced a Route")
	}
	if _, err := route.Select(fixture.snapshot, route.Selection{Seed: fixture.selectionSeed, At: fixture.now,
		ExcludedFamilies: []string{fixture.snapshot.Candidates[0].Family}}); err == nil {
		t.Fatal("family exclusion produced a Route")
	}
	plan.Positions = plan.Positions[:3]
	if err := route.Validate(plan); err == nil {
		t.Fatal("shortened Route validated")
	}
}

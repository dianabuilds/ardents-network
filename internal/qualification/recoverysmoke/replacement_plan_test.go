package recoverysmoke

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestReplacementSelectionsUseOneFixedLayeredPolicy(t *testing.T) {
	candidates := make([]route.Position, 0, 12)
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	for generation := range 3 {
		for roleIndex, role := range roles {
			candidates = append(candidates, route.Position{Role: role, NodeID: [32]byte{byte(1 + generation*4 + roleIndex)},
				PublicKey: [32]byte{byte(21 + generation*4 + roleIndex)}, Endpoint: role})
		}
	}
	selections, err := replacementSelections(candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(selections) != 4 {
		t.Fatalf("selection generations=%d; want 4", len(selections))
	}
	if selections[1]["rendezvous"].NodeID != selections[0]["rendezvous"].NodeID {
		t.Fatal("leg replacement changed the live Rendezvous")
	}
	if selections[2]["rendezvous"].NodeID == selections[1]["rendezvous"].NodeID {
		t.Fatal("Rendezvous failure reused the failed candidate")
	}
	for event, role := range []string{"initiator", "rendezvous", "responder"} {
		if selections[event+1][role].NodeID == selections[event][role].NodeID {
			t.Fatalf("event %d reused failed %s", event, role)
		}
	}
}

func TestClientReplacementPlanContainsOnlyTheFixedLayerPolicy(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "route", "plans", "client.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	candidates := make([]route.Position, 0, 12)
	for generation := range 3 {
		for roleIndex, role := range replacementRoles {
			candidates = append(candidates, route.Position{Role: role,
				NodeID: [32]byte{byte(1 + generation*4 + roleIndex)}, PublicKey: [32]byte{byte(31 + generation*4 + roleIndex)},
				Endpoint: role + string(rune('0'+generation))})
		}
	}
	selections, err := replacementSelections(candidates)
	if err != nil {
		t.Fatal(err)
	}
	if err := byteio.WriteJSON(path, map[string]any{"Role": "client", "Attachments": 1}, 64<<10); err != nil {
		t.Fatal(err)
	}
	if err := configureClientReplacementPlan(root, candidates, selections, [32]byte{99}, "30s"); err != nil {
		t.Fatal(err)
	}
	raw, err := byteio.ReadFile(path, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Deadline, Lifetime string
		AttachmentPlans    []struct {
			ExcludedIdentities      []string
			IntroductionSetupSocket string
		}
	}
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.AttachmentPlans) != 4 || len(plan.AttachmentPlans[0].ExcludedIdentities) != 0 ||
		len(plan.AttachmentPlans[1].ExcludedIdentities) != 3 ||
		len(plan.AttachmentPlans[2].ExcludedIdentities) != 8 ||
		len(plan.AttachmentPlans[3].ExcludedIdentities) != 5 ||
		plan.AttachmentPlans[2].IntroductionSetupSocket == "" ||
		plan.Deadline != replacementSetupDeadline || plan.Lifetime != "30s" {
		t.Fatalf("fixed layered policy differs: %+v", plan.AttachmentPlans)
	}
}

func TestClientReplacementPlanUsesExactActiveProposalCount(t *testing.T) {
	candidates := replacementPlanTestCandidates()
	selections, err := replacementSelections(candidates)
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{2, 3, 4} {
		root := filepath.Join(t.TempDir(), string(rune('0'+count)))
		path := filepath.Join(root, "route", "plans", "client.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := byteio.WriteJSON(path, map[string]any{"Role": "client", "Attachments": 1}, 64<<10); err != nil {
			t.Fatal(err)
		}
		if err := configureClientReplacementPlan(root, candidates, selections[:count], [32]byte{99}, "30s"); err != nil {
			t.Fatalf("configure %d active proposals: %v", count, err)
		}
		raw, err := byteio.ReadFile(path, 64<<10)
		if err != nil {
			t.Fatal(err)
		}
		var plan struct{ AttachmentPlans []json.RawMessage }
		if err := json.Unmarshal(raw, &plan); err != nil {
			t.Fatal(err)
		}
		if len(plan.AttachmentPlans) != count {
			t.Fatalf("active proposals=%d; client plan has %d", count, len(plan.AttachmentPlans))
		}
	}
}

func TestNodeReplacementPlansUseOnlyActiveLocalProposals(t *testing.T) {
	candidates := replacementPlanTestCandidates()
	selections, err := replacementSelections(candidates)
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{2, 3, 4} {
		for ordinal, service := range []string{"introduction", "introduction-2", "introduction-3"} {
			position, err := serviceCandidate(candidates, service)
			if err != nil {
				t.Fatal(err)
			}
			expected := 0
			for _, selection := range selections[:count] {
				if selection["introduction"].NodeID == position.NodeID {
					expected++
				}
			}
			if expected == 0 {
				continue
			}
			root := filepath.Join(t.TempDir(), string(rune('0'+count)), string(rune('0'+ordinal)))
			path := filepath.Join(root, "route", "plans", service+".json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			base := map[string]any{"UpstreamPin": "pin", "NextNodeID": "node", "NextPin": "pin",
				"IntroductionSetupSocket": "stale"}
			if err := byteio.WriteJSON(path, base, 64<<10); err != nil {
				t.Fatal(err)
			}
			if err := configureNodeReplacementPlan(root, candidates, service, selections[:count],
				"client-peer", [32]byte{99}, "30s"); err != nil {
				t.Fatalf("configure %s with %d active proposals: %v", service, count, err)
			}
			raw, err := byteio.ReadFile(path, 64<<10)
			if err != nil {
				t.Fatal(err)
			}
			var plan struct {
				AttachmentPlans         []json.RawMessage
				IntroductionSetupSocket string
			}
			if err := json.Unmarshal(raw, &plan); err != nil {
				t.Fatal(err)
			}
			if len(plan.AttachmentPlans) != expected {
				t.Fatalf("%s active proposals=%d; local plan has %d", service, expected, len(plan.AttachmentPlans))
			}
			fresh := count > 2 && service == "introduction-3"
			if (plan.IntroductionSetupSocket != "") != fresh {
				t.Fatalf("%s count=%d stale/fresh setup mismatch", service, count)
			}
		}
	}
}

func replacementPlanTestCandidates() []route.Position {
	candidates := make([]route.Position, 0, 12)
	for generation := range 3 {
		for roleIndex, role := range replacementRoles {
			candidates = append(candidates, route.Position{Role: role,
				NodeID:    [32]byte{byte(1 + generation*4 + roleIndex)},
				PublicKey: [32]byte{byte(31 + generation*4 + roleIndex)}, Endpoint: role})
		}
	}
	return candidates
}

func TestReplacementSelectionsRejectCandidateExhaustion(t *testing.T) {
	candidates := make([]route.Position, 0, 4)
	for index, role := range []string{"initiator", "introduction", "rendezvous", "responder"} {
		candidates = append(candidates, route.Position{Role: role, NodeID: [32]byte{byte(index + 1)}})
	}
	if _, err := replacementSelections(candidates); err == nil {
		t.Fatal("replacement without an alternate candidate was accepted")
	}
}

func TestReplacementProposalCountMatchesEachRetainedCell(t *testing.T) {
	tests := map[string]int{"isolated-initiator": 2, "isolated-introduction": 2,
		"isolated-rendezvous": 3, "isolated-responder": 2, "sequential-three": 4}
	for mode, want := range tests {
		got, err := replacementProposalCount(mode)
		if err != nil || got != want {
			t.Fatalf("mode %s proposal count=%d err=%v; want %d", mode, got, err, want)
		}
	}
	if _, err := replacementProposalCount("unknown"); err == nil {
		t.Fatal("unknown replacement mode was accepted")
	}
}

func TestReplacementPlanContainsOnlyActiveProposals(t *testing.T) {
	selections := make([]selectedRoute, 4)
	for _, count := range []int{2, 3, 4} {
		bounded, err := boundedReplacementSelections(selections, count)
		if err != nil || len(bounded) != count {
			t.Fatalf("active proposal count %d produced %d selections: %v", count, len(bounded), err)
		}
	}
	for _, count := range []int{0, 1, 5} {
		if _, err := boundedReplacementSelections(selections, count); err == nil {
			t.Fatalf("invalid active proposal count %d was accepted", count)
		}
	}
}

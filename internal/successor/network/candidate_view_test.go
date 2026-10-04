package network

import (
	"reflect"
	"testing"
	"time"
)

func candidateFixture() (CandidatePolicy, CandidateInput) {
	now := time.Unix(1_800_000_000, 0).UTC()
	return CandidatePolicy{Network: [32]byte{1}, ValidFrom: now, Profile: "ardents-route-v3"},
		CandidateInput{Authentication: AuthenticatedRecord, Network: [32]byte{1}, Node: [32]byte{2}, Key: [32]byte{3},
			ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), Family: "family", Endpoint: "endpoint",
			Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 17}
}

func TestCandidateRejectionsPreservePrecedence(t *testing.T) {
	policy, valid := candidateFixture()
	for _, test := range []struct {
		name   string
		alter  func(*CandidateInput)
		reason uint16
	}{
		{"malformed", func(in *CandidateInput) { in.Authentication = MalformedRecord; in.Network = [32]byte{} }, 1},
		{"wrong Network before signature", func(in *CandidateInput) { in.Network = [32]byte{9}; in.Authentication = InvalidRecordSignature }, 2},
		{"signature before expiry", func(in *CandidateInput) { in.Authentication = InvalidRecordSignature; in.ValidUntil = policy.ValidFrom }, 3},
		{"exclusive expiry before capability", func(in *CandidateInput) { in.ValidUntil = policy.ValidFrom; in.Capability = 1 }, 4},
		{"future record", func(in *CandidateInput) { in.ValidFrom = policy.ValidFrom.Add(time.Second) }, 4},
		{"capability before capacity", func(in *CandidateInput) { in.Capability = 1; in.Capacity = 0 }, 5},
		{"capacity before Carrier", func(in *CandidateInput) { in.Capacity = 1025; in.Carrier = "unknown" }, 6},
		{"zero capacity", func(in *CandidateInput) { in.Capacity = 0 }, 6},
		{"Carrier before authority collision", func(in *CandidateInput) { in.Carrier = "ardents-carrier-quic-v1" }, 12},
		{"authority collision", func(in *CandidateInput) {}, 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.alter(&input)
			context := policy
			if test.reason == 7 || test.reason == 12 {
				context.AuthorityKeyIDs = [][32]byte{input.Key}
			}
			view, err := EvaluateCandidates(context, []CandidateInput{input})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(view.Rejections(), []CandidateRejection{{Index: 0, Reason: test.reason}}) || len(view.AcceptedIndices()) != 0 || view.Summary() != (CandidateSummary{}) {
				t.Fatalf("rejection = %v, summary = %+v", view.Rejections(), view.Summary())
			}
		})
	}
}

func TestCandidateCollisionsExcludeAllEligibleParticipants(t *testing.T) {
	policy, first := candidateFixture()
	for _, test := range []struct {
		name    string
		collide func(*CandidateInput)
		reason  uint16
	}{
		{"Node before key and endpoint", func(in *CandidateInput) { in.Node = first.Node; in.Key = first.Key; in.Endpoint = first.Endpoint }, 8},
		{"key before endpoint", func(in *CandidateInput) { in.Key = first.Key; in.Endpoint = first.Endpoint }, 10},
		{"endpoint", func(in *CandidateInput) { in.Endpoint = first.Endpoint }, 11},
	} {
		t.Run(test.name, func(t *testing.T) {
			second := first
			second.Node = [32]byte{4}
			second.Key = [32]byte{5}
			second.Endpoint = "second"
			test.collide(&second)
			for _, inputs := range [][]CandidateInput{{first, second}, {second, first}} {
				view, err := EvaluateCandidates(policy, inputs)
				if err != nil {
					t.Fatal(err)
				}
				if len(view.AcceptedIndices()) != 0 || !reflect.DeepEqual(view.Rejections(), []CandidateRejection{{0, test.reason}, {1, test.reason}}) {
					t.Fatalf("collision disposition = %v", view.Rejections())
				}
			}
			second.Authentication = InvalidRecordSignature
			view, err := EvaluateCandidates(policy, []CandidateInput{first, second})
			if err != nil || !reflect.DeepEqual(view.AcceptedIndices(), []int{0}) || !reflect.DeepEqual(view.Rejections(), []CandidateRejection{{1, 3}}) {
				t.Fatalf("untrusted input poisoned valid member: %v, %v, %v", view.AcceptedIndices(), view.Rejections(), err)
			}
		})
	}
}

func TestCandidateViewOwnsOrderingAndFamilySummaries(t *testing.T) {
	policy, first := candidateFixture()
	second := first
	second.Node = [32]byte{4}
	second.Key = [32]byte{5}
	second.Endpoint = "second"
	second.Capacity = 9
	second.Carrier = "ardents-carrier-quic-v2"
	third := second
	third.Node = [32]byte{6}
	third.Key = [32]byte{7}
	third.Endpoint = "third"
	third.Family = "other"
	third.Capacity = 3
	inputs := []CandidateInput{third, second, first}
	view, err := EvaluateCandidates(policy, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(view.AcceptedIndices(), []int{2, 1, 0}) || view.Summary() != (CandidateSummary{Count: 3, Capacity: 29, FamilyCount: 2, MaxFamilyCount: 2, MaxFamilyCapacity: 26}) {
		t.Fatalf("view = %v, %+v", view.AcceptedIndices(), view.Summary())
	}
	domains := []string{"alpha", "beta"}
	assigned, err := view.DomainSummaries(policy.Network, 7, [32]byte{2}, domains)
	if err != nil {
		t.Fatal(err)
	}
	familyDomain, err := AssignRoleDomain(policy.Network, 7, [32]byte{2}, first.Family, domains)
	if err != nil || assigned[familyDomain][0] < 2 || assigned[familyDomain][1] < 26 {
		t.Fatalf("family split across domains: %v (%v)", assigned, err)
	}
	var count, capacity uint32
	for _, summary := range assigned {
		count += summary[0]
		capacity += summary[1]
	}
	if len(assigned) != 2 || count != 3 || capacity != 29 {
		t.Fatalf("domain totals = %v", assigned)
	}
	view.AcceptedIndices()[0] = 99
	inputs[2].Family = "mutated"
	inputs[2].Capacity = 1024
	assigned[familyDomain] = [2]uint32{}
	again, err := view.DomainSummaries(policy.Network, 7, [32]byte{2}, domains)
	if err != nil || again[familyDomain][1] < 26 || view.AcceptedIndices()[0] != 2 {
		t.Fatal("caller mutation changed the View")
	}
}

func TestCandidateContextBounds(t *testing.T) {
	policy, _ := candidateFixture()
	if _, err := EvaluateCandidates(policy, make([]CandidateInput, 65)); err == nil {
		t.Fatal("unbounded input admitted")
	}
	policy.Profile = "unknown"
	if _, err := EvaluateCandidates(policy, nil); err == nil {
		t.Fatal("unknown profile admitted")
	}
}

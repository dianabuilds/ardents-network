package deployment

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRolePlansPreserveSeparateGrammarAndPaths(t *testing.T) {
	network := NodeNetworkInputs{Root: "/state", NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{3}, ClockObservationFile: "/clock"}
	join := JoinPlan{Schema: "ardents-join-runtime-v1", Network: network, NodeID: [32]byte{4}, ProcessProfile: "ardents-join-process-v1", ParticipationRoot: "/participation", SpendRoot: "/spend", HostingRoot: "/hosting", Certificate: "/certificate", PrivateKey: "/key", RetirementWaitMS: 1000}
	forward := ForwardingPlan(join)
	forward.Schema, forward.ProcessProfile = "ardents-forwarding-runtime-v1", "ardents-forwarding-process-v1"
	descriptor := DescriptorPlan{Schema: "ardents-descriptor-runtime-v1", Network: network, NodeID: join.NodeID, ProcessProfile: "ardents-descriptor-process-v1", ParticipationRoot: join.ParticipationRoot, SpendRoot: join.SpendRoot, HostingRoot: join.HostingRoot, DescriptorRoot: "/descriptor", Certificate: join.Certificate, PrivateKey: join.PrivateKey, RetirementWaitMS: join.RetirementWaitMS}
	introduction := IntroductionPlan{Schema: "ardents-introduction-runtime-v1", Network: network, NodeID: join.NodeID, ProcessProfile: "ardents-introduction-process-v1", ParticipationRoot: join.ParticipationRoot, SpendRoot: join.SpendRoot, HostingRoot: join.HostingRoot, IntroductionRoot: "/introduction", Certificate: join.Certificate, PrivateKey: join.PrivateKey, RetirementWaitMS: join.RetirementWaitMS}
	cases := []struct {
		name   string
		value  any
		decode func([]byte) error
	}{
		{"join", join, func(b []byte) error { _, err := DecodeJoinPlan(b); return err }},
		{"forwarding", forward, func(b []byte) error { _, err := DecodeForwardingPlan(b); return err }},
		{"descriptor", descriptor, func(b []byte) error { _, err := DecodeDescriptorPlan(b); return err }},
		{"introduction", introduction, func(b []byte) error { _, err := DecodeIntroductionPlan(b); return err }},
	}
	for i, tc := range cases {
		raw, err := json.Marshal(tc.value)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, '\n')
		if err := tc.decode(raw); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		for j, other := range cases {
			if i != j && !errors.Is(other.decode(raw), ErrNodePlan) {
				t.Fatalf("%s accepted as %s", tc.name, other.name)
			}
		}
		for _, changed := range [][]byte{raw[:len(raw)-1], []byte(strings.Replace(string(raw), `"/spend"`, `"/state/spend"`, 1)), []byte(strings.Replace(string(raw), `"threshold":1`, `"threshold":2`, 1))} {
			if !errors.Is(tc.decode(changed), ErrNodePlan) {
				t.Fatalf("%s accepted invalid bytes or boundaries", tc.name)
			}
		}
	}
}

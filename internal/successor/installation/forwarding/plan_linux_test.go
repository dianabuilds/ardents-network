package forwarding

import (
	"fmt"
	"strings"
	"testing"
)

func TestInstalledForwardingPlanIndependentCanonicalVector(t *testing.T) {
	id := func(first byte) string { return fmt.Sprintf("[%d%s]", first, strings.Repeat(",0", 31)) }
	raw := fmt.Sprintf("{\"schema\":\"ardents-forwarding-runtime-v1\",\"network\":{\"root\":\"/var/lib/ardents-node-state\",\"network_id\":%s,\"authorities\":[%s],\"threshold\":1,\"profile_authority\":%s,\"clock_observation_file\":\"/var/lib/ardents-clock/observation\"},\"node_id\":%s,\"process_profile\":\"ardents-forwarding-process-v1\",\"participation_root\":\"/var/lib/ardents-node-participation\",\"spend_root\":\"/var/lib/ardents-node-spend\",\"hosting_root\":\"/var/lib/ardents-node-hosting\",\"certificate\":\"/etc/ardents-node/certificate.pem\",\"private_key\":\"/etc/ardents-node/private-key.pem\",\"work\":{\"tx\":40000000,\"rx\":40000000},\"termination\":{\"tx\":1024,\"rx\":1024},\"retirement_wait_ms\":1000}\n", id(1), id(2), id(3), id(4))
	p, err := DecodePlan([]byte(raw))
	if err != nil {
		t.Fatal("independent plan vector", err)
	}
	r, err := DecodeRequest([]byte(forwardingRequestVector))
	if err != nil || r.MatchPlan(p) != nil {
		t.Fatal("exact future-input binding", err)
	}
	for _, fault := range []struct{ before, after string }{
		{"ardents-forwarding-runtime-v1", "ardents-source-runtime-v1"},
		{"\"process_profile\":", "\"role\":\"issuer\",\"process_profile\":"},
		{"\"node_id\":" + id(4), "\"node_id\":[4]"},
		{"\"network_id\":" + id(1), "\"network_id\":" + id(0)},
		{"\"authorities\":[" + id(2) + "]", "\"authorities\":[" + id(2) + "," + id(2) + "]"},
		{"\"threshold\":1", "\"threshold\":2"},
		{"\"threshold\":1", "\"threshold\":1,\"threshold\":1"},
		{"\"profile_authority\":" + id(3), "\"profile_authority\":" + id(0)},
		{"\"retirement_wait_ms\":1000", "\"retirement_wait_ms\":5001"},
		{"\"certificate\":", "\"Certificate\":"},
	} {
		if _, err := DecodePlan([]byte(strings.Replace(raw, fault.before, fault.after, 1))); err == nil {
			t.Fatal("foreign or noncanonical plan accepted", fault)
		}
	}
	for _, malformed := range []string{strings.TrimSpace(raw), raw + "\n", strings.Repeat("x", (16<<10)+1)} {
		if _, err := DecodePlan([]byte(malformed)); err == nil {
			t.Fatal("unbounded/noncanonical plan accepted")
		}
	}
	for _, change := range []func(*Plan){
		func(p *Plan) { p.Network.Root = "/other-state" }, func(p *Plan) { p.Network.ClockObservationFile = "/other-clock" },
		func(p *Plan) { p.ParticipationRoot = p.SpendRoot }, func(p *Plan) { p.SpendRoot = p.HostingRoot },
		func(p *Plan) { p.HostingRoot = "/new-allowance" }, func(p *Plan) { p.Certificate = "/other-certificate" },
		func(p *Plan) { p.PrivateKey = "/offline-key" }, func(p *Plan) { p.ProcessProfile = "ardents-source-process-v1" },
	} {
		other := p
		change(&other)
		if r.MatchPlan(other) == nil {
			t.Fatal("different runtime declaration matched the original request")
		}
	}
}

package join

import (
	"encoding/json"
	"strings"
	"testing"
)

func independentPlanVector() string {
	word := func(first string) string { return "[" + first + strings.Repeat(",0", 31) + "]" }
	return "{\"schema\":\"ardents-join-runtime-v1\",\"network\":{\"root\":\"/var/lib/ardents-node-state\",\"network_id\":" + word("1") + ",\"authorities\":[" + word("2") + "],\"threshold\":1,\"profile_authority\":" + word("3") + ",\"clock_observation_file\":\"/var/lib/ardents-clock/observation\"},\"node_id\":" + word("4") + ",\"process_profile\":\"ardents-join-process-v1\",\"participation_root\":\"/var/lib/ardents-node-participation\",\"spend_root\":\"/var/lib/ardents-node-spend\",\"hosting_root\":\"/var/lib/ardents-node-hosting\",\"certificate\":\"/etc/ardents-node/certificate.pem\",\"private_key\":\"/etc/ardents-node/private-key.pem\",\"work\":{\"tx\":10,\"rx\":20},\"termination\":{\"tx\":30,\"rx\":40},\"retirement_wait_ms\":5000}\n"
}

func TestPlanCanonicalBytesPurposeAndDeclaredTrustBounds(t *testing.T) {
	vector := independentPlanVector()
	p, err := DecodePlan([]byte(vector))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(p)
	if err != nil || string(append(raw, '\n')) != vector {
		t.Fatal("independent canonical plan", err)
	}
	for _, change := range []struct{ from, to string }{
		{"ardents-join-runtime-v1", "ardents-forwarding-runtime-v1"},
		{"ardents-join-process-v1", "ardents-forwarding-process-v1"},
		{"\"threshold\":1", "\"threshold\":0"}, {"\"threshold\":1", "\"threshold\":2"},
		{"\"retirement_wait_ms\":5000", "\"retirement_wait_ms\":5001"},
		{"\"retirement_wait_ms\":5000", "\"retirement_wait_ms\":0"},
		{"\"work\":", "\"extra\":0,\"work\":"},
		{"/var/lib/ardents-node-state", "../state"},
	} {
		if _, err := DecodePlan([]byte(strings.Replace(vector, change.from, change.to, 1))); err == nil {
			t.Fatal("foreign or noncanonical plan", change)
		}
	}
	for _, mutate := range []func(*Plan){
		func(p *Plan) { p.NodeID = [32]byte{} },
		func(p *Plan) { p.Network.NetworkID = [32]byte{} },
		func(p *Plan) { p.Network.ProfileAuthority = [32]byte{} },
		func(p *Plan) { p.Network.Authorities = nil },
		func(p *Plan) { p.Network.Authorities = [][32]byte{{}} },
		func(p *Plan) { p.Network.Authorities = append(p.Network.Authorities, p.Network.Authorities[0]) },
	} {
		changed, err := DecodePlan([]byte(vector))
		if err != nil {
			t.Fatal(err)
		}
		mutate(&changed)
		raw, _ := json.Marshal(changed)
		if _, err := DecodePlan(append(raw, '\n')); err == nil {
			t.Fatal("missing or repeated declared trust")
		}
	}
	if _, err := DecodePlan([]byte(strings.TrimSpace(vector))); err == nil {
		t.Fatal("missing canonical newline")
	}
}

func TestPlanMatchesOneRequestWithoutGrantingNativeCustody(t *testing.T) {
	r, err := DecodeRequest([]byte(requestVector))
	if err != nil {
		t.Fatal(err)
	}
	p, err := DecodePlan([]byte(independentPlanVector()))
	if err != nil || r.MatchPlan(p) != nil {
		t.Fatal("exact declared installed paths", err)
	}
	for _, mutate := range []func(*Plan){
		func(p *Plan) { p.Network.Root = "/var/lib/foreign-state" },
		func(p *Plan) { p.Network.ClockObservationFile = "/var/lib/foreign-clock" },
		func(p *Plan) { p.ParticipationRoot = "/var/lib/foreign-participation" },
		func(p *Plan) { p.SpendRoot = "/var/lib/foreign-spend" },
		func(p *Plan) { p.HostingRoot = "/var/lib/foreign-allowance" },
		func(p *Plan) { p.Certificate = r.Certificate },
		func(p *Plan) { p.PrivateKey = r.PrivateKey },
		func(p *Plan) { p.ProcessProfile = "ardents-forwarding-process-v1" },
	} {
		changed := p
		mutate(&changed)
		if r.MatchPlan(changed) == nil {
			t.Fatal("foreign runtime declaration matched")
		}
	}
	const output = "/var/lib/join-plan-output"
	r.Plan = output + "/join-plan.json"
	if err := r.CheckPlanPreparationOutput(output, "/var/lib/join-request.json"); err != nil {
		t.Fatal("separate public request and exact plan output", err)
	}
	for _, requestFile := range []string{r.Plan, r.Clock, r.InstallationRoot + "/request", r.InstallationRoot + ".join-initial-journal/request", output + "/request", "/var"} {
		if err := r.CheckPlanPreparationOutput(output, requestFile); err == nil {
			t.Fatal("original public input intersects output or lifetime namespace", requestFile)
		}
	}
	if err := r.CheckPlanPreparationOutput("/var/lib/foreign-output", "/var/lib/join-request.json"); err == nil {
		t.Fatal("foreign declared output")
	}
}

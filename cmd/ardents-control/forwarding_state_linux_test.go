package main

import (
	"reflect"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
)

func forwardingStatePlanFixture() forwarding.Plan {
	return forwarding.Plan{Schema: "ardents-forwarding-runtime-v1", ProcessProfile: "ardents-forwarding-process-v1", NodeID: [32]byte{9},
		Network:           forwarding.NetworkInputs{Root: "/var/lib/node-state", NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{3}, ClockObservationFile: "/etc/ardents/clock"},
		ParticipationRoot: "/var/lib/node-participation", SpendRoot: "/var/lib/node-spend", HostingRoot: "/var/lib/node-hosting", Certificate: "/etc/ardents/node/cert.pem", PrivateKey: "/etc/ardents/node/key.pem"}
}

func TestForwardingStatePreparationConfigOpensOnlySignedState(t *testing.T) {
	p := forwardingStatePlanFixture()
	cfg, err := forwardingPreparationStateConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Root != p.Network.Root || cfg.NetworkID != p.Network.NetworkID || cfg.Clock == nil || !cfg.Now.IsZero() || cfg.AcceptedProfile != "ardents-route-v3" || cfg.ClockObservationFile != p.Network.ClockObservationFile || cfg.LocalRoleStateRoot != "" || cfg.PermitWork != nil || !reflect.ValueOf(cfg.Source).IsZero() {
		t.Fatal("offline State config acquired runtime/Source/participation authority")
	}
	p.Network.Authorities[0][0] = 99
	p.Network.ProfileAuthority[0] = 98
	for _, key := range cfg.Authorities {
		if key[0] != 2 {
			t.Fatal("caller changed retained State pins")
		}
	}
	if cfg.ClosedProfileAuthority[0] != 3 {
		t.Fatal("caller changed retained profile pin")
	}
	for name, change := range map[string]func(*forwarding.Plan){
		"Source schema": func(v *forwarding.Plan) { v.Schema = "ardents-source-runtime-v1" }, "other role": func(v *forwarding.Plan) { v.ProcessProfile = "ardents-issuer-process-v1" },
		"missing node": func(v *forwarding.Plan) { v.NodeID = [32]byte{} }, "missing network": func(v *forwarding.Plan) { v.Network.NetworkID = [32]byte{} },
		"missing profile pin": func(v *forwarding.Plan) { v.Network.ProfileAuthority = [32]byte{} }, "threshold": func(v *forwarding.Plan) { v.Network.Threshold = 2 },
		"duplicate State pin": func(v *forwarding.Plan) {
			v.Network.Authorities = append(v.Network.Authorities, v.Network.Authorities[0])
		},
		"missing State pin": func(v *forwarding.Plan) { v.Network.Authorities = nil }, "zero State pin": func(v *forwarding.Plan) { v.Network.Authorities[0] = [32]byte{} },
		"relative root": func(v *forwarding.Plan) { v.Network.Root = "state" }, "overlapping clock": func(v *forwarding.Plan) { v.Network.ClockObservationFile = v.Network.Root + "/clock" },
		"participation root": func(v *forwarding.Plan) { v.ParticipationRoot = v.Network.Root }, "spend root": func(v *forwarding.Plan) { v.SpendRoot = v.Network.Root + "/spend" },
		"Hosting root": func(v *forwarding.Plan) { v.HostingRoot = "/var/lib" }, "key under State": func(v *forwarding.Plan) { v.PrivateKey = v.Network.Root + "/key" },
	} {
		v := forwardingStatePlanFixture()
		change(&v)
		if _, err := forwardingPreparationStateConfig(v); err == nil {
			t.Fatal("bad offline trust/path declaration admitted", name)
		}
	}
}

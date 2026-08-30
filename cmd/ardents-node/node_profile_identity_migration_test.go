package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodePlanReaderNormalizesDedicatedHostProfile(t *testing.T) {
	certificatePath, keyPath, nodeID := writeRendezvousListenCredential(t)
	rootA := writeNodeProfileInput(t, "source-a.pem", "source A root")
	rootB := writeNodeProfileInput(t, "source-b.pem", "source B root")
	for _, profile := range []string{"ardents-rendezvous-dedicated-host-v1", "h4-5-rendezvous-alpha-v1"} {
		t.Run(profile, func(t *testing.T) {
			plan := nodePlan{
				sourceServerPlan: sourceServerPlan{
					Schema: "ardents-node-plan-v1", StateRoot: t.TempDir(), LocalRoleStateRoot: t.TempDir(),
					NetworkID: strings.Repeat("11", 32), AuthorityPublic: []string{strings.Repeat("12", 32)}, Threshold: 1,
					ServerCertificate: certificatePath, ServerKey: keyPath,
				},
				ClockObservationFile: certificatePath, OrderSeed: strings.Repeat("13", 32),
				SourceClientCertificate: certificatePath, SourceClientKey: keyPath,
				Sources: []nodeSource{
					{Address: "192.0.2.10:48010", ServerName: "source-a.test", Identity: strings.Repeat("14", 32), Family: "source-a", EndpointHandle: "source-a", RootCA: rootA, LeafKeyDigest: strings.Repeat("15", 32)},
					{Address: "192.0.2.11:48011", ServerName: "source-b.test", Identity: strings.Repeat("16", 32), Family: "source-b", EndpointHandle: "source-b", RootCA: rootB, LeafKeyDigest: strings.Repeat("17", 32)},
				},
				NodeID: nodeID, IdentityKey: keyPath, NodeResourceProfile: profile,
				DiagnosticDirectory: t.TempDir(), Rendezvous: &rendezvousPlan{},
			}
			path := filepath.Join(t.TempDir(), "node-plan.json")
			raw, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			runtime, err := readNodePlan(path)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.node.ResourceProfile != "ardents-rendezvous-dedicated-host-v1" {
				t.Fatalf("normalized runtime profile = %q", runtime.node.ResourceProfile)
			}
		})
	}
}

func writeNodeProfileInput(t *testing.T, name, value string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

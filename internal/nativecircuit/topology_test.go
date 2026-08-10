package nativecircuit

import "testing"

func TestC3TopologyHasThreeDataNodesAndSeparateIntroduction(t *testing.T) {
	t.Parallel()
	if len(c3Topology.nodeRoles) != 4 || len(c3Topology.networkRoles) != 6 {
		t.Fatalf("unexpected C3 topology: %+v", c3Topology)
	}
	dataNodes := []string{"user-entry", "rendezvous", "data-service-entry"}
	for _, role := range dataNodes {
		if !containsRole(c3Topology.nodeRoles, role) {
			t.Fatalf("C3 data role %s is missing", role)
		}
	}
	if !containsRole(c3Topology.nodeRoles, "introduction-node") {
		t.Fatal("C3 has no separate Introduction Node")
	}
}

func containsRole(roles []string, wanted string) bool {
	for _, role := range roles {
		if role == wanted {
			return true
		}
	}
	return false
}

package architecture

import (
	"bytes"
	"testing"
)

func TestNativeCircuitPackageHasOneSmallInterface(t *testing.T) {
	t.Parallel()
	assertPackageExports(t, "internal/lab/nativecircuit", "Run", "RunAttached", "RunNegative", "RunRole", "RunWorkload")
}

func TestNativeComposeDeclaresExactRoleTopologyAndCapabilities(t *testing.T) {
	t.Parallel()
	compose := readProjectFile(t, repositoryRoot(t), "lab/carrier/compose.yaml")
	for _, required := range []string{
		`profiles: ["native"]`, "user:", "user-entry:", "user-interior:", "rendezvous:",
		"service-interior:", "data-service-entry:", "service:", "introduction-forwarder:",
		"introduction-node:", "introduction-interior:", "introduction-entry:",
		"user-ue:", "ue-ui:", "ui-rv:", "rv-si:", "si-dse:", "dse-service:",
		"ui-if:", "if-in:", "in-ii:", "ii-ie:", "ie-service:",
		`cap_add: [NET_ADMIN]`, `cap_add: [NET_RAW]`, `network_mode: "service:`,
		"CARRIER_LAB_APPLICATION_IMAGE", "CARRIER_LAB_TOOL_IMAGE", "native-tool-role", "native-role",
	} {
		if !bytes.Contains(compose, []byte(required)) {
			t.Errorf("native Compose contract is missing %q", required)
		}
	}
	if bytes.Count(compose, []byte(`cap_add: [NET_ADMIN]`)) != 11 {
		t.Error("native topology must have one NET_ADMIN-only shaper per role namespace")
	}
	if bytes.Count(compose, []byte(`cap_add: [NET_RAW]`)) != 10 {
		t.Error("native topology must have ten NET_RAW-only per-link capture roles")
	}
	if bytes.Count(compose, []byte("- *native-control")) != 11 {
		t.Error("every native Application role must receive the same read-only lifecycle control mount")
	}
	if bytes.Contains(compose, []byte("network_mode: host")) || bytes.Contains(compose, []byte("SYS_ADMIN")) {
		t.Error("native topology contains an unbounded namespace or capability")
	}
}

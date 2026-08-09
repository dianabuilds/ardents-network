package architecture

import (
	"bytes"
	"testing"
)

func TestCarrierLabIsolationPackageHasOneSmallInterface(t *testing.T) {
	t.Parallel()
	assertPackageExports(t, "internal/harness", "Run", "RunRole")
}

func TestCarrierLabComposeIsolationContract(t *testing.T) {
	t.Parallel()
	compose := readProjectFile(t, repositoryRoot(t), "carrier-lab/compose.yaml")

	required := []string{
		"alpha:", "beta:", `profiles: ["isolation"]`, "${CARRIER_LAB_IMAGE:?}", "adjacency:", "internal: true",
		"read_only: true", "cap_drop:", "- ALL", "no-new-privileges:true",
		`user: "65532:65532"`, "tmpfs:", "cpus:", "mem_limit:", "pids_limit:",
		"type: bind", "read_only: true", "create_host_path: false",
		"${CARRIER_LAB_RUN:?}/configs/alpha.json", "${CARRIER_LAB_RUN:?}/configs/beta.json",
		"${CARRIER_LAB_RUN:?}/evidence-alpha", "${CARRIER_LAB_RUN:?}/evidence-beta",
	}
	for _, value := range required {
		if !bytes.Contains(compose, []byte(value)) {
			t.Errorf("Carrier Lab Compose is missing %q", value)
		}
	}
	isolationStart := bytes.Index(compose, []byte("  alpha:"))
	toolingStart := bytes.Index(compose, []byte("  tracer-alpha:"))
	if isolationStart == -1 || toolingStart == -1 || isolationStart >= toolingStart {
		t.Fatal("Carrier Lab Compose does not separate isolation and tooling services")
	}
	if bytes.Contains(compose[isolationStart:toolingStart], []byte("network_mode:")) {
		t.Error("isolation profile must not join another service or host network namespace")
	}
	for _, forbidden := range []string{
		"ports:", "privileged:", "restart:", "/var/run/docker.sock",
		"default:", "external: true", "ipc: host", "pid: host",
	} {
		if bytes.Contains(compose, []byte(forbidden)) {
			t.Errorf("Carrier Lab Compose contains forbidden isolation setting %q", forbidden)
		}
	}
}

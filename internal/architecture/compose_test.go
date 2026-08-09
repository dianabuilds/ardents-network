package architecture

import (
	"bytes"
	"testing"
)

func TestCarrierLabComposeIsolationContract(t *testing.T) {
	t.Parallel()
	compose := readProjectFile(t, repositoryRoot(t), "compose.carrier-lab.yaml")

	required := []string{
		"alpha:", "beta:", "${CARRIER_LAB_IMAGE:?}", "adjacency:", "internal: true",
		"read_only: true", "cap_drop:", "- ALL", "no-new-privileges:true",
		`user: "65532:65532"`, "tmpfs:", "cpus:", "mem_limit:", "pids_limit:",
		"type: bind", "read_only: true", "create_host_path: false",
		"${ALPHA_CONFIG:?}", "${BETA_CONFIG:?}", "${ALPHA_EVIDENCE:?}", "${BETA_EVIDENCE:?}",
	}
	for _, value := range required {
		if !bytes.Contains(compose, []byte(value)) {
			t.Errorf("compose.carrier-lab.yaml is missing %q", value)
		}
	}
	for _, forbidden := range []string{
		"ports:", "network_mode:", "privileged:", "restart:", "/var/run/docker.sock",
		"default:", "external: true", "ipc: host", "pid: host",
	} {
		if bytes.Contains(compose, []byte(forbidden)) {
			t.Errorf("compose.carrier-lab.yaml contains forbidden isolation setting %q", forbidden)
		}
	}
}

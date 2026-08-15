package architecture

import (
	"bytes"
	"testing"
)

func TestCarrierLabIsolationPackageHasOneSmallInterface(t *testing.T) {
	t.Parallel()
	assertPackageExports(t, "internal/lab/carrier", "Run", "RunRole")
}

func TestCarrierLabComposeIsolationContract(t *testing.T) {
	t.Parallel()
	compose := readProjectFile(t, repositoryRoot(t), "lab/carrier/compose.yaml")

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

func TestH3RouteSmokeComposeContract(t *testing.T) {
	t.Parallel()
	compose := readProjectFile(t, repositoryRoot(t), "tests/qualification/h3-route-v1/compose.yaml")
	required := []string{
		"client:", "initiator:", "introduction:", "rendezvous:", "responder:", "publisher:", "verifier:",
		"${ARDENTS_ROUTE_IMAGE_TAG:?}", "${ARDENTS_ROUTE_ROOT:?}", "route_net:", "internal: true",
		"read_only: true", "cap_drop:", "- ALL", "no-new-privileges:true", "restart: \"no\"",
		"tmpfs:", "cpus:", "mem_limit:", "pids_limit:", "type: bind", "read_only: true", "create_host_path: false",
	}
	for _, value := range required {
		if !bytes.Contains(compose, []byte(value)) {
			t.Errorf("H3 Route smoke Compose is missing %q", value)
		}
	}
	for _, forbidden := range []string{"ports:", "privileged:", "/var/run/docker.sock", "network_mode:", "external: true"} {
		if bytes.Contains(compose, []byte(forbidden)) {
			t.Errorf("H3 Route smoke Compose contains forbidden setting %q", forbidden)
		}
	}
}

func TestH3ServiceSmokeComposeContract(t *testing.T) {
	t.Parallel()
	compose := readProjectFile(t, repositoryRoot(t), "tests/qualification/h3-service-v1/compose.yaml")
	required := []string{
		"client:", "initiator:", "introduction:", "rendezvous:", "responder:", "publisher:",
		"client-endpoint:", "publisher-endpoint:", "client-app:", "publisher-app:", "negative-suite:", "verifier:",
		"${ARDENTS_SERVICE_IMAGE_TAG:?}", "${ARDENTS_SERVICE_RUNTIME_USER:?}", "route_net:", "internal: true",
		"network_mode: none", "read_only: true", "cap_drop: [ALL]", "no-new-privileges:true", "restart: \"no\"",
		"tmpfs:", "cpus:", "mem_limit:", "pids_limit:", "type: bind", "read_only: true", "create_host_path: false",
		"user: \"0:0\"", "cap_add: [CHOWN, FOWNER]",
	}
	for _, value := range required {
		if !bytes.Contains(compose, []byte(value)) {
			t.Errorf("H3 Service smoke Compose is missing %q", value)
		}
	}
	for _, forbidden := range []string{"ports:", "privileged:", "/var/run/docker.sock", "network_mode: host", "external: true"} {
		if bytes.Contains(compose, []byte(forbidden)) {
			t.Errorf("H3 Service smoke Compose contains forbidden setting %q", forbidden)
		}
	}
}

func TestH3Stage3ComposeKeepsCurrentStreamCLIAndFrozenTopology(t *testing.T) {
	t.Parallel()
	compose := readProjectFile(t, repositoryRoot(t), "tests/qualification/h3-service-v1/compose.stage3.yaml")
	for _, command := range []string{
		"run, publisher, /run/ardents/publisher-app/app.sock, /run/ardents/workload/publisher.hex, /run/ardents/workload/client.hex, \"65536\", \"65536\"",
		"run, client, /run/ardents/client-app/app.sock, /run/ardents/workload/client.hex, /run/ardents/workload/publisher.hex, \"65536\", \"65536\"",
	} {
		if !bytes.Contains(compose, []byte(command)) {
			t.Errorf("isolated Stage 3 Compose is missing current stream command %q", command)
		}
	}
	for _, forbidden := range []string{"carrier_net:", "profiles: [s41]", "profiles: [s42]", "profiles: [s43"} {
		if bytes.Contains(compose, []byte(forbidden)) {
			t.Errorf("isolated Stage 3 Compose contains recovery topology %q", forbidden)
		}
	}
}

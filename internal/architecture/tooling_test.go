package architecture

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCarrierLabToolingPackageHasOneSmallInterface(t *testing.T) {
	t.Parallel()
	assertPackageExports(t, "internal/harness/tooling", "NativeImageReceipt", "VerifyInputs", "VerifyNativeImages", "RunSmoke", "RunRole", "RunNativeRole")
}

func TestCarrierLabContainerSourcesHaveFourResponsibilities(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "carrier-lab"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"Dockerfile": true, "compose.yaml": true, "tools.lock": true, "reference.lock": true}
	if len(entries) != len(want) {
		t.Fatalf("carrier-lab source entries = %d, want exactly %d", len(entries), len(want))
	}
	for _, entry := range entries {
		if entry.IsDir() || !want[entry.Name()] {
			t.Errorf("unexpected Carrier Lab container source %q", entry.Name())
		}
	}
	for _, obsolete := range []string{
		"Dockerfile.carrier-lab", "Dockerfile.carrier-lab-tooling",
		"compose.carrier-lab.yaml", "compose.carrier-lab-tooling.yaml", "carrier-lab-tools.lock",
	} {
		if _, err := os.Stat(filepath.Join(root, obsolete)); !os.IsNotExist(err) {
			t.Errorf("obsolete root Docker source remains: %s", obsolete)
		}
	}
}

func TestCarrierLabToolingImageIsOfflineAndContentAddressed(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	dockerfile := readProjectFile(t, root, "carrier-lab/Dockerfile")
	lock := readProjectFile(t, root, "carrier-lab/tools.lock")

	scanner := bufio.NewScanner(bytes.NewReader(dockerfile))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "FROM ") && !strings.HasPrefix(line, "FROM "+carrierLabBase+" AS ") {
			t.Errorf("mutable or unpinned tooling base: %s", line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		" AS tooling", "COPY --from=tool_bundle", "carrier-lab-tools.lock", "sha256sum -c", "dpkg-deb --field", "dpkg-deb --extract",
		"RUN --network=none", "type=bind,from=go_archive", "GOPROXY=off", "CGO_ENABLED=0", "USER 65532:65532",
		"carrier-lab-source.sha256", "org.opencontainers.image.base.digest", `io.ardents.carrier-lab.target="tooling"`,
	} {
		if !bytes.Contains(dockerfile, []byte(required)) {
			t.Errorf("Carrier Lab Dockerfile is missing tooling contract %q", required)
		}
	}
	for _, forbidden := range []string{"apt install", "apt-get", "curl ", "wget ", "go install", "GOTOOLCHAIN=auto"} {
		if bytes.Contains(dockerfile, []byte(forbidden)) {
			t.Errorf("Carrier Lab Dockerfile contains mutable/download behavior %q", forbidden)
		}
	}
	for _, required := range []string{"tool\ttc\t6.19.0", "tool\ttcpdump\t4.99.6", "tool\tlibpcap\t1.10.6", "package\tiproute2\t6.19.0-1ubuntu1.1", "https://archive.ubuntu.com/ubuntu/"} {
		if !bytes.Contains(lock, []byte(required)) {
			t.Errorf("tool identity lock is missing %q", required)
		}
	}
}

func TestCarrierLabToolingComposeIsolationAndCapabilities(t *testing.T) {
	t.Parallel()
	compose := readProjectFile(t, repositoryRoot(t), "carrier-lab/compose.yaml")
	for _, required := range []string{
		"tracer-alpha:", "tracer-beta:", "shape-alpha:", "shape-beta:", "capture-alpha:", `profiles: ["tooling"]`,
		`network_mode: "service:tracer-alpha"`, `network_mode: "service:tracer-beta"`,
		"cap_drop:", "- ALL", "- NET_ADMIN", "- NET_RAW", "internal: true", `user: "65532:65532"`, `user: "0:0"`,
		"read_only: true", "no-new-privileges:true", "create_host_path: false", "target: /capture", "${CARRIER_LAB_RUN:?}/raw-capture",
	} {
		if !bytes.Contains(compose, []byte(required)) {
			t.Errorf("tooling Compose contract is missing %q", required)
		}
	}
	if bytes.Count(compose, []byte("- NET_ADMIN")) != 2 || bytes.Count(compose, []byte("- NET_RAW")) != 1 {
		t.Error("tooling capabilities are not limited to two shapers and one capture role")
	}
	for _, forbidden := range []string{"privileged:", "ports:", "network_mode: host", "SYS_ADMIN", "/var/run/docker.sock", "external: true"} {
		if bytes.Contains(compose, []byte(forbidden)) {
			t.Errorf("tooling Compose contains forbidden isolation setting %q", forbidden)
		}
	}
}

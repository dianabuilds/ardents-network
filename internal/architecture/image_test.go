package architecture

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

const carrierLabBase = "ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960"

func TestCarrierLabImageContract(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	dockerfile := readProjectFile(t, root, "carrier-lab/Dockerfile")
	ignore := readProjectFile(t, root, ".dockerignore")

	var fromLines []string
	scanner := bufio.NewScanner(bytes.NewReader(dockerfile))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "FROM ") {
			fromLines = append(fromLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(fromLines) < 2 {
		t.Fatalf("Dockerfile must be multi-stage; FROM lines = %v", fromLines)
	}
	for _, line := range fromLines {
		if !strings.HasPrefix(line, "FROM "+carrierLabBase+" AS ") {
			t.Errorf("mutable or unpinned base image: %s", line)
		}
	}

	required := []string{
		" AS application", " AS tooling",
		"type=bind,from=go_archive", "source=go1.26.5.linux-amd64.tar.gz", "target=/input/go1.26.5.linux-amd64.tar.gz,ro",
		"5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053",
		"sha256sum -c -", "RUN --network=none", "GOTOOLCHAIN=local", "GOPROXY=off",
		"GOSUMDB=off", "CGO_ENABLED=0", "type=tmpfs,target=/tmp/go-build",
		"type=tmpfs,target=/tmp/go-mod", "-trimpath", "-buildvcs=false", "-buildid=",
		"COPY --from=build --chown=65532:65532 /out/carrier-lab /usr/local/bin/carrier-lab",
		"USER 65532:65532", `ENTRYPOINT ["/usr/local/bin/carrier-lab"]`,
	}
	for _, value := range required {
		if !bytes.Contains(dockerfile, []byte(value)) {
			t.Errorf("Dockerfile is missing %q", value)
		}
	}
	for _, forbidden := range []string{"apt install", "apt-get", "curl ", "wget ", "go install", "FROM ubuntu:", "GOTOOLCHAIN=auto"} {
		if bytes.Contains(dockerfile, []byte(forbidden)) {
			t.Errorf("Dockerfile contains forbidden mutable or download behavior %q", forbidden)
		}
	}
	for _, excluded := range []string{".git", "**/*_test.go", "docs", "experiments"} {
		if !bytes.Contains(ignore, []byte(excluded)) {
			t.Errorf(".dockerignore does not exclude %q", excluded)
		}
	}
}

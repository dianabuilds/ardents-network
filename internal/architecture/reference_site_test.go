package architecture

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestReferenceSiteHasExactlyTwoHumanAuthoredDockerInputs(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "reference-site"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatal("reference-site must not grow a nested deployment tree")
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"Dockerfile", "compose.yaml"}) {
		t.Fatalf("reference-site inputs = %v, want exactly Dockerfile and compose.yaml", names)
	}
	dockerfile := readProjectFile(t, root, "reference-site/Dockerfile")
	compose := readProjectFile(t, root, "reference-site/compose.yaml")
	for _, required := range []string{"COPY --from=go_vendor", "GOFLAGS=-mod=vendor", "RUN --network=none", "go1.26.5", " AS reference"} {
		if !bytes.Contains(dockerfile, []byte(required)) {
			t.Errorf("Reference Site Dockerfile is missing %q", required)
		}
	}
	for _, required := range []string{"http-client:", "http-application:", "administration:", "authority:", "relay:", "gateway:", "network_mode: none", "mem_limit: 512m", "mem_limit: 256m", "internal: true"} {
		if !bytes.Contains(compose, []byte(required)) {
			t.Errorf("Reference Site Compose is missing %q", required)
		}
	}
}

func TestGateCOfficialWorkflowIsPinnedAndConjunctive(t *testing.T) {
	t.Parallel()
	workflow := readProjectFile(t, repositoryRoot(t), ".github/workflows/gate-c.yml")
	for _, required := range []string{
		"workflow_dispatch:", "runs-on: ubuntu-26.04", "go-version: 1.26.5", "make check", "go mod verify",
		"--network=none", "route-experiment", "reference-site-lab run", `value.get("decision") != "advance"`, "positive_passed",
	} {
		if !bytes.Contains(workflow, []byte(required)) {
			t.Errorf("Gate C workflow is missing official-run control %q", required)
		}
	}
	controller := readProjectFile(t, repositoryRoot(t), "internal/siteexperiment/controller_docker.go")
	for _, required := range []string{"--no-build", "--pull", "never"} {
		if !bytes.Contains(controller, []byte(required)) {
			t.Errorf("Gate C controller is missing immutable-run control %q", required)
		}
	}
	assertPinnedActions(t, workflow)
}

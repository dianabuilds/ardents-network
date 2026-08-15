package architecture

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func assertQualityWiring(t *testing.T, root string) {
	t.Helper()
	makefile := readProjectFile(t, root, "Makefile")
	for _, required := range []string{
		"unit:", "e2e:", "live:",
		"quick-check: format-check vet unit build mod-check",
		"check: tools-check quick-check e2e lab-test test-race staticcheck vuln",
		"GOTOOLCHAIN := local",
		"GOMODCACHE := $(QUALITY_CACHE_ROOT)/go-mod",
	} {
		if !bytes.Contains(makefile, []byte(required)) {
			t.Errorf("Makefile is missing mandatory quality control %q", required)
		}
	}
	dockerRecipe := regexp.MustCompile(`(?m)^\t[^\n]*\bdocker([[:space:]]|$)`)
	if dockerRecipe.Match(makefile) {
		t.Error("ordinary Make quality gates must not build or run Docker; Carrier Lab qualification is explicit after source freeze")
	}
	hook := readProjectFile(t, root, ".githooks/pre-commit")
	if !bytes.Contains(hook, []byte("exec make quick-check")) {
		t.Error("pre-commit hook must run make quick-check")
	}
	workflow := readProjectFile(t, root, ".github/workflows/quality.yml")
	goVersion := moduleGoVersion(t, root)
	for _, required := range []string{"contents: read", "go-version: " + goVersion, "run: make check"} {
		if !bytes.Contains(workflow, []byte(required)) {
			t.Errorf("CI workflow is missing mandatory quality control %q", required)
		}
	}
	liveWorkflow := readProjectFile(t, root, ".github/workflows/live.yml")
	for _, required := range []string{"contents: read", "go-version-file: go.mod", "run: make live"} {
		if !bytes.Contains(liveWorkflow, []byte(required)) {
			t.Errorf("live workflow is missing mandatory control %q", required)
		}
	}
	carrierWorkflow := readProjectFile(t, root, ".github/workflows/carrier-lab.yml")
	for _, required := range []string{"workflow_dispatch:", "runs-on: ubuntu-26.04", "route-experiment", "--network=none", "ardents-experiment-session.", "experiment-verdict.json"} {
		if !bytes.Contains(carrierWorkflow, []byte(required)) {
			t.Errorf("Carrier Lab workflow is missing qualification control %q", required)
		}
	}
	assertPinnedActions(t, workflow)
	assertPinnedActions(t, liveWorkflow)
	assertPinnedActions(t, carrierWorkflow)
}

func moduleGoVersion(t *testing.T, root string) string {
	t.Helper()
	for _, line := range strings.Split(string(readProjectFile(t, root, "go.mod")), "\n") {
		if version, found := strings.CutPrefix(strings.TrimSpace(line), "go "); found {
			return version
		}
	}
	t.Fatal("go.mod is missing its Go version")
	return ""
}

func assertPinnedActions(t *testing.T, workflow []byte) {
	t.Helper()
	actionPin := regexp.MustCompile(`^[[:space:]]*uses:[[:space:]]+[^@[:space:]]+@[0-9a-f]{40}([[:space:]]+#.*)?$`)
	scanner := bufio.NewScanner(bytes.NewReader(workflow))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "uses:") && !actionPin.MatchString(line) {
			t.Errorf("GitHub Action must be pinned to a full commit SHA: %s", strings.TrimSpace(line))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan CI workflow: %v", err)
	}
}

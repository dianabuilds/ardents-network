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
		"unit:", "e2e:",
		"package-e2e:",
		"-tags packagee2e ./tests/e2e/endpoint",
		"$(MAKE) --output-sync=target package-e2e",
		"QUICK_CHECK_TARGETS := vet unit build mod-check",
		"$(MAKE) --output-sync=target -j 4 $(QUICK_CHECK_TARGETS)",
		"$(MAKE) --output-sync=target -j 4 $(QUICK_CHECK_TARGETS) staticcheck vuln",
		"$(MAKE) --output-sync=target e2e",
		"check: check-core",
		"$(MAKE) --output-sync=target test-race",
		"RACE_TEST_FLAGS := -short -race -shuffle=on -count=1 -timeout=15m",
		"go test -p 1 $(UNIT_PACKAGES) $(RACE_TEST_FLAGS)",
		"UNIT_TEST_FLAGS := -short -shuffle=on -count=1 -timeout=15m",
		"go test -p 1 $(UNIT_PACKAGES) $(UNIT_TEST_FLAGS)",
		"go test -p 1 $$packages $(RACE_TEST_FLAGS)",
		"race-shard-packages RACE_SHARD=$(RACE_SHARD)",
		`case "$(RACE_SHARD)" in 0|1|2|3)`,
		`assigned = ($$0 ~ /\/internal\/custody$$/) ? 3 : (NR-1)%4`,
		"GOTOOLCHAIN := local",
		"GOMODCACHE := $(QUALITY_CACHE_ROOT)/go-mod",
	} {
		if !bytes.Contains(makefile, []byte(required)) {
			t.Errorf("Makefile is missing mandatory quality control %q", required)
		}
	}
	parallelRuntimeSuites := regexp.MustCompile(`(?m)^\t[^\n]*-j[^\n]*(e2e[^\n]*test-race|test-race[^\n]*e2e)`)
	if parallelRuntimeSuites.Match(makefile) {
		t.Error("make check must not race wall-clock e2e against the race suite")
	}
	dockerRecipe := regexp.MustCompile(`(?m)^\t[^\n]*\bdocker([[:space:]]|$)`)
	if dockerRecipe.Match(makefile) {
		t.Error("ordinary Make quality gates must not build or run Docker; Carrier Lab qualification is explicit after source freeze")
	}
	hook := readProjectFile(t, root, ".githooks/pre-commit")
	if !bytes.Contains(hook, []byte("exec git diff --cached --check")) {
		t.Error("pre-commit hook must check the staged diff")
	}
	workflow := readProjectFile(t, root, ".github/workflows/quality.yml")
	goVersion := moduleGoVersion(t, root)
	for _, required := range []string{"contents: read", "go-version: " + goVersion, "cache: true", "go run ./scripts/select-pr-checks.go ./scripts/select-pr-check-registry.go --base", "fail-fast: false", "-timeout=5m", "330s go test", "fetch-depth: 0", "qualification-scripts:", `test "$(git rev-parse HEAD^2)" = "$PR_HEAD"`, `PR_BASE="$(git rev-parse HEAD^1)"`, "Management.Automation.Language.Parser", "needs.select.outputs.powershell == 'true'", "workflow_dispatch:",
		`run: make check-core UNIT_TEST_FLAGS="-shuffle=on -count=1 -timeout=15m"`, "shard: [0, 1, 2, 3]", "run: make test-race-shard RACE_SHARD=",
		"make --no-print-directory race-shard-packages RACE_SHARD=", "make tools-install DIAGNOSTIC_PARSERS_ONLY=1",
		`RACE_TEST_FLAGS="-race -shuffle=on -count=1 -timeout=15m"`, "--full-gate-matrix",
		"needs: [select, qualification-scripts, affected, full, full-race, portable]",
		"RACE_RESULT: ${{ needs.full-race.result }}", `test "$RACE_RESULT" = success`,
		"PORTABLE_RESULT: ${{ needs.portable.result }}", `test "$PORTABLE_RESULT" = success`,
		`go test -p 4 "${packages[@]}" -short -shuffle=on -count=1 -timeout=15m`,
		"GOCACHE: /tmp/ardents-network-quality/go-build", "GOMODCACHE: /tmp/ardents-network-quality/go-mod"} {
		if !bytes.Contains(workflow, []byte(required)) {
			t.Errorf("CI workflow is missing mandatory quality control %q", required)
		}
	}
	if bytes.Contains(workflow, []byte("  push:\n")) {
		t.Error("quality workflow must not repeat the protected PR gate after merge")
	}
	assertPinnedActions(t, workflow)
	selection := readProjectFile(t, root, "docs/development/ownership.json")
	for _, owner := range []string{"tests/qualification/worker-lifecycle/stream-worker", "internal/application/streamqualification"} {
		if !bytes.Contains(selection, []byte(owner)) {
			t.Errorf("PR ownership registry omits qualification packaging consumer %s", owner)
		}
	}
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

// TestPullRequestQualityRequiresTheFullGate keeps the fast local hook tied to
// the actual PR gate, including both race shards and the portable matrix.
func TestPullRequestQualityRequiresTheFullGate(t *testing.T) {
	workflow := string(readProjectFile(t, repositoryRoot(t), ".github/workflows/quality.yml"))
	for _, job := range []string{"full", "full-race"} {
		start := strings.Index(workflow, "  "+job+":\n")
		if start < 0 {
			t.Fatalf("missing CI job %s", job)
		}
		rest := workflow[start+len("  "+job+":\n"):]
		end := regexp.MustCompile(`(?m)^  [a-z][a-z-]*:\s*$`).FindStringIndex(rest)
		if end == nil {
			t.Fatalf("cannot locate end of CI job %s", job)
		}
		if regexp.MustCompile(`(?m)^    if:`).MatchString(rest[:end[0]]) {
			t.Errorf("CI job %s must run for pull requests", job)
		}
	}
	start := strings.Index(workflow, "          if [[ \"$EVENT\" == pull_request ]]; then\n")
	if start < 0 {
		t.Fatal("missing pull request quality decision")
	}
	rest := workflow[start:]
	end := strings.Index(rest, "          else\n")
	if end < 0 {
		t.Fatal("missing non-pull-request quality decision")
	}
	pullRequest := rest[:end]
	for _, check := range []string{`test "$FULL_RESULT" = success`, `test "$RACE_RESULT" = success`} {
		if !strings.Contains(pullRequest, check) {
			t.Errorf("pull request must require %s", check)
		}
	}
	if !strings.Contains(rest[end:], `test "$PORTABLE_RESULT" = success`) {
		t.Error("quality gate must require the portable matrix")
	}
}

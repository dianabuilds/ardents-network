package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditTraceRejectsMissingP1Finding(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "backlog.md", `
| Audit ID | Jira-like key | Epic | Priority |
|---|---|---|---|
| REL-001 | ARD-007 | EPIC-02 | P1 |
| SEC-002 | ARD-005 | EPIC-02 | P1 |
`)
	writeFixture(t, root, "ci.yml", "jobs:\n  critical-lifecycle:\n    steps:\n      - run: go test ./internal/ingressproxy\n")
	writeFixture(t, root, "internal/ingressproxy/proxy.go", "package ingressproxy\n")
	writeFixture(t, root, "internal/ingressproxy/proxy_test.go", "package ingressproxy\n\nfunc TestConnectionResetDoesNotStopIngressListener() {}\n")
	writeValidManifest(t, root)

	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-root", root,
		"-manifest", "manifest.json",
		"-backlog", "backlog.md",
		"-workflow", "ci.yml",
	}, &stdout, &stderr)
	require.ErrorContains(t, err, "P1 finding SEC-002 is missing")
}

func TestAuditTraceRejectsMissingGoTestSymbol(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "backlog.md", "| REL-001 | ARD-007 | EPIC-02 | P1 |\n")
	writeFixture(t, root, "ci.yml", "jobs:\n  critical-lifecycle:\n    steps:\n      - run: go test ./internal/ingressproxy\n")
	writeFixture(t, root, "internal/ingressproxy/proxy.go", "package ingressproxy\n")
	writeFixture(t, root, "internal/ingressproxy/proxy_test.go", "package ingressproxy\n\nfunc TestListenerSurvivesReset() {}\n")
	writeValidManifest(t, root)

	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-root", root,
		"-manifest", "manifest.json",
		"-backlog", "backlog.md",
		"-workflow", "ci.yml",
	}, &stdout, &stderr)
	require.ErrorContains(t, err, "TestConnectionResetDoesNotStopIngressListener is not declared")
}

func TestAuditTraceRejectsChangedCriticalFileWithoutEvidence(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "backlog.md", "| REL-001 | ARD-007 | EPIC-02 | P1 |\n")
	writeFixture(t, root, "ci.yml", "jobs:\n  critical-lifecycle:\n    steps:\n      - run: go test ./internal/ingressproxy\n")
	writeFixture(t, root, "internal/ingressproxy/proxy.go", "package ingressproxy\n")
	writeFixture(t, root, "internal/ingressproxy/proxy_test.go", "package ingressproxy\n\nfunc TestConnectionResetDoesNotStopIngressListener() {}\n")
	writeValidManifest(t, root)
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "audittrace@example.invalid")
	runGit(t, root, "config", "user.name", "Audit Trace Test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "baseline")
	writeFixture(t, root, "internal/ingressproxy/transport/process.go", "package transport\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "unmapped critical change")

	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-root", root,
		"-manifest", "manifest.json",
		"-backlog", "backlog.md",
		"-workflow", "ci.yml",
		"-base", "HEAD^",
	}, &stdout, &stderr)
	require.ErrorContains(t, err, "changed critical file internal/ingressproxy/transport/process.go is not mapped")
}

func TestAuditTraceRejectsGateThatDoesNotRunEvidence(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "backlog.md", "| REL-001 | ARD-007 | EPIC-02 | P1 |\n")
	writeFixture(t, root, "ci.yml", "jobs:\n  critical-lifecycle:\n    steps:\n      - run: go test ./internal/content\n      - run: go run ./internal/ingressproxy\n")
	writeFixture(t, root, "internal/ingressproxy/proxy.go", "package ingressproxy\n")
	writeFixture(t, root, "internal/ingressproxy/proxy_test.go", "package ingressproxy\n\nfunc TestConnectionResetDoesNotStopIngressListener() {}\n")
	writeValidManifest(t, root)

	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-root", root,
		"-manifest", "manifest.json",
		"-backlog", "backlog.md",
		"-workflow", "ci.yml",
	}, &stdout, &stderr)
	require.ErrorContains(t, err, "CI gate does not run Go test package ./internal/ingressproxy")
}

func TestAuditTraceRejectsGateThatDoesNotExecuteCIContract(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "backlog.md", "| OPS-001 | ARD-003 | EPIC-01 | P1 |\n")
	writeFixture(t, root, "ci.yml", "jobs:\n  deployment:\n    steps:\n      - run: ./tests/ci/other-gate.ps1\n")
	writeFixture(t, root, "scripts/deploy/rollout.ps1", "Write-Host rollout\n")
	writeFixture(t, root, "tests/ci/deployment-gate.ps1", "throw 'injected backup failure'\n")
	writeFixture(t, root, "tests/ci/other-gate.ps1", "Write-Host other\n")
	writeFixture(t, root, "manifest.json", `{
  "schema_version": 1,
  "critical_patterns": ["scripts/deploy/*.ps1"],
  "gates": [{"id":"deployment","ci_job":"deployment"}],
  "findings": [{
    "audit_id":"OPS-001",
    "issue":"ARD-003",
    "priority":"P1",
    "critical_files":["scripts/deploy/rollout.ps1"],
    "evidence":[{
      "kind":"ci_contract",
      "file":"tests/ci/deployment-gate.ps1",
      "name":"backup failure contract",
      "match":"injected backup failure",
      "gate":"deployment",
      "covers":["scripts/deploy/rollout.ps1"]
    }]
  }]
}`)

	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-root", root,
		"-manifest", "manifest.json",
		"-backlog", "backlog.md",
		"-workflow", "ci.yml",
	}, &stdout, &stderr)
	require.ErrorContains(t, err, "CI gate does not execute contract tests/ci/deployment-gate.ps1")
}

func writeValidManifest(t *testing.T, root string) {
	t.Helper()
	writeFixture(t, root, "manifest.json", `{
  "schema_version": 1,
  "critical_patterns": ["internal/ingressproxy/**"],
  "gates": [{"id":"critical-lifecycle","ci_job":"critical-lifecycle"}],
  "findings": [{
    "audit_id":"REL-001",
    "issue":"ARD-007",
    "priority":"P1",
    "critical_files":["internal/ingressproxy/proxy.go"],
    "evidence":[{
      "kind":"go_test",
      "file":"internal/ingressproxy/proxy_test.go",
      "name":"TestConnectionResetDoesNotStopIngressListener",
      "gate":"critical-lifecycle",
      "covers":["internal/ingressproxy/proxy.go"]
    }]
  }]
}`)
}

func writeFixture(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "git %v: %s", args, output)
}

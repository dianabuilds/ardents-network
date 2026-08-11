package nativecircuit

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestCandidateFailureKindIsCauseExact(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "contract", err: candidateContractFailure("binding mismatch"), want: "scenario"},
		{name: "downstream eof", err: candidatePeerReadFailure(io.EOF), want: "downstream"},
		{name: "downstream closed", err: candidatePeerReadFailure(net.ErrClosed), want: "downstream"},
		{name: "operational", err: errors.New("random source failed"), want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := candidateFailureKind(test.err); got != test.want {
				t.Fatalf("kind=%q, want %q", got, test.want)
			}
		})
	}
}

func TestNativeFailureKindRequiresTypedContractFailureAndCleanCleanup(t *testing.T) {
	contractErr := nativeContractFailure(errors.New("workload mismatch"))
	if kind := nativeFailureKind(contractErr, nil); kind != "scenario" {
		t.Fatalf("contract failure kind=%q", kind)
	}
	if kind := nativeFailureKind(errors.New("docker inspect failed"), nil); kind != "operational" {
		t.Fatalf("Docker failure kind=%q", kind)
	}
	if kind := nativeFailureKind(contractErr, errors.New("cleanup failed")); kind != "operational" {
		t.Fatalf("mixed contract/cleanup failure kind=%q", kind)
	}
}

func TestAttachedRoleScenarioFailureUsesTypedRoleEvidence(t *testing.T) {
	fixture := nativeFixture{runID: "active-run", roleEvidence: make(map[string]string)}
	for _, role := range []string{"user", "service"} {
		directory := filepath.Join(t.TempDir(), role)
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		fixture.roleEvidence[role] = directory
	}
	write := func(role, kind string) {
		t.Helper()
		if err := writeRoleJSON(filepath.Join(fixture.roleEvidence[role], "result.json"), map[string]any{
			"schema_version": nativeEvidenceSchema, "run_id": fixture.runID, "role": role,
			"status": "failed", "terminal_result": "explicit_failure", "failure_kind": kind,
		}); err != nil {
			t.Fatal(err)
		}
	}
	write("user", "scenario")
	write("service", "scenario")
	scenario, err := attachedRoleScenarioFailure(fixture)
	if err != nil || !scenario {
		t.Fatalf("typed scenario evidence not recognized: scenario=%t err=%v", scenario, err)
	}
	write("service", "operational")
	scenario, err = attachedRoleScenarioFailure(fixture)
	if err != nil || scenario {
		t.Fatalf("mixed scenario/operational failure misclassified: scenario=%t err=%v", scenario, err)
	}
	write("service", "downstream")
	scenario, err = attachedRoleScenarioFailure(fixture)
	if err != nil || !scenario {
		t.Fatalf("scenario plus verified downstream close not recognized: scenario=%t err=%v", scenario, err)
	}
	if err := os.Remove(filepath.Join(fixture.roleEvidence["service"], "result.json")); err != nil {
		t.Fatal(err)
	}
	scenario, err = attachedRoleScenarioFailure(fixture)
	if err != nil || scenario {
		t.Fatalf("scenario plus missing peer misclassified: scenario=%t err=%v", scenario, err)
	}
	write("service", "scenario")
	write("user", "unknown")
	if _, err := attachedRoleScenarioFailure(fixture); err == nil {
		t.Fatal("invalid role failure evidence was accepted")
	}
	write("user", "scenario")
	data, err := os.ReadFile(filepath.Join(fixture.roleEvidence["user"], "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.roleEvidence["service"], "result.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := attachedRoleScenarioFailure(fixture); err == nil {
		t.Fatal("swapped role evidence was accepted")
	}
}

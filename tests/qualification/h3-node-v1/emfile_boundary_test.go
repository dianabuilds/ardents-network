package state_test

import (
	"os"
	"strings"
	"testing"
)

func TestEMFILEInjectorDoesNotShareTargetPIDNamespace(t *testing.T) {
	raw, err := os.ReadFile("compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	compose := string(raw)
	start := strings.Index(compose, "  emfile_harness:")
	if start < 0 {
		t.Fatal("EMFILE harness service is missing")
	}
	end := strings.Index(compose[start:], "\n  node1_evidence:")
	if end < 0 {
		t.Fatal("EMFILE harness service boundary is missing")
	}
	service := compose[start : start+end]
	if strings.Contains(service, "pid:") {
		t.Fatal("EMFILE injector shares the target PID namespace")
	}
	if !strings.Contains(service, "nofile: {soft: 128, hard: 128}") {
		t.Fatal("EMFILE injector independent descriptor budget is missing")
	}
	if !strings.Contains(compose, "node1_emfile:") ||
		!strings.Contains(compose, "nofile: {soft: 16, hard: 16}") {
		t.Fatal("EMFILE target descriptor limit is missing")
	}
}

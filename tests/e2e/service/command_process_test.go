package service_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestServiceCommandReadinessTimeoutAndCleanup(t *testing.T) {
	binary := buildArdentsBinary(t)
	root := t.TempDir()
	applicationSocket := "application.sock"
	routeSocket := "route.sock"
	planPath := filepath.Join(root, "plan.json")
	plan := map[string]any{
		"Role": "client", "NetworkID": strings.Repeat("01", 32),
		"BrokerID": strings.Repeat("02", 32), "AuthorityPublic": strings.Repeat("03", 32),
		"ConnectionPrincipal": strings.Repeat("04", 32), "IntroductionPublic": strings.Repeat("05", 32),
		"Target": strings.Repeat("06", 32), "ApplicationSocket": applicationSocket, "RouteSocket": routeSocket,
		"PublicationFile": "publication.bin", "At": time.Now().UTC().Format(time.RFC3339),
		"Deadline": "500ms", "BytesEachDirection": 32,
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "endpoint", "run", planPath)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("Service command reported success without an Application: %s", output)
	}
	ready := false
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		var event map[string]any
		if json.Unmarshal(bytes.TrimSpace(line), &event) == nil && event["kind"] == "ready" && event["role"] == "client" {
			ready = true
		}
	}
	if !ready {
		t.Fatalf("Service command did not expose readiness before its bounded failure:\n%s", output)
	}
	for _, path := range []string{filepath.Join(root, applicationSocket),
		filepath.Join(root, applicationSocket+".result"), filepath.Join(root, routeSocket)} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("Service command retained socket %s after failure: %v", path, statErr)
		}
	}
}

func buildArdentsBinary(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	name := "ardents"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	command := exec.Command("go", "build", "-trimpath", "-o", path, "./cmd/ardents")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build ardents command: %v\n%s", err, output)
	}
	return path
}

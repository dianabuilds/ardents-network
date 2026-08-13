package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEndpointPlanRejectsUnknownAndCrossRoleInputs(t *testing.T) {
	root := t.TempDir()
	unknown := filepath.Join(root, "unknown.json")
	if err := os.WriteFile(unknown, []byte(`{"Role":"client","Unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPlan(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown endpoint plan field accepted: %v", err)
	}
	client := endpointPlan{Role: "client", NetworkID: strings.Repeat("01", 32), BrokerID: strings.Repeat("02", 32),
		AuthorityPublic: strings.Repeat("03", 32), ConnectionPrincipal: strings.Repeat("04", 32),
		IntroductionPublic:      strings.Repeat("07", 32),
		AdministrationPrincipal: strings.Repeat("05", 32), Target: strings.Repeat("06", 32),
		ApplicationSocket: "/run/app.sock", RouteSocket: "/run/route.sock", PublicationFile: "/run/publication",
		At: "2033-05-18T03:33:20Z", Deadline: "5s", BytesEachDirection: 4096}
	if err := client.validate(); err == nil || !strings.Contains(err.Error(), "publisher administration") {
		t.Fatalf("client accepted publisher authority input: %v", err)
	}
}

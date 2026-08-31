package endpoint

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRejectsInvalidPlanBeforeReadinessOrResourceCreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte(`{"Role":"client"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ready := false
	result, err := Run(context.Background(), path, func(string) { ready = true })
	if err == nil || ready || result.Class != "" {
		t.Fatalf("Run invalid lifecycle: result=%+v ready=%v err=%v", result, ready, err)
	}
}

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

func TestEndpointPlanAcceptsSustainedLiveStreamBound(t *testing.T) {
	plan := endpointPlan{Role: "client", ApplicationSocket: "app", RouteSocket: "route", PublicationFile: "publication",
		IntroductionPublic: "introduction", Target: "target", At: "2033-05-18T03:33:20Z", Deadline: "5s",
		BytesEachDirection: 768 << 20}
	if err := plan.validate(); err != nil {
		t.Fatalf("sustained live stream bound rejected: %v", err)
	}
	plan.BytesEachDirection++
	if err := plan.validate(); err == nil {
		t.Fatal("stream above the product bound was accepted")
	}
}

func TestEndpointPlanSeparatesSetupDeadlineFromConnectionLifetime(t *testing.T) {
	plan := endpointPlan{Role: "client", ApplicationSocket: "app", RouteSocket: "route", PublicationFile: "publication",
		IntroductionPublic: "introduction", Target: "target", At: "2033-05-18T03:33:20Z", Deadline: "15s",
		Lifetime: "12m", BytesEachDirection: 4 << 20}
	if err := plan.validate(); err != nil {
		t.Fatal(err)
	}
	plan.Lifetime = "31m"
	if err := plan.validate(); err == nil {
		t.Fatal("connection lifetime above the development campaign bound was accepted")
	}
	plan.Lifetime = "10s"
	if err := plan.validate(); err == nil {
		t.Fatal("connection lifetime shorter than setup deadline was accepted")
	}
}

func TestEndpointPlanBoundsConcurrentConnectionsForStage5Capacity(t *testing.T) {
	plan := endpointPlan{Role: "client", ApplicationSocket: "app", RouteSocket: "route", PublicationFile: "publication",
		IntroductionPublic: "introduction", Target: "target", At: "2033-05-18T03:33:20Z", Deadline: "15s",
		Lifetime: "12m", BytesEachDirection: 4 << 20, MaximumConnections: 16}
	if err := plan.validate(); err != nil {
		t.Fatalf("sixteen bounded connections rejected: %v", err)
	}
	plan.MaximumConnections = 17
	if err := plan.validate(); err == nil {
		t.Fatal("connection capacity above sixteen was accepted")
	}
}

func TestEndpointSetupCreatesOneBrokerGeneration(t *testing.T) {
	plan := endpointPlan{Role: "client", NetworkID: strings.Repeat("01", 32), BrokerID: strings.Repeat("02", 32),
		AuthorityPublic: strings.Repeat("03", 32), ConnectionPrincipal: strings.Repeat("04", 32),
		IntroductionPublic: strings.Repeat("05", 32), Target: strings.Repeat("06", 32),
		ApplicationSocket: "app", RouteSocket: "route", PublicationFile: "publication", At: "2033-05-18T03:33:20Z",
		Deadline: "5s", BytesEachDirection: 4096}
	setup, _, _, err := endpointSetup(plan)
	if err != nil || setup.Admission == nil || setup.Admission.Isolation().State() != "generic/unqualified" {
		t.Fatalf("Endpoint did not create its Broker generation: admission=%v error=%v", setup.Admission, err)
	}
}

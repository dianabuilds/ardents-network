package endpoint

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunEndpointCancellationClosesOwnedListeners(t *testing.T) {
	root := t.TempDir()
	applicationSocket := shortApplicationPath(t)
	routeSocket := strings.TrimSuffix(applicationSocket, ".sock") + "-route.sock"
	t.Cleanup(func() { _ = os.Remove(routeSocket) })
	plan := endpointPlan{Role: "client", NetworkID: strings.Repeat("01", 32), BrokerID: strings.Repeat("02", 32),
		AuthorityPublic: strings.Repeat("03", 32), ConnectionPrincipal: strings.Repeat("04", 32),
		IntroductionPublic: strings.Repeat("05", 32), Target: strings.Repeat("06", 32),
		ApplicationSocket: applicationSocket, RouteSocket: routeSocket,
		PublicationFile: filepath.Join(root, "publication.bin"), At: time.Now().UTC().Format(time.RFC3339),
		Deadline: "15s", BytesEachDirection: 32}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ready := make(chan struct{}, 1)
	completed := make(chan error, 1)
	go func() {
		_, err := runEndpoint(ctx, plan, func() { ready <- struct{}{} })
		completed <- err
	}()
	select {
	case <-ready:
	case err := <-completed:
		t.Fatalf("Endpoint failed before readiness: %v", err)
	case <-time.After(time.Second):
		t.Fatal("Endpoint did not become ready")
	}
	cancel()
	select {
	case err := <-completed:
		if err == nil {
			t.Fatal("cancelled Endpoint reported success")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled Endpoint did not join its listener loop")
	}
	for _, path := range []string{plan.ApplicationSocket, plan.ApplicationSocket + ".result", plan.RouteSocket} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("cancelled Endpoint retained %s: %v", path, err)
		}
	}
	if err := plan.validate(); err != nil {
		t.Fatalf("cancellation plan is invalid: %v", err)
	}
}

//go:build live

package network_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func startBlockedPressureWork(t *testing.T, ctx context.Context, compose composeCall) {
	t.Helper()
	startBlockedNetwork(t, ctx, compose, "C1")
	waitForBlockedJSON(t, ctx, compose, "bridge", func(line []byte) bool {
		var value struct{ Kind, State string }
		return json.Unmarshal(line, &value) == nil && value.Kind == "adapter" && value.State == "READY"
	})
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		waitForKind(t, ctx, compose, role, "ready")
	}
	startLiveService(t, ctx, compose, "publisher-service", "publisher")
	runLiveOneShot(t, ctx, compose, "publication-operator")
	startLiveService(t, ctx, compose, "client-service", "client")
	startLiveContainer(t, ctx, compose, "publisher-app")
	startLiveContainer(t, ctx, compose, "client-app")
	if output, err := compose(ctx, "up", "-d", "--no-build", "--no-deps", "endpoint", "endpoint-observer", "policy"); err != nil {
		t.Fatalf("start final pressure Endpoint: %v\n%s", err, output)
	}
	waitForBlockedJSON(t, ctx, compose, "policy", func(line []byte) bool {
		var value struct{ Kind, State string }
		return json.Unmarshal(line, &value) == nil && value.Kind == "policy" && value.State == "READY"
	})
}

func finishBlockedPressureWork(t *testing.T, ctx context.Context, compose composeCall,
	fixture blockedEntryFixture, transferBytes uint32,
) {
	t.Helper()
	clientApp := waitForApplication(t, ctx, compose, "client-app")
	publisherApp := waitForApplication(t, ctx, compose, "publisher-app")
	if clientApp.Terminal != "success" || publisherApp.Terminal != "success" ||
		clientApp.SentBytes != transferBytes || publisherApp.ReceivedBytes != transferBytes ||
		clientApp.SentDigest != publisherApp.ReceivedDigest {
		t.Fatalf("pressure Application work changed: client=%+v publisher=%+v", clientApp, publisherApp)
	}
	assertBlockedPressureServices(t, fixture, compose, ctx)
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		if result := waitForKind(t, ctx, compose, role, "complete"); result.Terminal != "success" || !result.Cleanup {
			t.Fatalf("pressure %s Route = %+v", role, result)
		}
	}
	publishFinalWorkerTerminal()
	writeLiveFile(t, filepath.Join(fixture.root, "sync", "bridge", "bridge-stop"), []byte("stop\n"))
	for _, service := range blockedContainers("C1") {
		waitBlockedContainer(t, ctx, compose, service)
	}
}

func assertBlockedPressureServices(t *testing.T, fixture blockedEntryFixture, compose composeCall, ctx context.Context) {
	t.Helper()
	client := waitForServiceResult(t, ctx, compose, "client-service")
	publisher := waitForServiceResult(t, ctx, compose, "publisher-service")
	for role, result := range map[string]serviceconn.Result{"client": client, "publisher": publisher} {
		if result.Class != "clean service connection close" || result.AuthenticatedTarget != fixture.target ||
			result.RouteAttachmentsAccepted != 1 || result.ApplicationIPCAccepts != 1 {
			t.Fatalf("pressure %s Service Connection = %+v", role, result)
		}
	}
}

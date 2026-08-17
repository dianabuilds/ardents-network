//go:build live

package network_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func rewriteBlockedWorkload(t *testing.T, fixture blockedEntryFixture, direction string, count uint32) {
	t.Helper()
	clientSend, clientReceive, publisherSend, publisherReceive := uint32(0), count, count, uint32(0)
	if direction == "endpoint-to-publisher" {
		clientSend, clientReceive, publisherSend, publisherReceive = count, 0, 0, count
	} else if direction != "publisher-to-endpoint" {
		t.Fatalf("unknown final blocked direction %q", direction)
	}
	rewriteBlockedServicePlan(t, filepath.Join(fixture.root, "input", "client-service", "plan.json"),
		clientSend, clientReceive)
	rewriteBlockedServicePlan(t, filepath.Join(fixture.root, "input", "publisher-service", "plan.json"),
		publisherSend, publisherReceive)
}

func rewriteBlockedServicePlan(t *testing.T, path string, send, receive uint32) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan map[string]any
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	plan["SendBytes"], plan["ReceiveBytes"], plan["Lifetime"] = send, receive, "20m"
	writeLivePlan(t, filepath.Dir(path), "plan", plan)
}

func rewriteBlockedCapacity(t *testing.T, fixture blockedEntryFixture, capacity uint16) {
	t.Helper()
	if capacity != 4 && capacity != 16 {
		t.Fatalf("unsupported final Bridge capacity %d", capacity)
	}
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		path := filepath.Join(fixture.root, "input", role, "plan.json")
		rewriteBlockedPlan(t, path, func(plan map[string]any) {
			plan["MaximumAttachments"], plan["AttachmentTarget"] = capacity, capacity
		})
	}
	rewriteBlockedPlan(t, filepath.Join(fixture.root, "input", "bridge", "initiator.json"), func(plan map[string]any) {
		plan["MaximumAttachments"], plan["AttachmentTarget"] = capacity, capacity
	})
	rewriteBlockedPlan(t, filepath.Join(fixture.root, "input", "publisher-service", "plan.json"), func(plan map[string]any) {
		plan["MaximumConnections"] = capacity
	})
	if capacity == 16 {
		rewriteBlockedPlan(t, filepath.Join(fixture.root, "input", "bridge", "serve.json"), func(plan map[string]any) {
			plan["resource_profile"] = "h3-s-v1-strong"
		})
	}
}

func rewriteBlockedPlan(t *testing.T, path string, change func(map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan map[string]any
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	change(plan)
	writeLivePlan(t, filepath.Dir(path), filepath.Base(path[:len(path)-len(filepath.Ext(path))]), plan)
}

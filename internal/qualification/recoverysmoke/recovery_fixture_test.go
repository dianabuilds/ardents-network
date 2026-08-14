package recoverysmoke

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func TestCarrierAdjacentRolesShareTheBoundedAttachmentDeadline(t *testing.T) {
	for _, role := range []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"} {
		plan := make(map[string]any)
		applyRouteDeadline(plan, role)
		expected := "15s"
		if role == "rendezvous" || role == "responder" {
			expected = "10.5s"
		}
		if plan["Deadline"] != expected {
			t.Fatalf("role %s deadline %v; want %s", role, plan["Deadline"], expected)
		}
	}
}

func TestBaselineAttachmentResetRemovesReplacementSequence(t *testing.T) {
	root := t.TempDir()
	plans := filepath.Join(root, "route", "plans")
	if err := os.MkdirAll(plans, 0o700); err != nil {
		t.Fatal(err)
	}
	roles := []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"}
	for _, role := range roles {
		path := filepath.Join(plans, role+".json")
		if err := byteio.WriteJSON(path, map[string]any{
			"Role": role, "AttachmentPlans": []map[string]any{{}}, "ConcurrentAttachments": role == "publisher",
			"Lifetime": "12m",
		}, 64<<10); err != nil {
			t.Fatal(err)
		}
	}
	if err := setRouteAttachments(root, 1); err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		raw, err := os.ReadFile(filepath.Join(plans, role+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var plan map[string]any
		if err := json.Unmarshal(raw, &plan); err != nil {
			t.Fatal(err)
		}
		if plan["Attachments"] != float64(1) || plan["AttachmentPlans"] != nil ||
			plan["ConcurrentAttachments"] != nil || plan["Lifetime"] != nil {
			t.Fatalf("role %s baseline reset retained replacement sequence: %v", role, plan)
		}
	}
}

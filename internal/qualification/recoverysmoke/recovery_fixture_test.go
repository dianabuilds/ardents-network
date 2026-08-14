package recoverysmoke

import "testing"

func TestCarrierAdjacentRolesShareTheBoundedAttachmentDeadline(t *testing.T) {
	for _, role := range []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"} {
		plan := make(map[string]any)
		applyRouteDeadline(plan, role)
		expected := "15s"
		if role == "rendezvous" || role == "responder" {
			expected = "10s"
		}
		if plan["Deadline"] != expected {
			t.Fatalf("role %s deadline %v; want %s", role, plan["Deadline"], expected)
		}
	}
}

package architecture

import (
	"strings"
	"testing"
)

// ADR-0104 selects one copied State value at the Node boundary. The value's
// contents and receipt-time bound are checked by the owning packages.
func TestNodeDutyBoundaryUsesStateValue(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	config := string(readProjectFile(t, root, "internal/node/process_config.go"))
	if !strings.Contains(config, "func() (state.NodeDuty, error)") {
		t.Error("Node Config lost its State-created duty value callback")
	}
	for _, retired := range []string{"type DutyView interface", "type dutyFacts struct", "type dutyCandidate struct", "type dutyAuthority struct"} {
		if strings.Contains(config, retired) {
			t.Errorf("Node Config regained retired getter projection %q", retired)
		}
	}
	duty := string(readProjectFile(t, root, "internal/network/state/node_duty.go"))
	for _, required := range []string{"type NodeDuty struct", "func ProjectNodeDuty(snapshot Snapshot) NodeDuty", "func (s *networkState) CurrentNodeDuty() (NodeDuty, error)"} {
		if !strings.Contains(duty, required) {
			t.Errorf("State lost duty value declaration %q", required)
		}
	}
	for _, retired := range []string{"NodeDutyView", "DutyAuthority", "TransitIssuance", "SourceAttempts", "PendingEpoch", "ViewRoot"} {
		if strings.Contains(duty, retired) {
			t.Errorf("State duty value regained retired projection member %q", retired)
		}
	}
}

package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR-0104 (F-07): the State-to-Node duty handoff is one State-created copied
// value. The 38-getter DutyView Interface, the Node-side dutyFacts parallel
// projection, the test-scope getter projection from ADR-0101, and the unused
// authority/Transit-issuance projections are all retired.
func TestNodeDutyHandoffIsOneCopiedValue(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	contract := string(readProjectFile(t, root, "internal/node/contract.go"))
	for _, forbidden := range []string{
		"type DutyView interface",
		"type dutyFacts struct",
		"type dutyCandidate struct",
		"type dutyAuthority struct",
		"DutyRecordGeneration() uint64",
	} {
		if strings.Contains(contract, forbidden) {
			t.Errorf("contract.go still declares retired seam member %q", forbidden)
		}
	}
	if !strings.Contains(contract, "func() (state.NodeDuty, error)") {
		t.Error("contract.go lost the copied-value Config.Current callback")
	}
	duty := string(readProjectFile(t, root, "internal/network/state/node_duty.go"))
	for _, required := range []string{
		"type NodeDuty struct",
		"type NodeDutyCandidate struct",
		"func ProjectNodeDuty(snapshot Snapshot) NodeDuty",
		"func (s *networkState) CurrentNodeDuty() (NodeDuty, error)",
	} {
		if !strings.Contains(duty, required) {
			t.Errorf("node_duty.go lost required declaration %q", required)
		}
	}
	for _, forbidden := range []string{
		"NodeDutyView",
		"DutyAuthority",
		"TransitIssuance",
		"SourceAttempts",
		"PendingEpoch",
		"ViewRoot",
	} {
		if strings.Contains(duty, forbidden) {
			t.Errorf("node_duty.go re-exposes retired projection member %q", forbidden)
		}
	}
	lifecycle := string(readProjectFile(t, root, "internal/node/lifecycle.go"))
	if !strings.Contains(lifecycle, "node duty candidate count is outside its bound") {
		t.Error("lifecycle lost the receipt-time candidate bound validation")
	}
	for _, relative := range []string{
		"internal/network/state/node_duty_view.go",
		"internal/node/duty_facts_projection_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired getter-projection file still exists: %s", relative)
		}
	}
}

func TestNodeAdmissionDeadlineHelpersRetired(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/node/admission_deadline.go",
		"internal/node/admission_deadline_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired admission-deadline helper file still exists: %s", relative)
		}
	}
	allowlist := string(readProjectFile(t, root, "tests/profiles/deadcode-allowlist.json"))
	if strings.Contains(allowlist, "unwired native-node tracer") ||
		strings.Contains(allowlist, "retained Node admission-deadline helpers") ||
		strings.Contains(allowlist, "internal/node.dutyFacts.Duty") ||
		strings.Contains(allowlist, "internal/node.boundedAdmissionDeadline") {
		t.Error("deadcode registry still carries the dissolved Node groups")
	}
}

func TestComposedNodeDutyHandoffIsPreserved(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	mode := string(readProjectFile(t, root, "cmd/ardents-node/node_mode.go"))
	if !strings.Contains(mode, "runtime.node.Current = store.CurrentNodeDuty") || !strings.Contains(mode, "node.Run(ctx, runtime.node)") {
		t.Error("node command lost the composed authenticated duty handoff")
	}
	dispatch := string(readProjectFile(t, root, "cmd/ardents-node/main.go"))
	if !strings.Contains(dispatch, `if arguments[0] == "node" {`) || !strings.Contains(dispatch, "runNode(ctx, arguments[2], output)") {
		t.Error("node command lost its selected dispatch")
	}
	lifecycle := string(readProjectFile(t, root, "internal/node/lifecycle.go"))
	if !strings.Contains(lifecycle, "duty, err := config.Current()") || !strings.Contains(lifecycle, "func currentFacts(config runtimeConfig) (state.NodeDuty, error)") {
		t.Error("lifecycle lost the receipt-time duty value copy")
	}
}

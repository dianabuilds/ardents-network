package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR-0101: the dutyFacts DutyView projection is test scope and the
// unimplemented local admission-timeout helpers are retired, while the
// composed production handoff stays exactly as network-route-node.md
// documents it.
func TestNodeDutyProjectionIsTestScope(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	contract := string(readProjectFile(t, root, "internal/node/contract.go"))
	if strings.Contains(contract, "func (facts dutyFacts) Duty") {
		t.Error("production contract still declares dutyFacts DutyView accessors")
	}
	for _, required := range []string{
		"type DutyView interface",
		"DutyRecordGeneration() uint64",
		"type dutyFacts struct",
		"duty_facts_projection_test.go",
	} {
		if !strings.Contains(contract, required) {
			t.Errorf("contract.go lost required declaration or note %q", required)
		}
	}
	projection := string(readProjectFile(t, root, "internal/node/duty_facts_projection_test.go"))
	if got := strings.Count(projection, "func (facts dutyFacts) Duty"); got != 38 {
		t.Errorf("test projection declares %d accessors, want the complete 38-getter DutyView set", got)
	}
	lifecycle := string(readProjectFile(t, root, "internal/node/lifecycle_test.go"))
	if strings.Contains(lifecycle, "DutyRecordGeneration") {
		t.Error("lifecycle_test.go still declares a partial out-of-place projection getter")
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
	if !strings.Contains(mode, "store.CurrentNodeDuty()") || !strings.Contains(mode, "node.Run(ctx, runtime.node)") {
		t.Error("node command lost the composed authenticated duty handoff")
	}
	dispatch := string(readProjectFile(t, root, "cmd/ardents-node/main.go"))
	if !strings.Contains(dispatch, `if arguments[0] == "node" {`) || !strings.Contains(dispatch, "runNode(ctx, arguments[2], output)") {
		t.Error("node command lost its selected dispatch")
	}
	lifecycle := string(readProjectFile(t, root, "internal/node/lifecycle.go"))
	if !strings.Contains(lifecycle, "view.DutyRecordGeneration()") || !strings.Contains(lifecycle, "result := dutyFacts{") {
		t.Error("lifecycle lost the production DutyView copy projection")
	}
	view := string(readProjectFile(t, root, "internal/network/state/node_duty_view.go"))
	if !strings.Contains(view, "func (s *networkState) CurrentNodeDuty() (NodeDutyView, error)") {
		t.Error("State lost the narrow authenticated current-generation view")
	}
}

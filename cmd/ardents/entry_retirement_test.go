package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ADR-0106 retired the Invite-root operator commands. This oracle proves
// the refusal precedes every effect: the plan file is never opened or
// altered, the former Entry root path is never created, and no output is
// rendered - for an absent plan and for a fully shaped pre-retirement plan
// alike.
func TestEntryInviteCommandsRetireBeforeEffects(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	rootPath := filepath.Join(directory, "invite-root")
	invitePath := filepath.Join(directory, "bridge.invite")
	planPath := filepath.Join(directory, "entry-import-plan.json")
	plan := map[string]any{
		"state_root":            rootPath,
		"network_state_root":    filepath.Join(directory, "network-state"),
		"invite_file":           invitePath,
		"network_id":            "00",
		"network_authorities":   []string{},
		"network_threshold":     1,
		"network_profile":       "ardents-interactive-route-v2",
		"local_role_state_root": filepath.Join(directory, "local-roles"),
		"time_confidence_file":  filepath.Join(directory, "time-confidence"),
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, rawPlan, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name     string
		planPath string
	}{
		{name: "shaped plan", planPath: planPath},
		{name: "absent plan", planPath: filepath.Join(directory, "missing-plan.json")},
	} {
		for _, verb := range []string{"import", "recipient"} {
			var output bytes.Buffer
			runErr := run(t.Context(), []string{"entry", verb, test.planPath}, &output)
			if runErr == nil || runErr.Error() != errEntryCommandRetired.Error() {
				t.Fatalf("%s %s err=%v", test.name, verb, runErr)
			}
			if output.Len() != 0 {
				t.Fatalf("%s %s rendered %q", test.name, verb, output.String())
			}
		}
	}

	if retained, readErr := os.ReadFile(planPath); readErr != nil || !bytes.Equal(retained, rawPlan) {
		t.Fatalf("refused entry plan was altered: %v", readErr)
	}
	for _, forbidden := range []string{rootPath, plan["local_role_state_root"].(string), plan["network_state_root"].(string)} {
		if _, statErr := os.Stat(forbidden); !os.IsNotExist(statErr) {
			t.Fatalf("refused entry command created %s", forbidden)
		}
	}
}

func TestEntryRouteRefusesUnknownAndIncompleteForms(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{
		{"entry"},
		{"entry", "export", "plan.json"},
		{"entry", ""},
	} {
		var output bytes.Buffer
		if err := run(t.Context(), arguments, &output); err == nil || output.Len() != 0 {
			t.Fatalf("entry %v err=%v output=%q", arguments, err, output.String())
		}
	}
}

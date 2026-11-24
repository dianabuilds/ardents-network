package node_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNodeRolesRefuseMissingNetworkHistoryWithoutInitializing(t *testing.T) {
	binary := buildNodeCommand(t)
	roles := []struct {
		name, profile, extra string
	}{
		{"forwarding", "ardents-forwarding-process-v1", ""},
		{"join", "ardents-join-process-v1", ""},
		{"descriptor", "ardents-descriptor-process-v1", "descriptor_root"},
		{"introduction", "ardents-introduction-process-v1", "introduction_root"},
	}
	for _, role := range roles {
		t.Run(role.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "state")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			plan := map[string]any{
				"network": map[string]any{
					"root": root, "network_id": [32]byte{1}, "authorities": [][32]byte{{2}},
					"threshold": 1, "profile_authority": [32]byte{2}, "clock_observation_file": filepath.Join(base, "clock"),
				},
				"process_profile": role.profile, "participation_root": filepath.Join(base, "participation"), "spend_root": filepath.Join(base, "spend"), "hosting_root": filepath.Join(base, "hosting"), "retirement_wait_ms": 100,
			}
			if role.extra != "" {
				plan[role.extra] = filepath.Join(base, "role-history")
			}
			raw, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(base, "plan.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			for _, option := range []string{"--config", "--retained-config"} {
				command := exec.CommandContext(t.Context(), binary, "node", role.name, option, path)
				var diagnostic bytes.Buffer
				command.Stderr = &diagnostic
				output, err := command.Output()
				if command.ProcessState == nil || command.ProcessState.ExitCode() != 1 || !bytes.Contains(output, []byte("state-unavailable")) {
					t.Errorf("expected State refusal for %s: %v: %s %s", option, err, output, &diagnostic)
				}
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("refusal modified empty history: %v %v", entries, err)
			}
		})
	}
}

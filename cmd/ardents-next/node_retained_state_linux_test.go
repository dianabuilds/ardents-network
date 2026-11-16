package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestNodeRolesRefuseMissingNetworkHistoryWithoutInitializing(t *testing.T) {
	roles := []struct {
		name, profile, extra string
		run                  func(context.Context, []string, io.Writer, io.Writer) int
	}{
		{"forwarding", "ardents-forwarding-process-v1", "", runNodeForwarding},
		{"join", "ardents-join-process-v1", "", runNodeJoin},
		{"descriptor", "ardents-descriptor-process-v1", "descriptor_root", runNodeDescriptor},
		{"introduction", "ardents-introduction-process-v1", "introduction_root", runNodeIntroduction},
	}
	for _, role := range roles {
		t.Run(role.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "state")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			plan := map[string]any{
				"network":         networkAuthorityPlan{Root: root, NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{2}, ClockObservationFile: filepath.Join(base, "clock")},
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
			var out, diagnostic bytes.Buffer
			if code := role.run(t.Context(), []string{"--config", path}, &out, &diagnostic); code != 1 || !bytes.Contains(out.Bytes(), []byte("state-unavailable")) {
				t.Errorf("expected State refusal, got %d: %s %s", code, &out, &diagnostic)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("refusal modified empty history: %v %v", entries, err)
			}
		})
	}
}

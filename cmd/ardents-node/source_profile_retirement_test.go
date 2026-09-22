package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceModeRefusesRetiredProfileBeforeEffects(t *testing.T) {
	for _, test := range []struct {
		name  string
		mixed bool
	}{
		{name: "old selector"},
		{name: "mixed old and closed selector", mixed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			stateRoot := filepath.Join(root, "state")
			roleRoot := filepath.Join(root, "role")
			plan := map[string]any{
				"schema":                    "ardents-source-server-v1",
				"state_root":                stateRoot,
				"local_role_state_root":     roleRoot,
				"network_id":                strings.Repeat("01", 32),
				"authority_public":          []string{strings.Repeat("02", 32)},
				"threshold":                 1,
				"at":                        "2026-09-22T00:00:00Z",
				"listen":                    "127.0.0.1:1",
				"server_certificate":        filepath.Join(root, "missing-server.pem"),
				"server_key":                filepath.Join(root, "missing-server.key"),
				"client_root":               filepath.Join(root, "missing-client.pem"),
				"client_key_digests":        []string{strings.Repeat("03", 32)},
				"native_rendezvous_profile": true,
			}
			if test.mixed {
				plan["state_profile"] = "ardents-route-v3"
				plan["state_profile_authority"] = strings.Repeat("02", 32)
			}
			raw, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "source.json")
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}

			var output bytes.Buffer
			err = run(context.Background(), []string{"source", "--config", path}, &output)
			if err == nil || err.Error() != "old Source profile is retired" {
				t.Fatalf("retired Source profile error = %v", err)
			}
			if output.Len() != 0 {
				t.Fatalf("retired Source profile output = %q", output.String())
			}
			for _, forbidden := range []string{stateRoot, roleRoot} {
				if _, statErr := os.Stat(forbidden); !os.IsNotExist(statErr) {
					t.Fatalf("retired Source profile created %s: %v", forbidden, statErr)
				}
			}
		})
	}
}

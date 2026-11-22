//go:build linux && network_signing_native

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestControlProvisioningKeysNativeRoleAndPurpose(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	program, _ := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_CONTROL_PROGRAM")
	seen := make(map[string]bool)
	for _, role := range []string{"forwarding", "join", "descriptor"} {
		for _, purpose := range []string{"node", "state"} {
			t.Run(role+"/"+purpose, func(t *testing.T) {
				operation := "prepare-" + purpose + "-key"
				field := purpose + "_public_key"
				name := purpose + "-key.pem"
				output := filepath.Join(parent, role+"-"+purpose)
				run := func(role, operation string) ([]byte, error) {
					return exec.CommandContext(t.Context(), program, role, operation, output).CombinedOutput()
				}
				raw, err := run(role, operation)
				if err != nil {
					t.Fatalf("key preparation failed: %v: %s", err, raw)
				}
				var receipt map[string]json.RawMessage
				var public string
				if err := json.Unmarshal(raw, &receipt); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(receipt[field], &public); err != nil {
					t.Fatalf("missing purpose-specific public key: %v", err)
				}
				key, err := hex.DecodeString(public)
				if err != nil || len(key) != 32 || seen[public] {
					t.Fatal("each role and purpose must create a fresh public key")
				}
				seen[public] = true
				want := fmt.Appendf(nil, `{"operation":%q,%q:%q,"state_accepted":false,"installed_qualification":false,"runtime_authority":false}`+"\n",
					role+"."+operation, field, public)
				if !bytes.Equal(raw, want) {
					t.Fatal("purpose-specific receipt differs")
				}
				entries, err := os.ReadDir(output)
				if err != nil || len(entries) != 1 || entries[0].Name() != name {
					t.Fatal("unexpected key output inventory", err)
				}
				path := filepath.Join(output, name)
				before, err := os.Lstat(path)
				if err != nil || before.Mode() != 0600 || before.Size() == 0 {
					t.Fatal("missing protected key file", err)
				}
				otherOperation := "prepare-state-key"
				if purpose == "state" {
					otherOperation = "prepare-node-key"
				}
				for _, attempt := range [][2]string{{role, otherOperation}, {"join", operation}, {"descriptor", operation}, {"forwarding", operation}} {
					if _, err := run(attempt[0], attempt[1]); err == nil {
						t.Fatalf("%s %s reused an occupied output", attempt[0], attempt[1])
					}
				}
				after, err := os.Lstat(path)
				if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() ||
					before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
					t.Fatal("refused operation changed the original key file", err)
				}
				entries, err = os.ReadDir(output)
				if err != nil || len(entries) != 1 || entries[0].Name() != name {
					t.Fatal("refused operation changed key output inventory", err)
				}
			})
		}
	}
}

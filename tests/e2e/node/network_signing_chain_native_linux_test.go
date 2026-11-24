//go:build linux && network_signing_native

package node_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
)

func writeSigningTestKey(t *testing.T, path string, key ed25519.PrivateKey) {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(der)
	raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	defer clear(raw)
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func signingDeclaration(t *testing.T, purpose string, plan any, reference time.Time) []byte {
	t.Helper()
	field := purpose
	if purpose == "node-record" {
		field = "record"
	}
	return preparationJSON(t, map[string]any{"schema": "ardents-initial-" + purpose + "-signing-plan-v1", "reference_time": reference.Format(time.RFC3339), field: plan})
}

func profileSigningDeclaration(t *testing.T, plan any, public ed25519.PublicKey, reference time.Time) []byte {
	t.Helper()
	return preparationJSON(t, map[string]any{"schema": "ardents-closed-profile-signing-plan-v1", "reference_time": reference.Format(time.RFC3339), "state_authority_key": preparationHex(public), "profile": plan})
}

func TestNetworkSigningCommandsGenuineStateChain(t *testing.T) {
	sourcefixture.SigningWorkspace(t)
	binary := buildNodeCommand(t)
	runNetworkPreparedStateChain(t, binary, func(t *testing.T, purpose string, plan any, key ed25519.PrivateKey, message []byte) ([]byte, [][]byte) {
		parent := sourcefixture.SigningWorkspace(t)
		planPath, prepared, keyPath, root := filepath.Join(parent, "plan"), filepath.Join(parent, "prepared"), filepath.Join(parent, "key"), filepath.Join(parent, "signed")
		declaration := signingDeclaration(t, purpose, plan, time.Now().UTC().Truncate(time.Second))
		operation := "sign-initial-" + purpose
		if purpose == "closed-profile" {
			operation = "sign-closed-profile"
			declaration = profileSigningDeclaration(t, plan, key.Public().(ed25519.PublicKey), time.Now().UTC().Truncate(time.Second))
		}
		if err := os.WriteFile(planPath, declaration, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(prepared, message, 0600); err != nil {
			t.Fatal(err)
		}
		writeSigningTestKey(t, keyPath, key)
		digest := sha256.Sum256(message)
		cmd := exec.CommandContext(t.Context(), binary, "network", operation, planPath, prepared, preparationHex(digest[:]), keyPath, root)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal("genuine signing command", err, string(out))
		}
		if purpose == "closed-profile" && (!bytes.Contains(out, []byte(`"state_accepted":false`)) || !bytes.Contains(out, []byte(`"installed_qualification":false`))) {
			t.Fatal("profile receipt overstates acceptance", string(out))
		}
		name := "node-record.bin"
		var materials [][]byte
		if purpose == "epoch" {
			name = "epoch.bin"
		}
		if purpose == "closed-profile" {
			name = "profile.bin"
		}
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if purpose == "epoch" {
			records := plan.(map[string]any)["records"].([]string)
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1+2*len(records) {
				t.Fatal("finite Epoch inventory", err)
			}
			for i, record := range records {
				material, e := os.ReadFile(filepath.Join(root, fmt.Sprintf("material-%02d.bin", i)))
				if e != nil {
					t.Fatal(e)
				}
				materials = append(materials, material)
				input, e := os.ReadFile(filepath.Join(root, fmt.Sprintf("input-%02d.bin", i)))
				if e != nil || preparationHex(input) != record {
					t.Fatal("product Source input differs", e)
				}
			}
		}
		retry := exec.CommandContext(t.Context(), binary, "network", operation, planPath, prepared, preparationHex(digest[:]), keyPath, root)
		if retry.Run() == nil {
			t.Fatal("existing output adopted")
		}
		return got, materials
	})
}

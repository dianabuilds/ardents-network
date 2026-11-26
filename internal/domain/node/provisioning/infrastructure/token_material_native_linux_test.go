//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuerprofile"
	"github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func forwardingTokenNativeArgs(output, signer string) []string {
	start := time.Now().UTC().Truncate(time.Hour)
	return []string{"--network-id", strings.Repeat("01", 32), "--issuer-node-id", strings.Repeat("02", 32), "--issuer-node-key", signer, "--start", start.Format(time.RFC3339), "--end", start.Add(time.Hour).Format(time.RFC3339), "--output", output}
}

func TestRoleTokenMaterialNativeCanonicalInventoryAndRetainedRoot(t *testing.T) {
	for _, role := range []nodeProvisioningRole{forwardingProvisioningRole, joinProvisioningRole, descriptorProvisioningRole} {
		t.Run(string(role), func(t *testing.T) { testRoleTokenMaterialNativeCanonicalInventoryAndRetainedRoot(t, role) })
	}
}

func testRoleTokenMaterialNativeCanonicalInventoryAndRetainedRoot(t *testing.T, role nodeProvisioningRole) {
	t.Helper()
	parent := networkfixture.ProtectedExt4Parent(t)
	command, _ := networkfixture.PinnedProvisionProgram(t, "ARDENTS_INFRASTRUCTURE_PROVISION_PROGRAM")
	keyRoot := filepath.Join(parent, "node-key")
	keyReceipt, err := networkfixture.ProvisionCommand(t, command, string(role), "prepare-node-key", keyRoot).CombinedOutput()
	var key struct {
		Public string `json:"node_public_key"`
	}
	if err != nil || json.Unmarshal(keyReceipt, &key) != nil || len(key.Public) != 64 {
		t.Fatal("canonical Node public key unavailable", err)
	}
	output := filepath.Join(parent, "offline-token-material")
	args := forwardingTokenNativeArgs(output, key.Public)
	_, binding, _, err := nodeTokenMaterialArguments(role, args)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := networkfixture.ProvisionCommand(t, command, append([]string{string(role), "prepare-token-keys"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatal("canonical finite material producer", err, string(receipt))
	}
	var result struct {
		Operation string `json:"operation"`
		SHA       string `json:"sha256"`
		State     bool   `json:"state_accepted"`
		Issuer    bool   `json:"issuer_authority"`
		Runtime   bool   `json:"runtime_authority"`
		Installed bool   `json:"installed_qualification"`
	}
	if json.Unmarshal(receipt, &result) != nil || result.Operation != string(role)+".prepare-token-keys" || result.State || result.Issuer || result.Runtime || result.Installed {
		t.Fatal("material receipt claims authority")
	}
	public, err := os.ReadFile(filepath.Join(output, "public", "token-inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(public)
	if hex.EncodeToString(sum[:]) != result.SHA {
		t.Fatal("receipt not bound to actual public inventory")
	}
	var exported struct {
		Schema    string             `json:"schema"`
		Inventory issuance.Inventory `json:"inventory"`
	}
	if json.Unmarshal(public, &exported) != nil || exported.Schema != "ardents-"+string(role)+"-token-inventory-v1" || exported.Inventory.Binding != binding || len(exported.Inventory.Keys) != 3 || issuerprofile.ValidateCohorts(binding.Start, binding.End, exported.Inventory.Keys) != nil {
		t.Fatal("public cohort/binding mismatch")
	}
	// Reopen only through the actual Admission owner. No test reads private files.
	material := filepath.Join(output, "material")
	store, err := issuance.Open(t.Context(), material, binding)
	if err != nil {
		t.Fatal(err)
	}
	inventory, inventoryErr := store.Inventory()
	if err := errors.Join(inventoryErr, store.Close()); err != nil {
		t.Fatal(err)
	}
	actual, _ := json.Marshal(inventory)
	wanted, _ := json.Marshal(exported.Inventory)
	if !bytes.Equal(actual, wanted) {
		t.Fatal("export differs from genuinely retained material")
	}
	before, err := os.Lstat(filepath.Join(material, "issuer.keys"))
	if err != nil || before.Mode() != 0600 {
		t.Fatal("private Admission material custody unavailable", err)
	}
	if _, err := networkfixture.ProvisionCommand(t, command, append([]string{string(role), "prepare-token-keys"}, args...)...).CombinedOutput(); err == nil {
		t.Fatal("existing material root was adopted")
	}
	after, err := os.Lstat(filepath.Join(material, "issuer.keys"))
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("refused retry changed original material", err)
	}
	if unchanged, err := os.ReadFile(filepath.Join(output, "public", "token-inventory.json")); err != nil || !bytes.Equal(public, unchanged) {
		t.Fatal("refused retry changed inventory", err)
	}
}

func TestForwardingTokenMaterialNativeCancellationRetainsLeaseAndCloseFailure(t *testing.T) {
	parent := networkfixture.ProtectedExt4Parent(t)
	output := filepath.Join(parent, "held-material")
	args := forwardingTokenNativeArgs(output, strings.Repeat("03", 32))
	_, binding, _, err := nodeTokenMaterialArguments(forwardingProvisioningRole, args)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	held, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	releaseHeld := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseHeld()
	done := make(chan error, 1)
	go func() {
		var closeErr error
		public, err := prepareNodeTokenMaterial(ctx, forwardingProvisioningRole, args, func(phase string, file *os.File) {
			if phase == "material-open" {
				close(held)
				<-release
				closeErr = file.Close()
			}
		})
		if public != nil {
			done <- errors.New("canceled operation returned public success")
			return
		}
		done <- errors.Join(err, closeErr)
	}()
	select {
	case <-held:
	case err := <-done:
		t.Fatal("did not retain original material", err)
	case <-time.After(20 * time.Second):
		t.Fatal("material preparation did not reach native hold")
	}
	cancel()
	material := filepath.Join(output, "material")
	if foreign, err := issuance.Open(t.Context(), material, binding); !errors.Is(err, issuance.ErrBusy) {
		if err == nil {
			_ = foreign.Close()
		}
		releaseHeld()
		t.Fatal("cancellation released original live lease", err)
	}
	select {
	case err := <-done:
		releaseHeld()
		t.Fatal("held physical work reported completion", err)
	default:
	}
	releaseHeld()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
			t.Fatal("original cancellation or actual Close failure lost", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("physical material/input join did not finish")
	}
	if _, err := os.Lstat(filepath.Join(output, "public")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canceled preparation published public output", err)
	}
	store, err := issuance.Open(t.Context(), material, binding)
	if err != nil {
		t.Fatal("original Admission lease not physically released", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareNodeTokenMaterial(t.Context(), forwardingProvisioningRole, args, nil); err == nil {
		t.Fatal("partial original output adopted or reset")
	}
}

//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
)

// These are command/storage controls with fixture bindings, not authenticated
// Network or installed startup. No predecessor runtime is imported or invoked.
func TestIssuanceFreshKeysAndPredecessorRootRefusal(t *testing.T) {
	var inventories [2]issuance.Inventory
	for i := range inventories {
		plan, input := issuanceCommandPlan(t)
		if result, err := issuanceCommand(t, "initialize", input, ""); err != nil || result.Outcome != "completed" {
			t.Fatal(result, err)
		}
		store, err := issuance.Open(t.Context(), plan.Root, plan.Binding)
		if err != nil {
			t.Fatal(err)
		}
		inventories[i], err = store.Inventory()
		if err != nil {
			t.Fatal(err)
		}
		if err = store.Close(); err != nil {
			t.Fatal(err)
		}
		before := snapshotAdmissionRoot(t, plan.Root)
		if result, err := issuanceCommand(t, "initialize", input, ""); err == nil || result.Outcome == "completed" {
			t.Fatal("initialization reused an existing key root")
		}
		if result, err := issuanceCommand(t, "inspect", input, ""); err != nil || result.Outcome != "completed" {
			t.Fatal(result, err)
		}
		if !reflect.DeepEqual(before, snapshotAdmissionRoot(t, plan.Root)) {
			t.Fatal("repeated initialization or inspection changed keys")
		}
	}
	for _, first := range inventories[0].Keys {
		for _, second := range inventories[1].Keys {
			if bytes.Equal(first.SPKI, second.SPKI) {
				t.Fatal("independent initializations reused a key")
			}
		}
	}

	for _, layout := range []string{"empty", "predecessor", "partial-current"} {
		t.Run(layout, func(t *testing.T) {
			plan, input := issuanceCommandPlan(t)
			if err := os.Mkdir(plan.Root, 0700); err != nil {
				t.Fatal(err)
			}
			files := map[string]string{}
			switch layout {
			case "predecessor":
				// Deliberately synthetic old inventory. Its presence must refuse
				// before decoding, key generation, adoption or public export.
				files[".ardents-closed-issuer-v1"] = "retired binding"
				files["closed-issuer-keys"] = "ARDCKR01synthetic private residue"
				files["closed-token-ledger"] = "ARDILG02synthetic spent history"
			case "partial-current":
				files["issuer.pending"] = "interrupted initialization"
			}
			for name, contents := range files {
				if err := os.WriteFile(filepath.Join(plan.Root, name), []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshotAdmissionRoot(t, plan.Root)
			for range 2 {
				for _, operation := range []string{"initialize", "inspect"} {
					if result, err := issuanceCommand(t, operation, input, ""); err == nil || result.Outcome == "completed" {
						t.Fatalf("%s accepted %s root", operation, layout)
					}
					if !reflect.DeepEqual(before, snapshotAdmissionRoot(t, plan.Root)) {
						t.Fatal("refused root was changed")
					}
					if _, err := os.Lstat(plan.InventoryFile); !os.IsNotExist(err) {
						t.Fatal("refusal exported a new inventory")
					}
				}
			}
		})
	}
}

func TestAdmissionFreshInitializationCannotResetSpentHistory(t *testing.T) {
	root, initialize, debit, _ := admissionCommandPlans(t)
	command := func(operation string, config any) (admissionResult, error) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), compiledCommand(t), "admission", operation, "--config", hostingConfig(t, config))
		var output, diagnostic bytes.Buffer
		cmd.Stdout, cmd.Stderr = &output, &diagnostic
		err := cmd.Run()
		var result admissionResult
		if json.Unmarshal(output.Bytes(), &result) != nil {
			t.Fatal("command returned no structured outcome", err)
		}
		return result, err
	}
	if result, err := command("initialize", initialize); err != nil || result.Outcome != quota.Initialized {
		t.Fatal(result, err)
	}
	if result, err := command("debit", debit); err != nil || result.Outcome != quota.Debited {
		t.Fatal(result, err)
	}
	before := snapshotAdmissionRoot(t, root)
	for range 2 {
		if result, err := command("initialize", initialize); err == nil || result.Outcome == quota.Initialized {
			t.Fatal("initialization accepted an existing spent root")
		}
		if !reflect.DeepEqual(before, snapshotAdmissionRoot(t, root)) {
			t.Fatal("initialization changed spent history")
		}
		if result, err := command("debit", debit); err != nil || result.Outcome != quota.AlreadyDebited {
			t.Fatal("initialization lost exact retry", result, err)
		}
	}
	foreign := t.TempDir()
	if err := os.WriteFile(filepath.Join(foreign, "closed-token-ledger"), []byte("ARDILG02synthetic spent history"), 0600); err != nil {
		t.Fatal(err)
	}
	initialize["root"], debit["root"] = foreign, foreign
	before = snapshotAdmissionRoot(t, foreign)
	for range 2 {
		for _, operation := range []string{"initialize", "debit"} {
			input := initialize
			if operation == "debit" {
				input = debit
			}
			if result, err := command(operation, input); err == nil || result.Outcome != quota.Unavailable {
				t.Fatal("predecessor journal accepted", result, err)
			}
			if !reflect.DeepEqual(before, snapshotAdmissionRoot(t, foreign)) {
				t.Fatal("predecessor journal changed")
			}
		}
	}
}

// Hash fixture contents instead of retaining or logging private key bytes.
func snapshotAdmissionRoot(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string][32]byte, len(entries))
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		result[entry.Name()] = sha256.Sum256(raw)
		clear(raw)
	}
	return result
}

func TestAdmissionFreshAuthorityRefusesPredecessorPermission(t *testing.T) {
	raw, facts, binding := admissionCommandFixture(t, 2, 1, 2, 1)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	copy(binding.Authority[:], public)
	facts.Authority = binding.Authority
	root := filepath.Join(t.TempDir(), "quota")
	if err := quota.Initialize(root, binding); err != nil {
		t.Fatal(err)
	}
	before := snapshotAdmissionRoot(t, root)
	batch := filepath.Join(t.TempDir(), "batch")
	config := map[string]any{"root": root, "binding": admissionBindingConfig(binding), "batch_file": batch, "facts": admissionFactsConfig(facts), "kind": "bootstrap"}
	runBatch := func() (admissionResult, error) {
		t.Helper()
		if err := os.WriteFile(batch, raw, 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), compiledCommand(t), "admission", "debit", "--config", hostingConfig(t, config))
		var output bytes.Buffer
		cmd.Stdout = &output
		err := cmd.Run()
		var result admissionResult
		if json.Unmarshal(output.Bytes(), &result) != nil {
			t.Fatal("no structured debit outcome", err)
		}
		return result, err
	}
	if result, err := runBatch(); err == nil || result.Outcome != "invalid-signature" {
		t.Fatal("predecessor authority admitted", result, err)
	}
	if !reflect.DeepEqual(before, snapshotAdmissionRoot(t, root)) {
		t.Fatal("foreign permission changed quota")
	}
	// Independent fixture grammar: eight-byte batch magic, 164-byte Permission
	// transcript and its 64-byte signature. The holder proof is unchanged.
	transcript := append([]byte("ardents-issuance-permission-v1\x00"), raw[8:172]...)
	copy(raw[172:236], ed25519.Sign(private, transcript))
	if result, err := runBatch(); err != nil || result.Outcome != quota.Debited {
		t.Fatal("new authority's permission refused", result, err)
	}
}

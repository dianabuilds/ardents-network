//go:build linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestConfirmedIssuanceOutputCannotChangeOwnedRoots(t *testing.T) {
	for _, target := range []string{"result_root", "key_root", "admission_root"} {
		t.Run(target, func(t *testing.T) {
			_, init, issue, verify := issuanceResultCommandPlan(t)
			if r, err := resultCommand(t, "initialize-results", init, ""); err != nil {
				t.Fatal(r, err)
			}
			journal := filepath.Join(issue["admission_root"].(string), "admission.journal")
			before, err := os.ReadFile(journal)
			if err != nil {
				t.Fatal(err)
			}
			outside := issue["response_file"]
			output := filepath.Join(issue[target].(string), "response.bin")
			issue["response_file"] = output
			if r, err := resultCommand(t, "issue", issue, ""); err == nil || r.Phase != "input" || r.Outcome != "invalid-input" {
				t.Fatalf("owned output must refuse before debit: %v %v", r, err)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("output created: %v", err)
			}
			after, err := os.ReadFile(journal)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("invalid output consumed quota", err)
			}
			issue["response_file"] = outside
			if r, err := resultCommand(t, "issue", issue, ""); err != nil || r.Outcome != "issued-offline" {
				t.Fatal("owners no longer usable", r, err)
			}
			response, err := os.ReadFile(outside.(string))
			if err != nil {
				t.Fatal(err)
			}
			verify(response)
		})
	}
}

func TestIssuanceOutputRefusesBeforeKeyCreation(t *testing.T) {
	p, input := issuanceCommandPlan(t)
	input["inventory_file"] = filepath.Join(p.Root, "inventory.json")
	if r, err := issuanceCommand(t, "initialize", input, ""); err == nil || r.Phase != "input" || r.Outcome != "invalid-input" {
		t.Fatalf("owned output must refuse before creating keys: %v %v", r, err)
	}
	if _, err := os.Stat(p.Root); !os.IsNotExist(err) {
		t.Fatalf("invalid output created key root: %v", err)
	}
	input["inventory_file"] = p.InventoryFile
	if r, err := issuanceCommand(t, "initialize", input, ""); err != nil || r.Outcome != "completed" {
		t.Fatal("valid initialization after refusal", r, err)
	}
}

func TestIssuanceOutputRefusesOtherOwnedRootBeforeDebit(t *testing.T) {
	_, init, issue, _ := issuanceResultCommandPlan(t)
	if r, err := resultCommand(t, "initialize-results", init, ""); err != nil {
		t.Fatal(r, err)
	}
	foreign := t.TempDir()
	if err := os.Chmod(foreign, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "budget.pin"), []byte("owned state"), 0600); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(issue["admission_root"].(string), "admission.journal")
	before, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	issue["response_file"] = filepath.Join(foreign, "response.bin")
	if r, err := resultCommand(t, "issue", issue, ""); err == nil || r.Phase != "input" || r.Outcome != "storage-unavailable" {
		t.Fatalf("other owned root must refuse before debit: %v %v", r, err)
	}
	after, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("other owned root output consumed quota", err)
	}
	if _, err := os.Stat(issue["response_file"].(string)); !os.IsNotExist(err) {
		t.Fatal("other owned root modified", err)
	}
}

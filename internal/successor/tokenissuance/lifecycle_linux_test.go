//go:build linux

package tokenissuance

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/issuance"
)

func TestInitializeClosesOwnersAndRetainsResultRoot(t *testing.T) {
	base := t.TempDir()
	start := time.Unix(3600, 0).UTC()
	keys := issuance.Binding{Network: [32]byte{1}, Issuer: [32]byte{2}, Signer: [32]byte{3}, Start: start, End: start.Add(time.Hour)}
	p := Plan{AdmissionRoot: filepath.Join(base, "admission"), KeyRoot: filepath.Join(base, "keys"), ResultRoot: filepath.Join(base, "results"), KeyBinding: keys}
	if err := issuance.Initialize(t.Context(), p.KeyRoot, keys); err != nil {
		t.Fatal(err)
	}
	store, err := issuance.Open(t.Context(), p.KeyRoot, keys)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := store.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	p.AdmissionBinding = admission.LedgerBinding{Network: keys.Network, Issuer: keys.Issuer, Authority: [32]byte{4}, Profile: [32]byte{5}, Duty: 1, Start: keys.Start, End: keys.End}
	for _, k := range inventory.Keys {
		p.AdmissionBinding.Keys = append(p.AdmissionBinding.Keys, admission.TokenKey{Window: k.Window, Class: k.Class, SPKI: k.SPKI})
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := admission.Initialize(p.AdmissionRoot, p.AdmissionBinding); err != nil {
		t.Fatal(err)
	}
	if got := Initialize(t.Context(), p); got.Outcome != "initialized-results" {
		t.Fatal(got)
	}
	before, err := os.ReadFile(filepath.Join(p.ResultRoot, "results.pin"))
	if err != nil {
		t.Fatal(err)
	}
	if got := Initialize(t.Context(), p); got.Outcome != "storage-unavailable" {
		t.Fatal(got)
	}
	after, err := os.ReadFile(filepath.Join(p.ResultRoot, "results.pin"))
	if err != nil || string(before) != string(after) {
		t.Fatal("replacement", err)
	}
	if got := Issue(t.Context(), p, nil, admission.Facts{}, admission.Bootstrap); got.Outcome == "busy" || got.Response != nil {
		t.Fatal(got)
	}
	ledger, err := admission.Open(p.AdmissionRoot, p.AdmissionBinding)
	if err != nil {
		t.Fatal(err)
	}
	if err = ledger.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = issuance.Open(t.Context(), p.KeyRoot, keys)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	results, err := issuance.OpenResults(t.Context(), p.ResultRoot, store, p.AdmissionBinding)
	if err != nil {
		t.Fatal(err)
	}
	if err = results.Close(); err != nil {
		t.Fatal(err)
	}
}

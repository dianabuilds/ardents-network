package custody

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAdmissionBundleQuarantineExportAndConfirmedPurgeRetainFloors(t *testing.T) {
	fixture := newAllocationVaultFixture(t)
	operation := fixture.request(t, 1)
	if _, err := fixture.vault.Execute(t.Context(), operation, &sequenceSecrets{values: [][]byte{fixture.password}}); err != nil {
		t.Fatal(err)
	}
	bundlePassword := []byte("distinct encrypted recovery bundle password")
	bundle := filepath.Join(t.TempDir(), "recovery.bundle")
	exported, err := fixture.vault.Execute(t.Context(), Operation{Kind: OperationExportRecoveryBundle, RecordID: fixture.created.RecordID, Expected: fixture.created.Authority.Binding, Path: bundle}, &sequenceSecrets{values: [][]byte{fixture.password, bundlePassword, bundlePassword}})
	if err != nil || !exported.TestRestored {
		t.Fatalf("export/test restore: %+v / %v", exported, err)
	}
	quarantine, err := Open(VaultConfig{Root: t.TempDir(), Now: fixture.vault.now})
	if err != nil {
		t.Fatal(err)
	}
	defer quarantine.Close()

	wrong := fixture.created.Authority.Binding
	wrong.Network[0] ^= 1
	if _, err := quarantine.Execute(t.Context(), Operation{Kind: OperationRestoreRecoveryBundle, Path: bundle, Expected: wrong}, &sequenceSecrets{values: [][]byte{bundlePassword}}); err == nil {
		t.Fatal("wrong recovery binding accepted")
	}
	original, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatal(err)
	}
	altered := bytes.Clone(original)
	altered[len(altered)-4] ^= 1
	if err := os.WriteFile(bundle, altered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := quarantine.Execute(t.Context(), Operation{Kind: OperationRestoreRecoveryBundle, Path: bundle, Expected: fixture.created.Authority.Binding}, &sequenceSecrets{values: [][]byte{bundlePassword}}); err == nil {
		t.Fatal("altered bundle accepted")
	}
	if err := os.WriteFile(bundle, original, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := quarantine.Execute(t.Context(), Operation{Kind: OperationRestoreRecoveryBundle, Path: bundle, Expected: fixture.created.Authority.Binding}, &sequenceSecrets{values: [][]byte{bundlePassword, fixture.password, fixture.password}})
	if err != nil || restored.State != RecordAuthorityLocked {
		t.Fatalf("quarantine: %+v / %v", restored, err)
	}
	if _, err := quarantine.Execute(t.Context(), Operation{Kind: OperationRestoreRecoveryBundle, Path: bundle, Expected: fixture.created.Authority.Binding}, &sequenceSecrets{}); err == nil {
		t.Fatal("existing recovery record replaced")
	}
	operation.RecordID = restored.RecordID
	if _, err := quarantine.Execute(t.Context(), operation, &sequenceSecrets{values: [][]byte{fixture.password}}); err == nil {
		t.Fatal("quarantine signed")
	}
	verified, err := quarantine.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord, RecordID: restored.RecordID, Expected: restored.Authority.Binding}, &sequenceSecrets{values: [][]byte{fixture.password}})
	if err != nil || verified.State != RecordAuthorityLocked {
		t.Fatalf("quarantine verify: %+v / %v", verified, err)
	}
	secondBundle := filepath.Join(t.TempDir(), "terminal-export.bundle")
	if _, err := quarantine.Execute(t.Context(), Operation{Kind: OperationExportRecoveryBundle, RecordID: restored.RecordID, Expected: restored.Authority.Binding, Path: secondBundle}, &sequenceSecrets{values: [][]byte{fixture.password, bundlePassword, bundlePassword}}); err != nil {
		t.Fatalf("quarantine re-export: %v", err)
	}
	floor, err := os.ReadFile(quarantine.floors)
	if err != nil {
		t.Fatal(err)
	}
	purge := Operation{Kind: OperationPurgeVaultRecord, RecordID: restored.RecordID, Expected: restored.Authority.Binding}
	if _, err := quarantine.Execute(t.Context(), purge, &sequenceSecrets{values: [][]byte{fixture.password}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unconfirmed purge: %v", err)
	}
	if _, err := quarantine.Execute(t.Context(), purge, &sequenceSecrets{values: [][]byte{fixture.password}, confirm: true}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(quarantine.floors)
	if err != nil || !bytes.Equal(floor, after) {
		t.Fatalf("purge lowered floors: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(quarantine.quarantine, "record-"+restored.RecordID+".json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("purge retained selected record: %v", err)
	}
}

func TestAdmissionRecoveryRefusesRollbackAfterActivePurge(t *testing.T) {
	f := newAllocationVaultFixture(t)
	bundlePassword := []byte("distinct old bundle recovery password")
	bundle := filepath.Join(t.TempDir(), "old.bundle")
	export := Operation{Kind: OperationExportRecoveryBundle, RecordID: f.created.RecordID, Expected: f.created.Authority.Binding, Path: bundle}
	if _, err := f.vault.Execute(t.Context(), export, &sequenceSecrets{values: [][]byte{f.password, bundlePassword, bundlePassword}}); err != nil {
		t.Fatal(err)
	}
	retained, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.vault.Execute(t.Context(), export, &sequenceSecrets{values: [][]byte{f.password, bundlePassword, bundlePassword}}); err == nil {
		t.Fatal("unconfirmed existing Bundle replaced")
	}
	after, err := os.ReadFile(bundle)
	if err != nil || !bytes.Equal(retained, after) {
		t.Fatal("refused replacement changed Bundle")
	}
	if _, err := f.vault.Execute(t.Context(), f.request(t, 1), &sequenceSecrets{values: [][]byte{f.password}}); err != nil {
		t.Fatal(err)
	}
	floor, err := readSmallFile(f.vault.floors)
	if err != nil {
		t.Fatal(err)
	}
	purge := Operation{Kind: OperationPurgeVaultRecord, RecordID: f.created.RecordID, Expected: f.created.Authority.Binding}
	if _, err := f.vault.Execute(t.Context(), purge, &sequenceSecrets{values: [][]byte{f.password}, confirm: true}); err != nil {
		t.Fatal(err)
	}
	ledger, _ := admissionLedgerPath(f.vault.root, f.created.RecordID)
	if _, err := os.Stat(ledger); !os.IsNotExist(err) {
		t.Fatal("purge retained private allocation ledger")
	}
	if _, err := f.vault.Execute(t.Context(), Operation{Kind: OperationRestoreRecoveryBundle, Path: bundle, Expected: f.created.Authority.Binding}, &sequenceSecrets{values: [][]byte{bundlePassword}}); err == nil {
		t.Fatal("old Bundle reset retained allocation floor")
	}
	after, err = readSmallFile(f.vault.floors)
	if err != nil || !bytes.Equal(floor, after) {
		t.Fatal("rollback refusal reset floor")
	}
}

func TestRetainedNameAndServiceBundlesRemainEvidenceOnly(t *testing.T) {
	for _, kind := range []AuthorityKind{AuthorityName, AuthorityService} {
		t.Run(string(kind), func(t *testing.T) {
			v, err := Open(VaultConfig{Root: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			defer v.Close()
			password := []byte("generic evidence recovery password")
			vaultPassword := []byte("generic evidence vault password")
			state := authorityState{Binding: AuthorityBinding{Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}, Kind: kind, IDCommitment: [32]byte{4}}, RootMaterial: []byte("retained opaque historical authority material"), Generation: 7, Revision: 6, Watermarks: []Watermark{{Domain: "retained-history", Value: 6}}}
			plain, err := encodeAuthorityState(PurposeBundle, state)
			if err != nil {
				t.Fatal(err)
			}
			defer zero(plain)
			raw, err := sealEnvelope(PurposeBundle, plain, password)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "historical.bundle")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			restored, err := v.Execute(t.Context(), Operation{Kind: OperationRestoreRecoveryBundle, Path: path, Expected: state.Binding}, &sequenceSecrets{values: [][]byte{password, vaultPassword, vaultPassword}})
			if err != nil {
				t.Fatal(err)
			}
			verify := Operation{Kind: OperationVerifyVaultRecord, RecordID: restored.RecordID, Expected: state.Binding}
			if r, err := v.Execute(t.Context(), verify, &sequenceSecrets{values: [][]byte{vaultPassword}}); err != nil || r.State != RecordAuthorityLocked || r.Authority.Binding.Kind != kind {
				t.Fatalf("retained evidence changed: %v", err)
			}
			if _, err := v.Execute(t.Context(), Operation{Kind: OperationIssueAdmissionPermission, RecordID: restored.RecordID, Expected: state.Binding}, &sequenceSecrets{}); err == nil {
				t.Fatal("retired/foreign purpose signed")
			}
			if _, err := v.Execute(t.Context(), Operation{Kind: "activate-recovered-authority", RecordID: restored.RecordID, Expected: state.Binding}, &sequenceSecrets{}); err == nil {
				t.Fatal("retired activation returned")
			}
			if _, err := v.Execute(t.Context(), Operation{Kind: OperationExportRecoveryBundle, RecordID: restored.RecordID, Expected: state.Binding, Path: filepath.Join(t.TempDir(), "retained.bundle")}, &sequenceSecrets{values: [][]byte{vaultPassword, password, password}}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

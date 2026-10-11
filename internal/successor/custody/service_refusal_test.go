package custody

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

func TestServiceRefusalCannotPublishOrChangeFloor(t *testing.T) {
	f := newServiceVaultFixture(t)
	original := f.request(f.now, f.now.Add(time.Hour), f.created.RecordID)
	floor, err := os.ReadFile(f.vault.floors)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"purpose", "authority", "network", "environment", "root", "request", "approval", "password", "oversized-password", "oversized-request", "horizon", "zero", "unsupported"} {
		t.Run(name, func(t *testing.T) {
			op := original
			op.ServiceRequest = bytes.Clone(original.ServiceRequest)
			password := f.password
			preUnlock := true
			switch name {
			case "purpose":
				op.Expected.Kind = AuthorityAdmission
			case "authority":
				op.Expected.IDCommitment[0] ^= 1
				preUnlock = false
			case "network":
				op.Expected.Network[0] ^= 1
			case "environment":
				op.Expected.Environment[0] ^= 1
				preUnlock = false
			case "root":
				op.Expected.Root[0] ^= 1
				preUnlock = false
			case "request":
				op.ServiceRequest[len(op.ServiceRequest)-1] ^= 1
				op.ServiceRequestCommitment = sha256.Sum256(op.ServiceRequest)
			case "approval":
				op.ServiceRequestCommitment[0] ^= 1
			case "password":
				password = []byte("wrong Service custody password")
				preUnlock = false
			case "oversized-password":
				password = make([]byte, 1025)
				preUnlock = false
			case "oversized-request":
				op.ServiceRequest = make([]byte, 1025)
			case "horizon":
				op = f.request(f.now.Add(48*time.Hour), f.now.Add(49*time.Hour), original.RecordID)
			case "zero":
				op = Operation{}
			case "unsupported":
				op.Kind = "sign"
			}
			secrets := &sequenceSecrets{values: [][]byte{password}}
			r, err := f.vault.Execute(t.Context(), op, secrets)
			if err == nil || len(r.ServiceResponse) != 0 || r.RecordID != "" {
				t.Fatal("refusal exposed result")
			}
			if preUnlock && len(secrets.values) != 1 {
				t.Fatal("public refusal reached password prompt")
			}
			after, err := os.ReadFile(f.vault.floors)
			if err != nil || !bytes.Equal(after, floor) {
				t.Fatal("refusal changed floor", err)
			}
			entries, err := os.ReadDir(f.vault.records)
			if err != nil || len(entries) != 1 {
				t.Fatal("refusal published successor", err)
			}
		})
	}
	if err := os.Remove(filepath.Join(f.root, "independent-root")); err != nil {
		t.Fatal(err)
	}
	secrets := &sequenceSecrets{values: [][]byte{f.password}}
	if _, err := f.vault.Execute(t.Context(), original, secrets); err == nil || len(secrets.values) != 1 {
		t.Fatal("foreign root reached signing/unlock")
	}
	if err := f.vault.admitIndependentRoot(); err == nil {
		t.Fatal("predecessor root adopted")
	}
}

func TestServiceQuarantineCannotSignAndPurgeRetainsFloor(t *testing.T) {
	f := newServiceVaultFixture(t)
	path := filepath.Join(t.TempDir(), "service.bundle")
	password := []byte("independent Service recovery password")
	exported, err := f.vault.Execute(t.Context(), Operation{Kind: OperationExportRecoveryBundle, RecordID: f.created.RecordID, Expected: f.created.Authority.Binding, Path: path}, &sequenceSecrets{values: [][]byte{f.password, password, password}})
	if err != nil || !exported.TestRestored {
		t.Fatal("Service bundle", err)
	}
	recovered, err := Open(VaultConfig{Root: t.TempDir(), Now: func() time.Time { return f.now }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recovered.Close() })
	locked, err := recovered.Execute(t.Context(), Operation{Kind: OperationRestoreRecoveryBundle, Path: path, Expected: f.created.Authority.Binding}, &sequenceSecrets{values: [][]byte{password, f.password, f.password}})
	if err != nil || locked.State != RecordAuthorityLocked {
		t.Fatal("Service quarantine", err)
	}
	op := f.request(f.now, f.now.Add(time.Hour), locked.RecordID)
	if r, err := recovered.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}}); err == nil || len(r.ServiceResponse) != 0 {
		t.Fatal("quarantine signed")
	}
	floor, err := os.ReadFile(recovered.floors)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recovered.Execute(t.Context(), Operation{Kind: OperationPurgeVaultRecord, RecordID: locked.RecordID, Expected: f.created.Authority.Binding}, &sequenceSecrets{values: [][]byte{f.password}, confirm: true}); err != nil {
		t.Fatal("quarantine purge", err)
	}
	after, err := os.ReadFile(recovered.floors)
	if err != nil || !bytes.Equal(floor, after) {
		t.Fatal("purge changed retained floor", err)
	}
	view, err := publication.ParseServiceRequest(op.ServiceRequest)
	if err != nil || view.NetworkID != f.created.Authority.Binding.Network {
		t.Fatal("request test premise", err)
	}
}

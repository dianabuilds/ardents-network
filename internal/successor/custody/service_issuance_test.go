package custody

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

type serviceVaultFixture struct {
	vault    *Vault
	created  Receipt
	password []byte
	now      time.Time
	root     string
}

func newServiceVaultFixture(t *testing.T) *serviceVaultFixture {
	t.Helper()
	f := &serviceVaultFixture{password: []byte("independent Service authority password"), now: time.Unix(1900000000, 0).UTC(), root: t.TempDir()}
	var err error
	f.vault, err = Open(VaultConfig{Root: f.root, Now: func() time.Time { return f.now }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.vault.Close() })
	f.created, err = f.vault.Execute(t.Context(), Operation{Kind: OperationCreateServiceAuthority,
		Binding: AuthorityBinding{Kind: AuthorityService, Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}}}, &sequenceSecrets{values: [][]byte{f.password, f.password}})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *serviceVaultFixture) request(before, after time.Time, record string) Operation {
	view := publication.RequestView{NetworkID: f.created.Authority.Binding.Network, InstancePublic: [32]byte{4}, NotBefore: before.Unix(), NotAfter: after.Unix()}
	view.Commitment = publication.ServiceRequestCommitment(view)
	raw := publication.EncodeServiceRequest(view)
	return Operation{Kind: OperationIssueServiceCredential, RecordID: record, Expected: f.created.Authority.Binding, ServiceRequest: raw, ServiceRequestCommitment: sha256.Sum256(raw)}
}

func TestServiceIssuanceExactRetryGenerationAndIndependentReopen(t *testing.T) {
	f := newServiceVaultFixture(t)
	op := f.request(f.now, f.now.Add(time.Hour), f.created.RecordID)
	issue := func(op Operation) Receipt {
		t.Helper()
		receipt, err := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}})
		if err != nil {
			t.Fatal(err)
		}
		return receipt
	}
	first := issue(op)
	view, err := publication.ParseServiceRequest(op.ServiceRequest)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := publication.VerifyServiceResponse(first.ServiceResponse, view)
	if err != nil || verified.Delegation().Generation != 1 || verified.Delegation().Authority != f.created.ServiceAuthority.Public {
		t.Fatal("public verification", err)
	}
	path := filepath.Join(f.vault.records, "record-"+first.RecordID+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.vault.Close(); err != nil {
		t.Fatal(err)
	}
	f.vault, err = Open(VaultConfig{Root: f.root, Now: func() time.Time { return f.now }})
	if err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(2 * time.Hour)
	retry := issue(op)
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || !bytes.Equal(first.ServiceResponse, retry.ServiceResponse) {
		t.Fatal("expired retry changed response/envelope", err)
	}
	wrong := op
	wrong.ServiceRequestCommitment[0] ^= 1
	if _, err := f.vault.Execute(t.Context(), wrong, &sequenceSecrets{}); !errors.Is(err, ErrInvalid) {
		t.Fatal("wrong approval reached unlock", err)
	}
	overlap := f.request(time.Unix(view.NotBefore+10, 0), time.Unix(view.NotAfter+10, 0), first.RecordID)
	if _, err := f.vault.Execute(t.Context(), overlap, &sequenceSecrets{values: [][]byte{f.password}}); !errors.Is(err, ErrInvalid) {
		t.Fatal("overlap accepted", err)
	}
	next := issue(f.request(f.now, f.now.Add(time.Hour), first.RecordID))
	if next.Authority.Generation != 3 || next.Authority.Watermarks[0].Value != 2 {
		t.Fatal("generation did not advance exactly once")
	}
	if _, err := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}}); !errors.Is(err, ErrInvalid) {
		t.Fatal("rollback retry accepted", err)
	}
}

func TestServiceExpiryAfterUnlockAndEncryptionCreatesNoSuccessor(t *testing.T) {
	for _, stage := range []string{"unlock", "seal"} {
		t.Run(stage, func(t *testing.T) {
			f := newServiceVaultFixture(t)
			op := f.request(f.now, f.now.Add(time.Hour), f.created.RecordID)
			calls := 0
			f.vault.now = func() time.Time {
				calls++
				if stage == "seal" && calls == 3 {
					f.now = f.now.Add(2 * time.Hour)
				}
				return f.now
			}
			_, err := f.vault.Execute(t.Context(), op, advancingSecret{f.password, func() {
				if stage == "unlock" {
					f.now = f.now.Add(2 * time.Hour)
				}
			}})
			if !errors.Is(err, ErrInvalid) {
				t.Fatal("expiry accepted", err)
			}
			view, err := publication.ParseServiceRequest(op.ServiceRequest)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(f.vault.records, "record-"+serviceSuccessorRecordID(op.RecordID, view.Commitment)+".json")
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("elapsed first write", err)
			}
			floors, err := f.vault.readFloors()
			floor, found := floorFor(floors, op.Expected)
			if err != nil || !found || floor.Generation != 1 || floor.Revision != 0 {
				t.Fatal("elapsed floor advanced", err)
			}
		})
	}
}

func TestServiceCopiedSuccessorCannotAuthorizeForeignRequest(t *testing.T) {
	f := newServiceVaultFixture(t)
	op := f.request(f.now, f.now.Add(time.Hour), f.created.RecordID)
	first, err := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}})
	if err != nil {
		t.Fatal(err)
	}
	view, err := publication.ParseServiceRequest(op.ServiceRequest)
	if err != nil {
		t.Fatal(err)
	}
	view.InstancePublic[0] ^= 1
	view.Commitment = publication.ServiceRequestCommitment(view)
	foreign := op
	foreign.ServiceRequest = publication.EncodeServiceRequest(view)
	foreign.ServiceRequestCommitment = sha256.Sum256(foreign.ServiceRequest)
	persisted, err := os.ReadFile(filepath.Join(f.vault.records, "record-"+first.RecordID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	// Copying authentic ciphertext to the deterministic name of another request
	// cannot replace an authenticated request journal, even with matching floors.
	foreignPath := filepath.Join(f.vault.records, "record-"+serviceSuccessorRecordID(op.RecordID, view.Commitment)+".json")
	if err := os.WriteFile(foreignPath, persisted, 0o600); err != nil {
		t.Fatal(err)
	}
	if r, err := f.vault.Execute(t.Context(), foreign, &sequenceSecrets{values: [][]byte{f.password}}); err == nil || len(r.ServiceResponse) != 0 {
		t.Fatal("copied successor authorized a foreign request at the same generation")
	}
}

func TestServiceConcurrentExactIssuanceRetainsOneGeneration(t *testing.T) {
	f := newServiceVaultFixture(t)
	op := f.request(f.now, f.now.Add(time.Hour), f.created.RecordID)
	type result struct {
		receipt Receipt
		err     error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			r, err := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}})
			results <- result{r, err}
		}()
	}
	close(start)
	one, two := <-results, <-results
	if one.err != nil || two.err != nil || !bytes.Equal(one.receipt.ServiceResponse, two.receipt.ServiceResponse) || one.receipt.RecordID != two.receipt.RecordID {
		t.Fatal("concurrent exact issuance diverged", one.err, two.err)
	}
	floors, err := f.vault.readFloors()
	floor, found := floorFor(floors, op.Expected)
	if err != nil || !found || floor.Generation != 2 || floor.Revision != 1 {
		t.Fatal("concurrent issuance advanced more than once", err)
	}
}

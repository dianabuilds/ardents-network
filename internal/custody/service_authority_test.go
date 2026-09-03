package custody

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func TestCreateServiceAuthorityReturnsPublicIdentityAndRetainsEncryptedRoot(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	var binding AuthorityBinding
	for index := range binding.Environment {
		binding.Environment[index] = byte(index + 1)
		binding.Network[index] = byte(index + 2)
		binding.Root[index] = byte(index + 3)
	}
	binding.Kind = AuthorityService
	password := []byte("service authority vault password")
	receipt, err := vault.Execute(t.Context(), Operation{
		Kind:      OperationCreateServiceAuthority,
		Authority: AuthorityState{Binding: binding},
	}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create Service Authority: %v", err)
	}
	if receipt.Operation != OperationCreateServiceAuthority || receipt.RecordID == "" || receipt.State != RecordActive {
		t.Fatalf("creation receipt = %+v", receipt)
	}
	if receipt.Authority.Binding.Kind != AuthorityService || receipt.Authority.Binding.IDCommitment == [32]byte{} ||
		receipt.ServiceAuthority.Public == [ed25519.PublicKeySize]byte{} {
		t.Fatalf("public Authority receipt = %+v", receipt)
	}
	if receipt.ServiceAuthority.Target != publication.Target(receipt.ServiceAuthority.Public) {
		t.Fatal("Service Target was not derived from the created Authority")
	}
	verified, err := vault.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord,
		RecordID: receipt.RecordID, Expected: receipt.Authority.Binding}, &sequenceSecrets{values: [][]byte{password}})
	if err != nil || verified.State != RecordActive {
		t.Fatalf("verify encrypted Service Authority: %+v / %v", verified, err)
	}
}

func TestIssueServiceCredentialAdvancesAuthorityAndRetriesExactRequest(t *testing.T) {
	now := time.Date(2026, time.September, 1, 2, 3, 4, 0, time.UTC)
	clock := now
	vault, err := Open(VaultConfig{Root: t.TempDir(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	binding := AuthorityBinding{Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}, Kind: AuthorityService}
	password := []byte("service authority vault password")
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateServiceAuthority,
		Authority: AuthorityState{Binding: binding}}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatal(err)
	}
	request := serviceInstanceRequest(t, binding.Network, now, now.Add(time.Hour))
	issue := Operation{Kind: OperationIssueServiceCredential, RecordID: created.RecordID,
		Expected: created.Authority.Binding, ServiceRequest: request, ServiceRequestCommitment: sha256.Sum256(request)}
	first, err := vault.Execute(t.Context(), issue, &sequenceSecrets{values: [][]byte{password}})
	if err != nil {
		t.Fatalf("issue first Service Credential: %v", err)
	}
	retried, err := vault.Execute(t.Context(), issue, &sequenceSecrets{values: [][]byte{password}})
	if err != nil {
		t.Fatalf("retry first Service Credential: %v", err)
	}
	if first.RecordID == created.RecordID || retried.RecordID != first.RecordID ||
		!bytes.Equal(retried.ServiceResponse, first.ServiceResponse) {
		t.Fatalf("issuance retry changed successor: first=%+v retried=%+v", first, retried)
	}
	response, err := instance.ParseResponse(first.ServiceResponse)
	if err != nil || response.RequestCommitment == [32]byte{} || response.Credential.Generation != 1 {
		t.Fatalf("first response = %+v / %v", response, err)
	}
	if err := publication.Validate(response.Credential, created.ServiceAuthority.Public, binding.Network,
		now.Add(time.Minute), publication.CapabilityPublish|publication.CapabilityConnect); err != nil {
		t.Fatal(err)
	}
	different := serviceInstanceRequest(t, binding.Network, now.Add(30*time.Minute), now.Add(90*time.Minute))
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationIssueServiceCredential, RecordID: created.RecordID,
		Expected: created.Authority.Binding, ServiceRequest: different, ServiceRequestCommitment: sha256.Sum256(different)},
		&sequenceSecrets{values: [][]byte{password}}); err == nil {
		t.Fatal("stale Authority record issued a different successor")
	}
	request2 := serviceInstanceRequest(t, binding.Network, now.Add(time.Hour), now.Add(2*time.Hour))
	issue2 := Operation{Kind: OperationIssueServiceCredential, RecordID: first.RecordID,
		Expected: created.Authority.Binding, ServiceRequest: request2, ServiceRequestCommitment: sha256.Sum256(request2)}
	second, err := vault.Execute(t.Context(), issue2, &sequenceSecrets{values: [][]byte{password}})
	if err != nil {
		t.Fatalf("issue successor Service Credential: %v", err)
	}
	response2, err := instance.ParseResponse(second.ServiceResponse)
	if err != nil || response2.Credential.Generation != 2 || response2.Credential.NotBefore != now.Add(time.Hour).Unix() {
		t.Fatalf("successor response = %+v / %v", response2, err)
	}
	clock = now.Add(3 * time.Hour)
	expiredRetry, err := vault.Execute(t.Context(), issue2, &sequenceSecrets{values: [][]byte{password}})
	if err != nil || expiredRetry.RecordID != second.RecordID ||
		!bytes.Equal(expiredRetry.ServiceResponse, second.ServiceResponse) {
		t.Fatalf("expired exact retry changed successor: retry=%+v / %v", expiredRetry, err)
	}
}

func TestIssueServiceCredentialRejectsUnboundedValidityBeforePassword(t *testing.T) {
	now := time.Date(2026, time.September, 1, 2, 3, 4, 0, time.UTC)
	vault, err := Open(VaultConfig{Root: t.TempDir(), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	binding := AuthorityBinding{Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}, Kind: AuthorityService}
	password := []byte("bounded service authority password")
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateServiceAuthority,
		Authority: AuthorityState{Binding: binding}}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatal(err)
	}
	request := serviceInstanceRequest(t, binding.Network, now, now.Add(24*time.Hour+time.Second))
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationIssueServiceCredential,
		RecordID: created.RecordID, Expected: created.Authority.Binding, ServiceRequest: request,
		ServiceRequestCommitment: sha256.Sum256(request)}, unreadSecrets{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbounded Service Credential request = %v, want invalid before password", err)
	}
	future := serviceInstanceRequest(t, binding.Network, now.Add(49*time.Hour), now.Add(50*time.Hour))
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationIssueServiceCredential,
		RecordID: created.RecordID, Expected: created.Authority.Binding, ServiceRequest: future,
		ServiceRequestCommitment: sha256.Sum256(future)}, unreadSecrets{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("far-future Service Credential request = %v, want invalid before password", err)
	}
	expired := serviceInstanceRequest(t, binding.Network, now.Add(-2*time.Hour), now.Add(-time.Hour))
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationIssueServiceCredential,
		RecordID: created.RecordID, Expected: created.Authority.Binding, ServiceRequest: expired,
		ServiceRequestCommitment: sha256.Sum256(expired)}, unreadSecrets{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("new expired Service Credential request = %v, want invalid before password", err)
	}
}

func serviceInstanceRequest(t *testing.T, network [32]byte, notBefore, notAfter time.Time) []byte {
	t.Helper()
	root, err := instance.Initialize(instance.InitializeConfig{Root: serviceInstanceFixtureRoot(t), NetworkID: network,
		NotBefore: notBefore, NotAfter: notAfter})
	if err != nil {
		t.Fatal(err)
	}
	request, err := root.Request()
	if closeErr := root.Close(); err != nil || closeErr != nil {
		t.Fatalf("read Service Instance request: %v / %v", err, closeErr)
	}
	return request
}

func serviceInstanceFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "service-instance-root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

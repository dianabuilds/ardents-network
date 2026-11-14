//go:build linux && descriptor_installed_native

package descriptor_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/custody"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
)

type descriptorNativeProofs struct {
	target                                   [32]byte
	first, conflict, second, foreign, future []byte
}

// This is external signed public input to a receiving Descriptor. The genuine
// Custody authority issues a Credential; a separate fixture Instance signs the
// public proof. No live Publisher, registration, slot or readiness is claimed.
func prepareDescriptorNativeProofs(t *testing.T, prepared descriptorPreparedStock, window time.Duration) descriptorNativeProofs {
	t.Helper()
	if window != 5*time.Minute && window != 10*time.Minute {
		t.Fatal("fixed native proof input interval required")
	}
	password := make([]byte, 32)
	if _, err := rand.Read(password); err != nil {
		t.Fatal(err)
	}
	defer clear(password)
	vault, err := custody.Open(custody.VaultConfig{Root: filepath.Join(descriptorNativeParent, "service-vault")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := vault.Close(); err != nil {
			t.Error(err)
		}
	}()
	secrets := descriptorTokenTestSecrets{password}
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateServiceAuthority, Binding: custody.AuthorityBinding{Kind: custody.AuthorityService, Environment: sha256.Sum256([]byte("installed Descriptor signed input")), Network: prepared.facts.NetworkID, Root: sha256.Sum256([]byte("separate Service authority"))}}, secrets)
	if err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	view, err := prepared.current()
	if err != nil {
		t.Fatal(err)
	}
	now := view.ObservedAt().UTC().Truncate(time.Second)
	end := now.Add(window)
	if end.After(view.Profile().NotAfter) {
		end = view.Profile().NotAfter
	}
	var instance [32]byte
	copy(instance[:], public)
	request := publication.RequestView{NetworkID: prepared.facts.NetworkID, InstancePublic: instance, NotBefore: now.Unix(), NotAfter: end.Unix()}
	request.Commitment = publication.ServiceRequestCommitment(request)
	raw := publication.EncodeServiceRequest(request)
	issued, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationIssueServiceCredential, RecordID: created.RecordID, Expected: created.Authority.Binding, ServiceRequest: raw, ServiceRequestCommitment: sha256.Sum256(raw)}, secrets)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := publication.VerifyServiceResponse(issued.ServiceResponse, request)
	if err != nil || credential.Delegation().Authority != created.ServiceAuthority.Public {
		t.Fatal("actual Service Credential", err)
	}
	const credentialSize = 222
	if len(issued.ServiceResponse) != len("ardents-service-instance-response-v3\x00")+32+credentialSize {
		t.Fatal("independent Credential framing differs")
	}
	record := append([]byte("ardents-service-publication-v3\x00"), issued.ServiceResponse[len(issued.ServiceResponse)-credentialSize:]...)
	record = append(record, make([]byte, 32)...)
	digest := sha256.Sum256(record)
	record = append(record, ed25519.Sign(private, digest[:])...)
	target := publication.Target(created.ServiceAuthority.Public)
	if _, err := publication.VerifyPublish(record, target, prepared.facts.NetworkID, time.Now()); err != nil {
		t.Fatal("independent public Publication proof", err)
	}
	var introduction [32]byte
	for _, m := range view.Members() {
		if m.RoleDomain == 4 && m.Subrole == 3 {
			introduction = m.NodeID
			break
		}
	}
	if introduction == ([32]byte{}) {
		t.Fatal("actual assigned Introduction absent")
	}
	var slot, recipient [32]byte
	if _, err := rand.Read(slot[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(recipient[:]); err != nil {
		t.Fatal(err)
	}
	makeProof := func(revision uint64, profile [32]byte, slot [32]byte, before, at time.Time) []byte {
		t.Helper()
		draft, err := reachability.PrepareDescriptor(record, target, prepared.facts.NetworkID, profile, reachability.Introduction{Revision: revision, Node: introduction, Slot: slot, RecipientKey: recipient, NotBefore: before, NotAfter: end}, at)
		if err != nil {
			t.Fatal(err)
		}
		proof, err := draft.Complete(ed25519.Sign(private, draft.Transcript()), at)
		if err != nil {
			t.Fatal(err)
		}
		return proof.Bytes()
	}
	other := slot
	other[0] ^= 1
	foreign := prepared.facts.Digest
	foreign[0] ^= 1
	return descriptorNativeProofs{target: target, first: makeProof(1, prepared.facts.Digest, slot, now, time.Now()), conflict: makeProof(1, prepared.facts.Digest, other, now, time.Now()), second: makeProof(2, prepared.facts.Digest, slot, now, time.Now()), foreign: makeProof(3, foreign, slot, now, time.Now()), future: makeProof(3, prepared.facts.Digest, slot, now.Add(2*time.Minute), now.Add(2*time.Minute))}
}

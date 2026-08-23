package custody

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestVaultSignsOneSealedNamespaceTransitionWithoutReleasingRoot(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	network := [32]byte{19}
	seed := sha256.Sum256([]byte("custody-name-authority"))
	private := ed25519.NewKeyFromSeed(seed[:])
	public := private.Public().(ed25519.PublicKey)
	state := testAuthorityState()
	state.Binding.Kind, state.Binding.Network = AuthorityName, network
	state.Binding.IDCommitment = sha256.Sum256(public)
	state.RootMaterial = append([]byte(nil), private...)
	state.Generation, state.Revision = 1, 2
	password := []byte("correct horse battery staple")
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: state}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create name authority record: %v", err)
	}
	current := namespace.Record{Name: "alice", Generation: 1, Revision: 2, Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(public), Target: [32]byte{1}, LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000, Continuity: 1}
	op := namespace.Op{Kind: "renew", Name: current.Name, Authority: current.Authority,
		ExpectedGeneration: current.Generation, ExpectedRevision: current.Revision, LeaseDuration: 2 * time.Hour}
	signed, err := vault.Execute(t.Context(), Operation{Kind: OperationSignNamespaceTransition, RecordID: created.RecordID, Expected: state.Binding,
		Transition: func(signer namespace.TransitionSigner) ([]byte, error) {
			return namespace.SignTransitionWith(network, current, op, signer)
		}}, &sequenceSecrets{values: [][]byte{password}})
	if err != nil {
		t.Fatalf("sign sealed Namespace transition: %v", err)
	}
	if signed.Operation != OperationSignNamespaceTransition || len(signed.Proof) != ed25519.SignatureSize || bytes.Contains(signed.Proof, state.RootMaterial) {
		t.Fatalf("unexpected signing receipt: %+v", signed)
	}
	digest, err := namespace.TransitionDigest(network, current, op)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := namespace.NewAdmission([32]byte{1}, network, 1, [32]byte{2})
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := admission.Issue(100_000, "renewal-update", digest, [32]byte{3}, 110_000, [16]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	admissionProof, _ := challenge.Solve()
	updated, err := namespace.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, current, op, signed.Proof, 101, namespace.Policy{})
	if err != nil || updated.Revision != current.Revision+1 {
		t.Fatalf("apply custody proof: updated=%+v err=%v", updated, err)
	}
	stale := current
	stale.Revision--
	staleOp := op
	staleOp.ExpectedRevision = stale.Revision
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationSignNamespaceTransition, RecordID: created.RecordID, Expected: state.Binding,
		Transition: func(signer namespace.TransitionSigner) ([]byte, error) {
			return namespace.SignTransitionWith(network, stale, staleOp, signer)
		}}, &sequenceSecrets{values: [][]byte{password}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("stale predecessor signing = %v, want invalid", err)
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationSignNamespaceTransition, RecordID: created.RecordID, Expected: state.Binding,
		Transition: func(namespace.TransitionSigner) ([]byte, error) { return []byte("not a sealed Namespace proof"), nil }}, &sequenceSecrets{values: [][]byte{password}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("generic signing callback = %v, want invalid", err)
	}
}

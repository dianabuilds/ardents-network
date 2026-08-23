package custody

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/authority"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/epoch"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
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
	current := record.Record{Name: "alice", Generation: 1, Revision: 2, Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(public), Target: [32]byte{1}, LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000, Continuity: 1}
	op := record.Op{Kind: "renew", Name: current.Name, Authority: current.Authority,
		ExpectedGeneration: current.Generation, ExpectedRevision: current.Revision, LeaseDuration: 2 * time.Hour}
	signed, err := vault.Execute(t.Context(), Operation{Kind: OperationSignNamespaceTransition, RecordID: created.RecordID, Expected: state.Binding,
		Transition: func(signer authority.TransitionSigner) ([]byte, error) {
			return authority.SignTransitionWith(network, current, op, signer)
		}}, &sequenceSecrets{values: [][]byte{password}})
	if err != nil {
		t.Fatalf("sign sealed Namespace transition: %v", err)
	}
	if signed.Operation != OperationSignNamespaceTransition || len(signed.Proof) != ed25519.SignatureSize || bytes.Contains(signed.Proof, state.RootMaterial) {
		t.Fatalf("unexpected signing receipt: %+v", signed)
	}
	digest, err := authority.TransitionDigest(network, current, op)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := admission.NewAdmission([32]byte{1}, network, 1, [32]byte{2})
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := admission.Issue(100_000, "renewal-update", digest, [32]byte{3}, 110_000, [16]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	admissionProof, _ := challenge.Solve()
	updated, err := authority.ApplyAdmittedTransition(admission, admissionProof, 100_000,
		admissionProof.Challenge.OperationDigest, network, current, op, signed.Proof, 101, record.Policy{})
	if err != nil || updated.Revision != current.Revision+1 {
		t.Fatalf("apply custody proof: updated=%+v err=%v", updated, err)
	}
	stale := current
	stale.Revision--
	staleOp := op
	staleOp.ExpectedRevision = stale.Revision
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationSignNamespaceTransition, RecordID: created.RecordID, Expected: state.Binding,
		Transition: func(signer authority.TransitionSigner) ([]byte, error) {
			return authority.SignTransitionWith(network, stale, staleOp, signer)
		}}, &sequenceSecrets{values: [][]byte{password}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("stale predecessor signing = %v, want invalid", err)
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationSignNamespaceTransition, RecordID: created.RecordID, Expected: state.Binding,
		Transition: func(authority.TransitionSigner) ([]byte, error) { return []byte("not a sealed Namespace proof"), nil }}, &sequenceSecrets{values: [][]byte{password}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("generic signing callback = %v, want invalid", err)
	}
}

func TestVaultPreparesOneCompleteCustodyDerivedNamespaceSubmission(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	now, network := time.Unix(1_800_001_100, 0).UTC(), [32]byte{20}
	seed := sha256.Sum256([]byte("custody-complete-name-control"))
	private := ed25519.NewKeyFromSeed(seed[:])
	public := private.Public().(ed25519.PublicKey)
	state := testAuthorityState()
	state.Binding.Kind, state.Binding.Network = AuthorityName, network
	state.Binding.IDCommitment = sha256.Sum256(public)
	state.RootMaterial, state.Generation, state.Revision = append([]byte(nil), private...), 1, 2
	password := []byte("correct horse battery staple")
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: state},
		&sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create name authority record: %v", err)
	}
	current := record.Record{Name: "alice", Generation: 1, Revision: 2, Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(public), Target: [32]byte{1}, LeaseExpiresAt: now.Add(time.Hour).Unix(),
		GraceExpiresAt: now.Add(2 * time.Hour).Unix(), Continuity: 1}
	op := record.Op{Kind: "renew", Name: current.Name, Authority: current.Authority,
		ExpectedGeneration: current.Generation, ExpectedRevision: current.Revision, LeaseDuration: time.Hour}
	var callbackErr error
	receipt, err := vault.Execute(t.Context(), Operation{Kind: OperationPrepareNamespaceSubmission,
		RecordID: created.RecordID, Expected: state.Binding, Preparation: func(signer authority.ControlSigner) (authority.Submission, error) {
			proof, signErr := authority.SignTransitionWith(network, current, op, transitionAdapter{signer: signer})
			if signErr != nil {
				callbackErr = signErr
				return authority.Submission{}, signErr
			}
			updated, applyErr := record.ApplyLegacy(&current, now.Unix(), op, record.Policy{})
			if applyErr != nil {
				callbackErr = applyErr
				return authority.Submission{}, applyErr
			}
			successor, recordErr := record.SignWith(network, updated, recordAdapter{signer: signer})
			if recordErr != nil {
				callbackErr = recordErr
				return authority.Submission{}, recordErr
			}
			submission, submissionErr := custodyDerivedSubmission(current, now, proof, successor)
			callbackErr = submissionErr
			return submission, submissionErr
		}}, &sequenceSecrets{values: [][]byte{password}})
	if err != nil {
		t.Fatalf("prepare custody-derived submission: %v (callback=%v)", err, callbackErr)
	}
	if receipt.Operation != OperationPrepareNamespaceSubmission || len(receipt.Submission) == 0 || bytes.Contains(receipt.Submission, state.RootMaterial) {
		t.Fatalf("unexpected preparation receipt: %+v", receipt)
	}
	if _, err := authority.OpenSubmission(receipt.Submission); err != nil {
		t.Fatalf("prepared submission is not canonical: %v", err)
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationPrepareNamespaceSubmission,
		RecordID: created.RecordID, Expected: state.Binding, Preparation: func(authority.ControlSigner) (authority.Submission, error) {
			return authority.OpenSubmission(receipt.Submission)
		}}, &sequenceSecrets{values: [][]byte{password}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("prepared submission without sealed pair = %v, want invalid", err)
	}
}

func TestRecoveredNameAuthorityActivatesOnlyFromStrictCurrentNamespaceWitness(t *testing.T) {
	now, network := time.Unix(1_800_001_200, 0).UTC(), [32]byte{21}
	seed := sha256.Sum256([]byte("custody-reconciliation-name-authority"))
	private := ed25519.NewKeyFromSeed(seed[:])
	public := private.Public().(ed25519.PublicKey)
	initial := testAuthorityState()
	initial.Binding.Kind, initial.Binding.Network = AuthorityName, network
	initial.Binding.IDCommitment = sha256.Sum256(public)
	initial.RootMaterial, initial.Generation, initial.Revision = append([]byte(nil), private...), 1, 1
	initial.Watermarks = []Watermark{{Domain: "name-transition", Value: 1}}
	source, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	vaultPassword, bundlePassword := []byte("correct horse battery staple"), []byte("one-time recovery bundle password")
	created, err := source.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: initial},
		&sequenceSecrets{values: [][]byte{vaultPassword, vaultPassword}})
	if err != nil {
		t.Fatalf("create source vault record: %v", err)
	}
	bundlePath := filepath.Join(t.TempDir(), "authority-recovery-bundle.json")
	if _, err := source.Execute(t.Context(), Operation{Kind: OperationExportRecoveryBundle, RecordID: created.RecordID,
		Expected: initial.Binding, Path: bundlePath}, &sequenceSecrets{values: [][]byte{vaultPassword, bundlePassword, bundlePassword}}); err != nil {
		t.Fatalf("export recovery bundle: %v", err)
	}
	recovered, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recovered.Close() })
	recoveredPassword := []byte("new recovered vault password")
	locked, err := recovered.Execute(t.Context(), Operation{Kind: OperationRestoreRecoveryBundle, Path: bundlePath,
		Expected: initial.Binding}, &sequenceSecrets{values: [][]byte{bundlePassword, recoveredPassword, recoveredPassword}})
	if err != nil {
		t.Fatalf("restore recovery bundle: %v", err)
	}
	current := record.Record{Name: "alice", Generation: 2, Revision: 2, Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(public), Target: [32]byte{1}, LeaseExpiresAt: now.Add(time.Hour).Unix(),
		GraceExpiresAt: now.Add(2 * time.Hour).Unix(), Continuity: 2}
	stale := current
	stale.Generation, stale.Revision, stale.Continuity = initial.Generation, initial.Revision, 1
	staleWitness := currentNameAuthorityWitness(t, network, stale, private)
	if _, err := recovered.Execute(t.Context(), Operation{Kind: OperationActivateRecoveredAuthority, RecordID: locked.RecordID,
		Expected: initial.Binding, Reconciliation: &staleWitness}, &sequenceSecrets{values: [][]byte{recoveredPassword}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("stale current Namespace witness = %v, want invalid", err)
	}
	witness := currentNameAuthorityWitness(t, network, current, private)
	extraLockedRecord := filepath.Join(recovered.quarantine, "record-00112233445566778899aabbccddeeff.json")
	if err := os.WriteFile(extraLockedRecord, []byte("unexpected quarantine record"), 0o600); err != nil {
		t.Fatalf("write unexpected quarantine record: %v", err)
	}
	if _, err := recovered.Execute(t.Context(), Operation{Kind: OperationActivateRecoveredAuthority, RecordID: locked.RecordID,
		Expected: initial.Binding, Reconciliation: &witness}, &sequenceSecrets{values: [][]byte{recoveredPassword}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("activation with an extra quarantine record = %v, want invalid", err)
	}
	if err := os.Remove(extraLockedRecord); err != nil {
		t.Fatalf("remove unexpected quarantine record: %v", err)
	}
	activated, err := recovered.Execute(t.Context(), Operation{Kind: OperationActivateRecoveredAuthority, RecordID: locked.RecordID,
		Expected: initial.Binding, Reconciliation: &witness}, &sequenceSecrets{values: [][]byte{recoveredPassword}})
	if err != nil {
		t.Fatalf("activate recovered Authority: %v", err)
	}
	if activated.State != RecordActive || activated.Authority.Generation != current.Generation || activated.Authority.Revision != current.Revision ||
		len(activated.Authority.Watermarks) != 1 || activated.Authority.Watermarks[0].Value != initial.Watermarks[0].Value+1 {
		t.Fatalf("activation receipt=%+v", activated)
	}
	_, err = recovered.Execute(t.Context(), Operation{Kind: OperationActivateRecoveredAuthority, RecordID: locked.RecordID,
		Expected: initial.Binding, Reconciliation: &witness}, &sequenceSecrets{values: [][]byte{recoveredPassword}})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("second activation err=%v, want invalid", err)
	}
	if _, err := recovered.Execute(t.Context(), Operation{Kind: OperationSignNamespaceTransition, RecordID: locked.RecordID,
		Expected: initial.Binding, Transition: func(authority.TransitionSigner) ([]byte, error) { return nil, nil }},
		&sequenceSecrets{values: [][]byte{recoveredPassword}}); err == nil {
		t.Fatal("authority-locked record signed after activation of its successor")
	}
}

func currentNameAuthorityWitness(t *testing.T, network [32]byte, current record.Record, private ed25519.PrivateKey) epoch.NameAuthorityReconciliation {
	t.Helper()
	keys := make([]ed25519.PrivateKey, 0, 2)
	policy := epoch.MaterializationPolicy{Network: network, Rule: "ardents-namespace-materialization-v1",
		Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}
	for _, label := range []string{"custody-reconciliation-a", "custody-reconciliation-b"} {
		seed := sha256.Sum256([]byte(label))
		key := ed25519.NewKeyFromSeed(seed[:])
		policy.Authorities[sha256.Sum256(key.Public().(ed25519.PublicKey))] = key.Public().(ed25519.PublicKey)
		keys = append(keys, key)
	}
	store, err := epoch.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	signed, err := record.SignRecord(network, current, private)
	if err != nil {
		t.Fatal(err)
	}
	installation := epoch.Epoch{Number: 1, Digest: [32]byte{1}, CutoffOffset: 1,
		TransitionRoot: [32]byte{2}, TransitionLength: 1, RejectionRoot: [32]byte{3}}
	if err := store.CommitLegacy(installation, [][]byte{signed}, func(transcript []byte) ([][32]byte, [][]byte, error) {
		type signature struct {
			id  [32]byte
			sig []byte
		}
		signatures := make([]signature, 0, len(keys))
		for _, key := range keys {
			signatures = append(signatures, signature{id: sha256.Sum256(key.Public().(ed25519.PublicKey)), sig: ed25519.Sign(key, transcript)})
		}
		sort.Slice(signatures, func(i, j int) bool { return bytes.Compare(signatures[i].id[:], signatures[j].id[:]) < 0 })
		ids, values := make([][32]byte, len(signatures)), make([][]byte, len(signatures))
		for index := range signatures {
			ids[index], values[index] = signatures[index].id, signatures[index].sig
		}
		return ids, values, nil
	}); err != nil {
		t.Fatal(err)
	}
	var authorityKey [ed25519.PublicKeySize]byte
	copy(authorityKey[:], private.Public().(ed25519.PublicKey))
	witness, err := store.CurrentNameAuthority(authorityKey)
	if err != nil {
		t.Fatal(err)
	}
	return witness
}

type transitionAdapter struct{ signer authority.ControlSigner }

func (adapter transitionAdapter) Sign(request authority.TransitionSigningRequest) ([]byte, error) {
	return adapter.signer.SignTransition(request)
}

type recordAdapter struct{ signer authority.ControlSigner }

func (adapter recordAdapter) Sign(request record.RecordSigningRequest) ([]byte, error) {
	return adapter.signer.SignRecord(request)
}

type custodyDerivedWire struct {
	Kind             string   `json:"kind"`
	SigningMode      string   `json:"signing_mode"`
	OperationDigest  [32]byte `json:"operation_digest"`
	Network          [32]byte `json:"network,omitempty"`
	Nonce            [32]byte `json:"nonce,omitempty"`
	Name             string   `json:"name"`
	Generation       uint64   `json:"generation"`
	ExpectedRevision uint64   `json:"expected_revision"`
	Authority        [32]byte `json:"authority,omitempty"`
	SuccessorAuth    [32]byte `json:"successor_authority,omitempty"`
	Target           [32]byte `json:"target,omitempty"`
	LeaseNotAfter    int64    `json:"lease_not_after"`
	PolicyID         [32]byte `json:"policy_id,omitempty"`
	AuthorityProof   []byte   `json:"authority_proof,omitempty"`
	SuccessorRecord  []byte   `json:"successor_record,omitempty"`
}

func custodyDerivedSubmission(current record.Record, now time.Time, proof, successor []byte) (authority.Submission, error) {
	value := custodyDerivedWire{Kind: "renew", SigningMode: "custody-derived-v1", Name: current.Name,
		Generation: current.Generation, ExpectedRevision: current.Revision, LeaseNotAfter: now.Add(time.Hour).UnixMilli()}
	canonical, err := json.Marshal(value)
	if err != nil {
		return authority.Submission{}, err
	}
	value.OperationDigest = sha256.Sum256(append([]byte("ardents-name-control-operation-v1\x00"), canonical...))
	intentRaw, err := json.Marshal(value)
	if err != nil {
		return authority.Submission{}, err
	}
	if _, err := authority.OpenIntent(intentRaw); err != nil {
		return authority.Submission{}, err
	}
	value.AuthorityProof, value.SuccessorRecord = proof, successor
	canonical, err = json.Marshal(value)
	if err != nil {
		return authority.Submission{}, err
	}
	return authority.OpenSubmission(canonical)
}

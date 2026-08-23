package namespace

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestPendingJournalSurvivesRestartAndRejectsTamper(t *testing.T) {
	now, network := time.Unix(1_800_000_000, 0).UTC(), [32]byte{9}
	store, policy := pendingTestStore(t, network)
	key := deterministicControlKey("pending-authority")
	current := controlTestRecord("alice", key, now)
	op := Op{Kind: "renew", Name: current.Name, Authority: current.Authority,
		ExpectedGeneration: current.Generation, ExpectedRevision: current.Revision, LeaseDuration: time.Hour}
	proof, err := SignTransition(network, current, op, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := signedControlOperation(t, controlOperation{Kind: "renew", Name: current.Name,
		Generation: current.Generation, ExpectedRevision: current.Revision,
		LeaseNotAfter: now.Add(time.Hour).UnixMilli(), AuthorityProof: proof})
	successor, err := SignRecord(network, current, key)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := store.appendPending(raw, successor, now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	root := store.root.path
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	entries, err := reopened.pending()
	if err != nil || len(entries) != 1 || entries[0].sequence != 1 || entries[0].decisionAt != now.UnixMilli() ||
		string(entries[0].submission) != string(raw) || string(entries[0].successor) != string(successor) {
		t.Fatalf("entries=%+v err=%v", entries, err)
	}
	path := filepath.Join(root, "distribution", "generations", entry.name, "entry.bin")
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	persisted[len(persisted)-1] ^= 1
	if err := os.WriteFile(path, persisted, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.pending(); err == nil {
		t.Fatal("tampered pending entry was accepted")
	}
}

func pendingTestStore(t *testing.T, network [32]byte) (*Store, MaterializationPolicy) {
	t.Helper()
	policy, _ := pendingTestPolicy(network)
	store, err := Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	return store, policy
}

func pendingTestPolicy(network [32]byte) (MaterializationPolicy, []ed25519.PrivateKey) {
	policy := MaterializationPolicy{Network: network, Rule: materializationRule,
		Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}
	keys := make([]ed25519.PrivateKey, 0, 2)
	for _, label := range []string{"pending-attester-a", "pending-attester-b"} {
		seed := sha256.Sum256([]byte(label))
		private := ed25519.NewKeyFromSeed(seed[:])
		public := private.Public().(ed25519.PublicKey)
		policy.Authorities[sha256.Sum256(public)] = public
		keys = append(keys, private)
	}
	return policy, keys
}

func TestPendingRejectsUnsignedOrSubstitutedSuccessor(t *testing.T) {
	now, network := time.Unix(1_800_000_100, 0).UTC(), [32]byte{8}
	store, _ := pendingTestStore(t, network)
	defer store.Close()
	key := deterministicControlKey("pending-substitution")
	record := controlTestRecord("alice", key, now)
	raw, _ := signedControlOperation(t, controlOperation{Kind: "renew", Name: record.Name,
		Generation: 1, ExpectedRevision: 1, LeaseNotAfter: now.Add(time.Hour).UnixMilli(), AuthorityProof: []byte{1}})
	if _, err := store.appendPending(raw, []byte{1}, now.UnixMilli()); err == nil {
		t.Fatal("unsigned successor was persisted")
	}
}

func TestDurableControlRestoresExactSignedPendingSuccessor(t *testing.T) {
	now, network := time.Unix(1_800_000_200, 0).UTC(), [32]byte{7}
	policy, attesters := pendingTestPolicy(network)
	root := t.TempDir()
	store, err := Open(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	key := deterministicControlKey("durable-control")
	current := controlTestRecord("alice", key, now)
	signedCurrent, err := SignRecord(network, current, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(Epoch{Number: 1, Digest: [32]byte{1}, CutoffOffset: 1,
		TransitionRoot: [32]byte{2}, TransitionLength: 1, RejectionRoot: [32]byte{3}},
		[][]byte{signedCurrent}, pendingTestAttester(attesters)); err != nil {
		t.Fatal(err)
	}
	gate, err := NewAdmission([32]byte{2}, network, 1, [32]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	leasePolicy := Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour}
	control, err := OpenControl(store, gate, ClaimOrder{}, func() time.Time { return now }, leasePolicy)
	if err != nil {
		t.Fatal(err)
	}
	updated := durableRenew(t, network, current, key, now, leasePolicy)
	substituted := updated
	substituted.Continuity++
	forgedSubmission, forgedAdmission := durableSubmission(t, gate, network, now, substituted, current, key, 17)
	if class := control.Submit(forgedSubmission, forgedAdmission); class != "denied" {
		t.Fatalf("substituted signed successor=%q", class)
	}
	if entries, pendingErr := store.pending(); pendingErr != nil || len(entries) != 0 {
		t.Fatalf("substituted successor changed pending journal: %+v / %v", entries, pendingErr)
	}
	if class := durableSubmit(t, control, gate, network, now, updated, current, key, 5); class != "submitted" {
		t.Fatalf("first durable submission=%q", class)
	}
	signedUpdated, err := SignRecord(network, updated, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(Epoch{Number: 2, Digest: [32]byte{4}, CutoffOffset: 2,
		TransitionRoot: [32]byte{5}, TransitionLength: 1, RejectionRoot: [32]byte{6}},
		[][]byte{signedUpdated}, pendingTestAttester(attesters)); err != nil {
		t.Fatalf("pending selected materialization: %v", err)
	}
	forged, err := SignRecord(network, current, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(Epoch{Number: 3, Digest: [32]byte{7}, CutoffOffset: 3,
		TransitionRoot: [32]byte{8}, TransitionLength: 1, RejectionRoot: [32]byte{9}},
		[][]byte{forged}, pendingTestAttester(attesters)); err == nil {
		t.Fatal("arbitrary current corpus bypassed durable pending state")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopenedGate, err := NewAdmission([32]byte{2}, network, 1, [32]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := OpenControl(reopened, reopenedGate, ClaimOrder{}, func() time.Time { return now }, leasePolicy)
	if err != nil {
		t.Fatalf("pending successor was not restored: %v", err)
	}
	next := durableRenew(t, network, updated, key, now, leasePolicy)
	if class := durableSubmit(t, restored, reopenedGate, network, now, next, updated, key, 6); class != "submitted" {
		t.Fatalf("restored durable submission=%q", class)
	}
}

func TestDurableControlRejectsLateRootClaim(t *testing.T) {
	now, network := time.Unix(1_800_000_300, 0).UTC(), [32]byte{6}
	policy, attesters := pendingTestPolicy(network)
	store, err := Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	key := deterministicControlKey("durable-root-claim")
	current := controlTestRecord("alice", key, now)
	signedCurrent, err := SignRecord(network, current, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(Epoch{Number: 1, Digest: [32]byte{1}, CutoffOffset: 1,
		TransitionRoot: [32]byte{2}, TransitionLength: 1, RejectionRoot: [32]byte{3}},
		[][]byte{signedCurrent}, pendingTestAttester(attesters)); err != nil {
		t.Fatal(err)
	}
	gate, err := NewAdmission([32]byte{2}, network, 1, [32]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	control, err := OpenControl(store, gate, ClaimOrder{}, func() time.Time { return now }, Policy{})
	if err != nil {
		t.Fatal(err)
	}
	raw, digest := signedControlOperation(t, controlOperation{Kind: "claim", Name: current.Name, Generation: 1,
		Authority: authorityBytes(current.Authority), LeaseNotAfter: now.Add(time.Hour).UnixMilli(),
		OrderingProof: []byte("{}"), SuccessorRecord: signedCurrent})
	submission, err := OpenSubmission(raw)
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := gate.Issue(now.UnixMilli(), "root-claim", digest, [32]byte{1},
		now.Add(15*time.Second).UnixMilli(), [16]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	proof, _ := challenge.Solve()
	if class := control.Submit(submission, proof); class != "denied" {
		t.Fatalf("late root claim=%q", class)
	}
	if entries, pendingErr := store.pending(); pendingErr != nil || len(entries) != 0 {
		t.Fatalf("late root claim changed pending journal: %+v / %v", entries, pendingErr)
	}
}

func durableRenew(t *testing.T, network [32]byte, current Record, key ed25519.PrivateKey,
	now time.Time, policy Policy,
) Record {
	t.Helper()
	updated, err := Apply(&current, now.Unix(), Op{Kind: "renew", Name: current.Name, Authority: current.Authority,
		ExpectedGeneration: current.Generation, ExpectedRevision: current.Revision, LeaseDuration: policy.DefaultLeaseDuration}, policy)
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

func durableSubmit(t *testing.T, control *control, gate *Admission, network [32]byte, now time.Time,
	updated, current Record, key ed25519.PrivateKey, salt byte,
) string {
	t.Helper()
	submission, admission := durableSubmission(t, gate, network, now, updated, current, key, salt)
	return control.Submit(submission, admission)
}

func durableSubmission(t *testing.T, gate *Admission, network [32]byte, now time.Time,
	updated, current Record, key ed25519.PrivateKey, salt byte,
) (Submission, Proof) {
	t.Helper()
	op := Op{Kind: "renew", Name: current.Name, Authority: current.Authority,
		ExpectedGeneration: current.Generation, ExpectedRevision: current.Revision, LeaseDuration: time.Hour}
	proof, err := SignTransition(network, current, op, key)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := SignRecord(network, updated, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, digest := signedControlOperation(t, controlOperation{Kind: "renew", Name: current.Name,
		Generation: current.Generation, ExpectedRevision: current.Revision, LeaseNotAfter: now.Add(time.Hour).UnixMilli(),
		AuthorityProof: proof, SuccessorRecord: signed})
	submission, err := OpenSubmission(raw)
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := gate.Issue(now.UnixMilli(), "renewal-update", digest, [32]byte{salt},
		now.Add(15*time.Second).UnixMilli(), [16]byte{salt})
	if err != nil {
		t.Fatal(err)
	}
	admission, _ := challenge.Solve()
	return submission, admission
}

func pendingTestAttester(keys []ed25519.PrivateKey) func([]byte) ([][32]byte, [][]byte, error) {
	return func(transcript []byte) ([][32]byte, [][]byte, error) {
		type signature struct {
			id  [32]byte
			raw []byte
		}
		values := make([]signature, len(keys))
		for index, key := range keys {
			public := key.Public().(ed25519.PublicKey)
			values[index] = signature{id: sha256.Sum256(public), raw: ed25519.Sign(key, transcript)}
		}
		sort.Slice(values, func(first, second int) bool { return bytes.Compare(values[first].id[:], values[second].id[:]) < 0 })
		ids, signatures := make([][32]byte, len(values)), make([][]byte, len(values))
		for index, value := range values {
			ids[index], signatures[index] = value.id, value.raw
		}
		return ids, signatures, nil
	}
}

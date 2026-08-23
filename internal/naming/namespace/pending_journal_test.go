package namespace

import (
	"crypto/ed25519"
	"crypto/sha256"
	"os"
	"path/filepath"
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
	policy := MaterializationPolicy{Network: network, Rule: materializationRule,
		Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}
	for _, label := range []string{"pending-attester-a", "pending-attester-b"} {
		seed := sha256.Sum256([]byte(label))
		private := ed25519.NewKeyFromSeed(seed[:])
		public := private.Public().(ed25519.PublicKey)
		policy.Authorities[sha256.Sum256(public)] = public
	}
	store, err := Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	return store, policy
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

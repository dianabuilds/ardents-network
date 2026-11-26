package release

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

type signedHistory struct {
	input Inputs
	root  *metadata.Metadata[metadata.RootType]
	keys  []signature.Signer
}

func historyKeys(t *testing.T) (map[string]*metadata.Key, []string, []signature.Signer) {
	t.Helper()
	keys := make(map[string]*metadata.Key)
	var ids []string
	var signers []signature.Signer
	for range 5 {
		pub, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signer, err := signature.LoadSigner(private, crypto.Hash(0))
		if err != nil {
			t.Fatal(err)
		}
		key, err := metadata.KeyFromPublicKey(pub)
		if err != nil {
			t.Fatal(err)
		}
		id, err := key.ID()
		if err != nil {
			t.Fatal(err)
		}
		keys[id] = key
		ids = append(ids, id)
		signers = append(signers, signer)
	}
	return keys, ids, signers
}
func historySign(t *testing.T, value interface {
	Sign(signature.Signer) (*metadata.Signature, error)
}, keys []signature.Signer) {
	t.Helper()
	for _, k := range keys {
		if _, err := value.Sign(k); err != nil {
			t.Fatal(err)
		}
	}
}
func historyBytes(t *testing.T, value interface{ ToBytes(bool) ([]byte, error) }) []byte {
	t.Helper()
	b, err := value.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The independent frozen target facts and artifact remain the oracle. Only
// signatures and dependent hashes are regenerated with ephemeral fixture keys.
func freshSignedHistory(t *testing.T) signedHistory {
	t.Helper()
	in := publicVector(t)
	root, err := metadata.Root().FromBytes(in.RootBytes)
	if err != nil {
		t.Fatal(err)
	}
	keys, ids, signers := historyKeys(t)
	root.Signed.Keys = keys
	for _, name := range []string{"root", "timestamp", "snapshot", "targets"} {
		root.Signed.Roles[name] = &metadata.Role{KeyIDs: append([]string(nil), ids...), Threshold: 3}
	}
	root.Signatures = nil
	historySign(t, root, signers[:3])
	in.RootBytes = historyBytes(t, root)
	history := signedHistory{in, root, signers}
	history.resignOnline(t, signers, 1)
	return history
}
func (h *signedHistory) resignOnline(t *testing.T, keys []signature.Signer, version int64) {
	h.resignOnlineThreshold(t, keys, version, 3)
}

func (h *signedHistory) resignOnlineThreshold(t *testing.T, keys []signature.Signer, version int64, targetSignatures int) {
	t.Helper()
	targets, err := metadata.Targets().FromBytes(h.input.Files[metadataBaseURL+"1.targets.json"])
	if err != nil {
		t.Fatal(err)
	}
	targets.Signed.Version = version
	targets.Signatures = nil
	historySign(t, targets, keys[:targetSignatures])
	targetBytes := historyBytes(t, targets)
	targetDigest := sha256.Sum256(targetBytes)
	snapshot, err := metadata.Snapshot().FromBytes(h.input.Files[metadataBaseURL+"1.snapshot.json"])
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Signed.Version = version
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: version, Length: int64(len(targetBytes)), Hashes: metadata.Hashes{"sha256": targetDigest[:]}}
	snapshot.Signatures = nil
	historySign(t, snapshot, keys[:3])
	snapshotBytes := historyBytes(t, snapshot)
	snapshotDigest := sha256.Sum256(snapshotBytes)
	timestamp, err := metadata.Timestamp().FromBytes(h.input.Files[metadataBaseURL+"timestamp.json"])
	if err != nil {
		t.Fatal(err)
	}
	timestamp.Signed.Version = version
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: version, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotDigest[:]}}
	timestamp.Signatures = nil
	historySign(t, timestamp, keys[:3])
	delete(h.input.Files, metadataBaseURL+"1.targets.json")
	h.input.Files[metadataBaseURL+fmt.Sprintf("%d.targets.json", version)] = targetBytes
	delete(h.input.Files, metadataBaseURL+"1.snapshot.json")
	h.input.Files[metadataBaseURL+fmt.Sprintf("%d.snapshot.json", version)] = snapshotBytes
	h.input.Files[metadataBaseURL+"timestamp.json"] = historyBytes(t, timestamp)
}

func TestSignedEmergencyThresholdAndExactExpiry(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		count int
		delta time.Duration
		want  Outcome
	}{
		{"ordinary-threshold-refused", 3, -time.Second, OutcomeReleaseInvalid},
		{"four-signatures-before-expiry", 4, -time.Second, OutcomeReleaseAccepted},
		{"exact-expiry-refused", 4, 0, OutcomeReleaseUnavailable},
		{"after-expiry-refused", 4, time.Second, OutcomeReleaseUnavailable},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			h := freshSignedHistory(t)
			expiry := h.input.Local.RefTime.Add(time.Hour)
			targets, err := metadata.Targets().FromBytes(h.input.Files[metadataBaseURL+"1.targets.json"])
			if err != nil {
				t.Fatal(err)
			}
			var custom map[string]any
			target := targets.Signed.Targets[h.input.TargetPath]
			if err = json.Unmarshal(*target.Custom, &custom); err != nil {
				t.Fatal(err)
			}
			custom["protocol_phase"] = "required"
			custom["emergency_reason"] = "credible-exploitable-flaw"
			custom["emergency_expiry"] = expiry.Format(time.RFC3339)
			raw, err := json.Marshal(custom)
			if err != nil {
				t.Fatal(err)
			}
			message := json.RawMessage(raw)
			target.Custom = &message
			h.input.Files[metadataBaseURL+"1.targets.json"] = historyBytes(t, targets)
			h.resignOnlineThreshold(t, h.keys, 1, scenario.count)
			h.input.Local.RefTime = expiry.Add(scenario.delta)
			v, err := Open(filepath.Join(t.TempDir(), "history"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := v.Close(); err != nil {
					t.Error(err)
				}
			})
			d := v.Evaluate(context.Background(), h.input)
			_, authorized := d.Authorization()
			if d.Outcome != scenario.want || authorized != (scenario.want == OutcomeReleaseAccepted) {
				t.Fatalf("signed emergency decision: %s %v", d.Outcome, d.Err())
			}
			floors, err := v.CurrentFloors(context.Background())
			if err != nil || (floors.TargetsVersion != 0) != authorized {
				t.Fatal("refusal committed emergency metadata floors", err)
			}
		})
	}
}

func TestAuthenticatedRollbackAndSameVersionConflictRetainFloors(t *testing.T) {
	for _, name := range []string{"rollback", "same-version-conflict"} {
		t.Run(name, func(t *testing.T) {
			h := freshSignedHistory(t)
			old := freezeInputs(h.input)
			h.resignOnline(t, h.keys, 2)
			root := filepath.Join(t.TempDir(), "history")
			v, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = v.Close() })
			if d := v.Evaluate(context.Background(), h.input); d.Outcome != OutcomeReleaseAccepted {
				t.Fatal(d.Outcome, d.Err())
			}
			before, err := v.CurrentFloors(context.Background())
			if err != nil || before.TargetsVersion != 2 {
				t.Fatal("baseline did not advance", err)
			}
			candidate := old
			if name == "same-version-conflict" {
				// Independent re-signing at the retained version changes exact
				// signature bytes while preserving authenticated artifact facts.
				h.input = old
				h.resignOnlineThreshold(t, h.keys, 2, 4)
				candidate = h.input
			}
			d := v.Evaluate(context.Background(), candidate)
			if _, ok := d.Authorization(); ok {
				t.Fatal("rollback/conflict authorized")
			}
			after, err := v.CurrentFloors(context.Background())
			if err != nil || !floorSetEqual(before, after) {
				t.Fatal("refusal changed retained floors", err)
			}
			if err = v.Close(); err != nil {
				t.Fatal(err)
			}
			v, err = Open(root)
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := v.CurrentFloors(context.Background())
			if err != nil || !floorSetEqual(before, reopened) {
				t.Fatal("reopen lost conflict floors", err)
			}
		})
	}
}

func TestRootRoleCannotCountDuplicateKeysAsFive(t *testing.T) {
	h := freshSignedHistory(t)
	for _, role := range h.root.Signed.Roles {
		ids := role.KeyIDs
		role.KeyIDs = []string{ids[0], ids[1], ids[2], ids[0], ids[1]}
	}
	h.root.Signatures = nil
	historySign(t, h.root, h.keys[:3])
	h.input.RootBytes = historyBytes(t, h.root)
	v, err := Open(filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	d := v.Evaluate(context.Background(), h.input)
	if _, ok := d.Authorization(); ok {
		t.Fatal("three distinct keys masqueraded as five")
	}
	floors, err := v.CurrentFloors(context.Background())
	if err != nil || floors.RootVersion != 0 {
		t.Fatal("invalid key set established Root", err)
	}
}

func TestRootRoleNeedsFiveDistinctPublicKeys(t *testing.T) {
	h := freshSignedHistory(t)
	ids := h.root.Signed.Roles["root"].KeyIDs
	aliased := append([]string(nil), ids[:3]...)
	// Canonical key IDs include auxiliary fields. They cannot be counted as
	// distinct signing keys when the actual normalized public material repeats.
	for i := range 2 {
		original := h.root.Signed.Keys[ids[i]]
		key := &metadata.Key{Type: original.Type, Scheme: original.Scheme, Value: original.Value, UnrecognizedFields: map[string]any{"alias_probe": i}}
		id, err := key.ID()
		if err != nil {
			t.Fatal(err)
		}
		h.root.Signed.Keys[id] = key
		aliased = append(aliased, id)
	}
	for _, role := range h.root.Signed.Roles {
		role.KeyIDs = append([]string(nil), aliased...)
	}
	h.root.Signatures = nil
	historySign(t, h.root, h.keys[:3])
	h.input.RootBytes = historyBytes(t, h.root)
	v, err := Open(filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	d := v.Evaluate(context.Background(), h.input)
	if _, ok := d.Authorization(); ok {
		t.Fatal("three public keys disguised as five identities authorized")
	}
	floors, err := v.CurrentFloors(context.Background())
	if err != nil || floors.RootVersion != 0 {
		t.Fatal("invalid key material set established Root", err)
	}
}
func (h *signedHistory) rotate(t *testing.T, version int64, oldCount int) {
	t.Helper()
	next, err := metadata.Root().FromBytes(historyBytes(t, h.root))
	if err != nil {
		t.Fatal(err)
	}
	keys, ids, newSigners := historyKeys(t)
	next.Signed.Keys = keys
	next.Signed.Version = version
	for _, name := range []string{"root", "timestamp", "snapshot", "targets"} {
		next.Signed.Roles[name] = &metadata.Role{KeyIDs: append([]string(nil), ids...), Threshold: 3}
	}
	next.Signatures = nil
	historySign(t, next, h.keys[:oldCount])
	historySign(t, next, newSigners[:3])
	h.input.Files[metadataBaseURL+fmt.Sprintf("%d.root.json", version)] = historyBytes(t, next)
	h.resignOnline(t, newSigners, 2)
}

func TestConsecutiveRootRotationRequiresBothThresholds(t *testing.T) {
	for _, s := range []struct {
		name          string
		version       int64
		oldSignatures int
		accept        bool
	}{{"both-thresholds", 2, 3, true}, {"new-only", 2, 0, false}, {"insufficient-old", 2, 2, false}, {"gap", 3, 3, false}} {
		t.Run(s.name, func(t *testing.T) {
			h := freshSignedHistory(t)
			root := filepath.Join(t.TempDir(), "history")
			v, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = v.Close() })
			if d := v.Evaluate(context.Background(), h.input); d.Outcome != OutcomeReleaseAccepted {
				t.Fatalf("initial genuine metadata refused %s %v", d.Outcome, d.Err())
			}
			h.rotate(t, s.version, s.oldSignatures)
			d := v.Evaluate(context.Background(), h.input)
			_, ok := d.Authorization()
			if ok != s.accept {
				t.Fatalf("rotation authorization=%v outcome=%s cause=%v", ok, d.Outcome, d.Err())
			}
			floors, err := v.CurrentFloors(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			want := int64(1)
			if s.accept {
				want = 2
			}
			if floors.RootVersion != want {
				t.Fatalf("Root floor %d want%d", floors.RootVersion, want)
			}
			if err := v.Close(); err != nil {
				t.Fatal(err)
			}
			v, err = Open(root)
			if err != nil {
				t.Fatal(err)
			}
			retained, err := v.CurrentFloors(context.Background())
			if err != nil || !floorSetEqual(retained, floors) {
				t.Fatal("rotation floors not retained on reopen")
			}
			if s.accept {
				h.input.RootBytes = append([]byte(nil), h.input.Files[metadataBaseURL+"2.root.json"]...)
				delete(h.input.Files, metadataBaseURL+"2.root.json")
				if d := v.Evaluate(context.Background(), h.input); d.Outcome != OutcomeNoUpdate {
					t.Fatalf("current trusted Root retry: %s %v", d.Outcome, d.Err())
				}
			}
		})
	}
}

package release

import (
	"testing"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestEvaluateB11RejectsOneSidedRootThreshold(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	newKeys := makeFiveEd25519Keys(t)
	next := newTestRoot(t, newKeys, 2, testRefTime.Add(24*time.Hour))
	signSyntheticMetadataCount(t, next, newKeys, ordinaryThreshold)
	encoded, err := next.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	repo.files["https://release.invalid/metadata/2.root.json"] = encoded
	decision := evaluateWithStore(t, repo, newMemoryStoreForTest(), testRefTime)
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

func TestEvaluateB11RejectsExpiredSuccessorRoot(t *testing.T) {
	t.Parallel()
	repo := withConsecutiveRoots(t, newSyntheticRepository(t, syntheticOptions{}), 1)
	name := "https://release.invalid/metadata/2.root.json"
	root, err := metadata.Root().FromBytes(repo.files[name])
	if err != nil {
		t.Fatal(err)
	}
	root.Signed.Expires = testRefTime.Add(-time.Hour)
	root.Signatures = nil
	signSyntheticMetadataCount(t, root, repo.keys, ordinaryThreshold)
	repo.files[name], err = root.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	decision := evaluateWithStore(t, repo, newMemoryStoreForTest(), testRefTime)
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

func TestEvaluateB11RejectsUnknownRootPolicy(t *testing.T) {
	t.Parallel()
	repo := withConsecutiveRoots(t, newSyntheticRepository(t, syntheticOptions{}), 1)
	name := "https://release.invalid/metadata/2.root.json"
	root, err := metadata.Root().FromBytes(repo.files[name])
	if err != nil {
		t.Fatal(err)
	}
	root.Signed.UnrecognizedFields["future_root_authority"] = true
	root.Signatures = nil
	signSyntheticMetadataCount(t, root, repo.keys, ordinaryThreshold)
	repo.files[name], err = root.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	decision := evaluateWithStore(t, repo, newMemoryStoreForTest(), testRefTime)
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

func newTestRoot(t *testing.T, keys []syntheticKey, version int64, expires time.Time) *metadata.Metadata[metadata.RootType] {
	t.Helper()
	root := metadata.Root(expires)
	root.Signed.Version = version
	root.Signed.UnrecognizedFields = map[string]any{
		rootSchemaField: targetSchemaVersion, rootProfileField: targetProfile,
		rootEnvField: "h3-test", rootNetworkField: "ardents-h3-test-1",
	}
	keyIDs := make([]string, 0, len(keys))
	for _, key := range keys {
		publicKey, err := metadata.KeyFromPublicKey(key.public)
		if err != nil {
			t.Fatal(err)
		}
		keyID, err := publicKey.ID()
		if err != nil {
			t.Fatal(err)
		}
		root.Signed.Keys[keyID] = publicKey
		keyIDs = append(keyIDs, keyID)
	}
	for _, role := range metadata.TOP_LEVEL_ROLE_NAMES {
		root.Signed.Roles[role] = &metadata.Role{KeyIDs: append([]string(nil), keyIDs...), Threshold: ordinaryThreshold}
	}
	return root
}

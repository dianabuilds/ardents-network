package releasedecision

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// syntheticKey is one ephemeral Ed25519 key plus the signer used to
// produce valid go-tuf signatures. Tests construct keys, sign
// metadata, and immediately discard the private material.
type syntheticKey struct {
	id     string
	signer signature.Signer
	public ed25519.PublicKey
}

// syntheticRepository is the test-side container for one complete
// valid offline-import set. The bytes are the same ones the
// releasedecision package would receive from the offline-import
// caller.
type syntheticRepository struct {
	rootBytes  []byte
	files      map[string][]byte
	targetPath string
	artifact   []byte
	root       *metadata.Metadata[metadata.RootType]
	timestamp  *metadata.Metadata[metadata.TimestampType]
	snapshot   *metadata.Metadata[metadata.SnapshotType]
	targets    *metadata.Metadata[metadata.TargetsType]
	keys       []syntheticKey
}

// newSyntheticRepository produces a complete, valid Stage 7
// release-decision fixture. The caller customises the target
// identity, the environment, the artifact size, and the root
// version. The supplied version is the active root version.
func newSyntheticRepository(t *testing.T, opts syntheticOptions) syntheticRepository {
	t.Helper()
	keys := makeFiveEd25519Keys(t)
	rootVersion := opts.rootVersion
	if rootVersion == 0 {
		rootVersion = 1
	}
	expires := opts.expires
	if expires.IsZero() {
		expires = time.Now().UTC().Add(24 * time.Hour)
	}
	root := metadata.Root(expires)
	for _, key := range keys {
		publicKey, err := metadata.KeyFromPublicKey(key.public)
		if err != nil {
			t.Fatal(err)
		}
		id, err := publicKey.ID()
		if err != nil {
			t.Fatal(err)
		}
		root.Signed.Keys[id] = publicKey
		_ = key.id
	}
	roleKeyIDs := make([]string, 0, totalTopLevelKeys)
	for index, key := range keys {
		publicKey, err := metadata.KeyFromPublicKey(key.public)
		if err != nil {
			t.Fatal(err)
		}
		id, err := publicKey.ID()
		if err != nil {
			t.Fatal(err)
		}
		_ = key
		_ = index
		roleKeyIDs = append(roleKeyIDs, id)
	}
	for _, role := range metadata.TOP_LEVEL_ROLE_NAMES {
		root.Signed.Roles[role] = &metadata.Role{KeyIDs: append([]string(nil), roleKeyIDs...), Threshold: ordinaryThreshold}
	}
	signSyntheticMetadata(t, root, keys)
	rootBytes, err := root.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	// Root version override: tests may want to pretend the durable
	// floor has a different version, so this helper rebuilds a
	// matching root with the requested version when asked.
	if root.Signed.Version != rootVersion {
		root.Signed.Version = rootVersion
		// Re-sign because the version is part of the canonical bytes.
		root.Signatures = nil
		signSyntheticMetadata(t, root, keys)
		rootBytes, err = root.ToBytes(false)
		if err != nil {
			t.Fatal(err)
		}
	}
	artifactLength := opts.artifactLength
	if artifactLength == 0 {
		artifactLength = 4096
	}
	artifact := make([]byte, artifactLength)
	if _, err := rand.Read(artifact); err != nil {
		t.Fatal(err)
	}
	targetPath := opts.targetPath
	if targetPath == "" {
		targetPath = "ardents/windows-amd64/application"
	}
	artifactHash := sha256.Sum256(artifact)
	selected := &metadata.TargetFiles{Length: int64(len(artifact)), Hashes: metadata.Hashes{"sha256": artifactHash[:]}, Path: targetPath}
	custom := buildCustomJSON(t, opts)
	targets := metadata.Targets(expires)
	targets.Signed.Targets[targetPath] = &metadata.TargetFiles{
		Length: selected.Length,
		Hashes: selected.Hashes,
		Path:   targetPath,
		Custom: &custom,
	}
	signSyntheticMetadata(t, targets, keys)
	targetsBytes, err := targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	targetsHash := sha256.Sum256(targetsBytes)
	snapshot := metadata.Snapshot(expires)
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(targetsBytes)), Hashes: metadata.Hashes{"sha256": targetsHash[:]}}
	signSyntheticMetadata(t, snapshot, keys)
	snapshotBytes, err := snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	snapshotHash := sha256.Sum256(snapshotBytes)
	timestamp := metadata.Timestamp(expires)
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotHash[:]}}
	signSyntheticMetadata(t, timestamp, keys)
	timestampBytes, err := timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"https://release.invalid/metadata/timestamp.json":  timestampBytes,
		"https://release.invalid/metadata/1.snapshot.json": snapshotBytes,
		"https://release.invalid/metadata/1.targets.json":  targetsBytes,
	}
	return syntheticRepository{
		rootBytes:  rootBytes,
		files:      files,
		targetPath: targetPath,
		artifact:   artifact,
		root:       root,
		timestamp:  timestamp,
		snapshot:   snapshot,
		targets:    targets,
		keys:       keys,
	}
}

// buildCustomJSON renders the target custom identity block. The
// fields are documented in the lifecycle specification; the package
// rejects any unrecognized critical field name.
func buildCustomJSON(t *testing.T, opts syntheticOptions) json.RawMessage {
	t.Helper()
	if opts.platform == "" {
		opts.platform = "windows-amd64"
	}
	if opts.architecture == "" {
		opts.architecture = "amd64"
	}
	if opts.environment == "" {
		opts.environment = "h3-test"
	}
	if opts.network == "" {
		opts.network = "ardents-h3-test-1"
	}
	if opts.sourceRevision == "" {
		opts.sourceRevision = "rev-0001"
	}
	if opts.buildIdentity == "" {
		opts.buildIdentity = "build-0001"
	}
	if opts.dependencyIdentity == "" {
		opts.dependencyIdentity = "deps-0001"
	}
	if opts.sbomIdentity == "" {
		opts.sbomIdentity = "sbom-0001"
	}
	if opts.attestationPolicy == "" {
		opts.attestationPolicy = "two-builder"
	}
	if opts.qualification == "" {
		opts.qualification = "development"
	}
	if opts.protocolPhase == "" {
		opts.protocolPhase = "overlap-supported"
	}
	if opts.buildSafetyNoNewWorkAfter.IsZero() {
		opts.buildSafetyNoNewWorkAfter = time.Now().UTC().Add(30 * 24 * time.Hour)
	}
	if opts.buildSafetyTerminateAfter.IsZero() {
		opts.buildSafetyTerminateAfter = time.Now().UTC().Add(180 * 24 * time.Hour)
	}
	encoded, err := json.Marshal(struct {
		Platform              string    `json:"platform"`
		Architecture          string    `json:"architecture"`
		Environment           string    `json:"environment"`
		Network               string    `json:"network"`
		SourceRevision        string    `json:"source_revision"`
		BuildIdentity         string    `json:"build_identity"`
		DependencyIdentity    string    `json:"dependency_identity"`
		SBOMIdentity          string    `json:"sbom_identity"`
		AttestationPolicy     string    `json:"attestation_policy"`
		Qualification         string    `json:"qualification"`
		BuildSafetyNoNewAfter time.Time `json:"build_safety_no_new_work_after"`
		BuildSafetyTermAfter  time.Time `json:"build_safety_terminate_after"`
		ProtocolPhase         string    `json:"protocol_phase"`
	}{
		Platform:              opts.platform,
		Architecture:          opts.architecture,
		Environment:           opts.environment,
		Network:               opts.network,
		SourceRevision:        opts.sourceRevision,
		BuildIdentity:         opts.buildIdentity,
		DependencyIdentity:    opts.dependencyIdentity,
		SBOMIdentity:          opts.sbomIdentity,
		AttestationPolicy:     opts.attestationPolicy,
		Qualification:         opts.qualification,
		BuildSafetyNoNewAfter: opts.buildSafetyNoNewWorkAfter,
		BuildSafetyTermAfter:  opts.buildSafetyTerminateAfter,
		ProtocolPhase:         opts.protocolPhase,
	})
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

// Key generation and signing helpers are in testfixtures_keys.go.
// memoryStore is an in-memory Store for tests that do not need to
// exercise the file-based atomic publication. The committed floors
// are kept in a struct; restart tampers are simulated by mutating
// the in-memory copy.
type memoryStore struct {
	closed bool
	floors FloorSet
}

// ReadFloors returns the in-memory committed floors.
func (store *memoryStore) ReadFloors() (FloorSet, error) {
	if store.closed {
		return FloorSet{}, fmt.Errorf("releasedecision: memory store is closed")
	}
	return store.floors, nil
}

// CommitFloors accepts the supplied floors after the watermarking
// invariant passes.
func (store *memoryStore) CommitFloors(floors FloorSet) error {
	if store.closed {
		return fmt.Errorf("releasedecision: memory store is closed")
	}
	if err := validateFloorSet(floors); err != nil {
		return err
	}
	if err := assertFloorAdvance(store.floors, floors); err != nil {
		return err
	}
	store.floors = floors
	return nil
}

// Close marks the in-memory store as closed.
func (store *memoryStore) Close() error {
	store.closed = true
	return nil
}

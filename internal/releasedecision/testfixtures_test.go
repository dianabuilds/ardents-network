package releasedecision

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
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
		expires = testRefTime.Add(365 * 24 * time.Hour)
	}
	root := metadata.Root(expires)
	root.Signed.UnrecognizedFields = map[string]any{
		rootSchemaField: targetSchemaVersion, rootProfileField: targetProfile,
		rootEnvField:     defaultString(opts.rootEnvironment, "h3-test"),
		rootNetworkField: defaultString(opts.rootNetwork, "ardents-h3-test-1"),
	}
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
	for index := range artifact {
		artifact[index] = byte((index*31 + 17) % 251)
	}
	targetPath := opts.targetPath
	if targetPath == "" {
		targetPath = "ardents/windows-amd64/application"
	}
	artifactHash := sha256.Sum256(artifact)
	selected := &metadata.TargetFiles{Length: int64(len(artifact)), Hashes: metadata.Hashes{"sha256": artifactHash[:]}, Path: targetPath}
	custom := buildCustomJSON(t, opts, artifactHash[:])
	targets := metadata.Targets(expires)
	targets.Signed.Targets[targetPath] = &metadata.TargetFiles{
		Length: selected.Length,
		Hashes: selected.Hashes,
		Path:   targetPath,
		Custom: &custom,
	}
	targetSignatures := opts.targetsSignatureCount
	if targetSignatures == 0 {
		targetSignatures = ordinaryThreshold
		if opts.emergencyReason != "" {
			targetSignatures = emergencyThreshold
		}
	}
	signSyntheticMetadataCount(t, targets, keys, targetSignatures)
	targetsBytes, err := targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	targetsHash := sha256.Sum256(targetsBytes)
	snapshot := metadata.Snapshot(expires)
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(targetsBytes)), Hashes: metadata.Hashes{"sha256": targetsHash[:]}}
	signSyntheticMetadataCount(t, snapshot, keys, ordinaryThreshold)
	snapshotBytes, err := snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	snapshotHash := sha256.Sum256(snapshotBytes)
	timestamp := metadata.Timestamp(expires)
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotHash[:]}}
	signSyntheticMetadataCount(t, timestamp, keys, ordinaryThreshold)
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

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func withConsecutiveRoots(t *testing.T, repo syntheticRepository, count int) syntheticRepository {
	t.Helper()
	current := repo.root
	for index := 0; index < count; index++ {
		encoded, err := current.ToBytes(false)
		if err != nil {
			t.Fatal(err)
		}
		next, err := metadata.Root().FromBytes(encoded)
		if err != nil {
			t.Fatal(err)
		}
		next.Signed.Version = current.Signed.Version + 1
		next.Signatures = nil
		signSyntheticMetadataCount(t, next, repo.keys, ordinaryThreshold)
		nextBytes, err := next.ToBytes(false)
		if err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("https://release.invalid/metadata/%d.root.json", next.Signed.Version)
		repo.files[name] = nextBytes
		current = next
	}
	repo.root = current
	return repo
}

func withCrossEnvironmentRoot(t *testing.T, repo syntheticRepository) syntheticRepository {
	t.Helper()
	repo = withConsecutiveRoots(t, repo, 1)
	name := "https://release.invalid/metadata/2.root.json"
	root, err := metadata.Root().FromBytes(repo.files[name])
	if err != nil {
		t.Fatal(err)
	}
	root.Signed.UnrecognizedFields[rootEnvField] = "development"
	root.Signatures = nil
	signSyntheticMetadataCount(t, root, repo.keys, ordinaryThreshold)
	repo.files[name], err = root.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func withMetadataVersion(t *testing.T, repo syntheticRepository, version int64) syntheticRepository {
	t.Helper()
	files := make(map[string][]byte, len(repo.files))
	for name, data := range repo.files {
		files[name] = append([]byte(nil), data...)
	}
	repo.files = files
	targetBytes, err := repo.targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	targets, err := metadata.Targets().FromBytes(targetBytes)
	if err != nil {
		t.Fatal(err)
	}
	targets.Signed.Version = version
	targets.Signatures = nil
	signSyntheticMetadataCount(t, targets, repo.keys, ordinaryThreshold)
	targetBytes, err = targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	delete(repo.files, "https://release.invalid/metadata/1.targets.json")
	repo.files[fmt.Sprintf("https://release.invalid/metadata/%d.targets.json", version)] = targetBytes
	targetDigest := sha256.Sum256(targetBytes)

	snapshotBytes, err := repo.snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := metadata.Snapshot().FromBytes(snapshotBytes)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Signed.Version = version
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: version, Length: int64(len(targetBytes)), Hashes: metadata.Hashes{"sha256": targetDigest[:]}}
	snapshot.Signatures = nil
	signSyntheticMetadataCount(t, snapshot, repo.keys, ordinaryThreshold)
	snapshotBytes, err = snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	delete(repo.files, "https://release.invalid/metadata/1.snapshot.json")
	repo.files[fmt.Sprintf("https://release.invalid/metadata/%d.snapshot.json", version)] = snapshotBytes
	snapshotDigest := sha256.Sum256(snapshotBytes)

	timestampBytes, err := repo.timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	timestamp, err := metadata.Timestamp().FromBytes(timestampBytes)
	if err != nil {
		t.Fatal(err)
	}
	timestamp.Signed.Version = version
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: version, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotDigest[:]}}
	timestamp.Signatures = nil
	signSyntheticMetadataCount(t, timestamp, repo.keys, ordinaryThreshold)
	timestampBytes, err = timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	repo.files["https://release.invalid/metadata/timestamp.json"] = timestampBytes
	repo.targets, repo.snapshot, repo.timestamp = targets, snapshot, timestamp
	return repo
}

// buildCustomJSON renders the target custom identity block. The
// fields are documented in the lifecycle specification; the package
// rejects any unrecognized critical field name.
func buildCustomJSON(t *testing.T, opts syntheticOptions, targetDigest []byte) json.RawMessage {
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
	if opts.sbomIdentity == "" && !opts.omitSBOM {
		opts.sbomIdentity = "sbom-0001"
	}
	if opts.attestationPolicy == "" {
		opts.attestationPolicy = "two-builder"
	}
	if opts.qualification == "" {
		opts.qualification = "qualified"
	}
	if opts.buildState == "" {
		opts.buildState = "current"
	}
	if opts.protocolPhase == "" {
		opts.protocolPhase = "required"
	}
	if opts.protocolOverlappedSince.IsZero() && !opts.omitProtocolOverlap {
		opts.protocolOverlappedSince = testRefTime.Add(-100 * 24 * time.Hour)
	}
	if opts.buildSafetyNoNewWorkAfter.IsZero() {
		opts.buildSafetyNoNewWorkAfter = testRefTime.Add(30 * 24 * time.Hour)
	}
	if opts.buildSafetyTerminateAfter.IsZero() {
		opts.buildSafetyTerminateAfter = testRefTime.Add(180 * 24 * time.Hour)
	}
	encoded, err := json.Marshal(struct {
		SchemaVersion         int                  `json:"schema_version"`
		Profile               string               `json:"profile"`
		Platform              string               `json:"platform"`
		Architecture          string               `json:"architecture"`
		Environment           string               `json:"environment"`
		Network               string               `json:"network"`
		ReleaseIdentity       string               `json:"release_identity"`
		ReleaseVersion        int64                `json:"release_version"`
		SourceRevision        string               `json:"source_revision"`
		BuildInputCommitment  string               `json:"build_input_commitment"`
		BuildIdentity         string               `json:"build_identity"`
		DependencyIdentity    string               `json:"dependency_identity"`
		SBOMIdentity          string               `json:"sbom_identity"`
		AttestationPolicy     string               `json:"attestation_policy"`
		Qualification         string               `json:"qualification"`
		BuildState            string               `json:"build_state"`
		BuilderAttestations   []builderAttestation `json:"builder_attestations"`
		BuildSafetyNoNewAfter time.Time            `json:"build_safety_no_new_work_after"`
		BuildSafetyTermAfter  time.Time            `json:"build_safety_terminate_after"`
		ProtocolPhase         string               `json:"protocol_phase"`
		ProtocolOverlappedAt  time.Time            `json:"protocol_overlapped_since"`
		CapacityReady         bool                 `json:"capacity_ready"`
		DrainReady            bool                 `json:"drain_ready"`
		EmergencyReason       emergencyReason      `json:"emergency_reason,omitempty"`
		EmergencyExpiry       time.Time            `json:"emergency_expiry,omitempty"`
	}{
		SchemaVersion:         targetSchemaVersion,
		Profile:               targetProfile,
		Platform:              opts.platform,
		Architecture:          opts.architecture,
		Environment:           opts.environment,
		Network:               opts.network,
		ReleaseIdentity:       "ardents-release-0001",
		ReleaseVersion:        1,
		SourceRevision:        opts.sourceRevision,
		BuildInputCommitment:  "inputs-0001",
		BuildIdentity:         opts.buildIdentity,
		DependencyIdentity:    opts.dependencyIdentity,
		SBOMIdentity:          opts.sbomIdentity,
		AttestationPolicy:     opts.attestationPolicy,
		Qualification:         opts.qualification,
		BuildState:            opts.buildState,
		BuilderAttestations:   fixtureAttestations(opts, targetDigest),
		BuildSafetyNoNewAfter: opts.buildSafetyNoNewWorkAfter,
		BuildSafetyTermAfter:  opts.buildSafetyTerminateAfter,
		ProtocolPhase:         opts.protocolPhase,
		ProtocolOverlappedAt:  opts.protocolOverlappedSince,
		CapacityReady:         !opts.capacityNotReady,
		DrainReady:            !opts.drainNotReady,
		EmergencyReason:       emergencyReason(opts.emergencyReason),
		EmergencyExpiry:       opts.emergencyExpiry,
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.unknownCustomField {
		var object map[string]any
		if err := json.Unmarshal(encoded, &object); err != nil {
			t.Fatal(err)
		}
		object["future_critical_policy"] = "must-not-ignore"
		encoded, err = json.Marshal(object)
		if err != nil {
			t.Fatal(err)
		}
	}
	return encoded
}

func fixtureAttestations(opts syntheticOptions, targetDigest []byte) []builderAttestation {
	digest := hex.EncodeToString(targetDigest)
	if opts.attestationDigestMismatch {
		digest = hex.EncodeToString(make([]byte, sha256.Size))
	}
	records := []builderAttestation{
		{BuilderIdentity: "builder-a", BuildIdentity: opts.buildIdentity, SourceRevision: opts.sourceRevision,
			BuildInputCommitment: "inputs-0001", TargetSHA256: digest},
		{BuilderIdentity: "builder-b", BuildIdentity: opts.buildIdentity, SourceRevision: opts.sourceRevision,
			BuildInputCommitment: "inputs-0001", TargetSHA256: digest},
	}
	if opts.attestationInputMismatch {
		records[0].BuildInputCommitment = "other-inputs"
	}
	return records
}

// Key generation and signing helpers are in testfixtures_keys.go.
// memoryStore is an in-memory Store for tests that do not need to
// exercise the file-based atomic publication. The committed floors
// are kept in a struct; restart tampers are simulated by mutating
// the in-memory copy.
type memoryStore struct {
	closed      bool
	floors      FloorSet
	rootCommits []int64
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
func (store *memoryStore) CommitFloors(floors FloorSet, _ [][]byte) error {
	if store.closed {
		return fmt.Errorf("releasedecision: memory store is closed")
	}
	if err := validateMemoryFloorSet(floors); err != nil {
		return err
	}
	if err := validateMemoryAdvance(store.floors, floors); err != nil {
		return err
	}
	store.floors = floors
	return nil
}

func (store *memoryStore) CommitRoot(version int64, digest []byte, _ [][]byte) error {
	next := store.floors
	next.RootVersion = version
	next.RootDigest = append([]byte(nil), digest...)
	if err := store.CommitFloors(next, nil); err != nil {
		return err
	}
	store.rootCommits = append(store.rootCommits, version)
	return nil
}

// Close marks the in-memory store as closed.
func (store *memoryStore) Close() error {
	store.closed = true
	return nil
}

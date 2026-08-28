package custody

import (
	"crypto"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

const alphaEndpointTargetPath = "ardents/linux-amd64/endpoint"

type alphaBuilderAttestation struct {
	BuilderIdentity      string `json:"builder_identity"`
	BuildIdentity        string `json:"build_identity"`
	SourceRevision       string `json:"source_revision"`
	BuildInputCommitment string `json:"build_input_commitment"`
	TargetSHA256         string `json:"target_sha256"`
}

type alphaTargetDescriptor struct {
	SchemaVersion         int                       `json:"schema_version"`
	Profile               string                    `json:"profile"`
	Platform              string                    `json:"platform"`
	Architecture          string                    `json:"architecture"`
	Environment           string                    `json:"environment"`
	Network               string                    `json:"network"`
	ReleaseIdentity       string                    `json:"release_identity"`
	ReleaseVersion        int64                     `json:"release_version"`
	SourceRevision        string                    `json:"source_revision"`
	BuildInputCommitment  string                    `json:"build_input_commitment"`
	BuildIdentity         string                    `json:"build_identity"`
	DependencyIdentity    string                    `json:"dependency_identity"`
	SBOMIdentity          string                    `json:"sbom_identity"`
	AttestationPolicy     string                    `json:"attestation_policy"`
	Qualification         string                    `json:"qualification"`
	BuildState            string                    `json:"build_state"`
	BuilderAttestations   []alphaBuilderAttestation `json:"builder_attestations"`
	BuildSafetyNoNewAfter time.Time                 `json:"build_safety_no_new_work_after"`
	BuildSafetyTermAfter  time.Time                 `json:"build_safety_terminate_after"`
	ProtocolPhase         string                    `json:"protocol_phase"`
	ProtocolOverlappedAt  time.Time                 `json:"protocol_overlapped_since"`
	CapacityReady         bool                      `json:"capacity_ready"`
	DrainReady            bool                      `json:"drain_ready"`
	EmergencyReason       string                    `json:"emergency_reason,omitempty"`
	EmergencyExpiry       time.Time                 `json:"emergency_expiry,omitempty"`
}

func buildAlphaTUFVersion(request alphaInputsRequest, record seedRecord, endpoint, rootBytes []byte, generation uint64) (map[string][]byte, error) {
	if generation == 0 || generation > uint64(^uint64(0)>>1) {
		return nil, ErrInvalid
	}
	metadataVersion := int64(generation)
	if len(rootBytes) == 0 {
		root := metadata.Root(request.NotAfter)
		root.Signed.UnrecognizedFields = map[string]any{"ardents_schema_version": 1, "ardents_profile": "ardents-h3-release-v1", "ardents_environment": request.Environment, "ardents_network": request.Network}
		keyIDs := make([]string, 0, 5)
		for index := 0; index < 5; index++ {
			public := alphaRolePublic(record, index)
			key, err := metadata.KeyFromPublicKey(public)
			if err != nil {
				return nil, fmt.Errorf("construct TUF key: %w", err)
			}
			identifier, err := key.ID()
			if err != nil {
				return nil, fmt.Errorf("identify TUF key: %w", err)
			}
			root.Signed.Keys[identifier] = key
			keyIDs = append(keyIDs, identifier)
		}
		for _, role := range metadata.TOP_LEVEL_ROLE_NAMES {
			root.Signed.Roles[role] = &metadata.Role{KeyIDs: append([]string(nil), keyIDs...), Threshold: 3}
		}
		if err := signAlphaMetadata(root, record, 5); err != nil {
			return nil, err
		}
		encoded, err := root.ToBytes(false)
		if err != nil {
			return nil, fmt.Errorf("encode TUF root: %w", err)
		}
		rootBytes = encoded
	}

	targetDigest := sha256.Sum256(endpoint)
	digestText := hex.EncodeToString(targetDigest[:])
	attestations := make([]alphaBuilderAttestation, 2)
	for index, builder := range request.Builders {
		attestations[index] = alphaBuilderAttestation{BuilderIdentity: builder, BuildIdentity: request.BuildIdentity,
			SourceRevision: request.SourceRevision, BuildInputCommitment: request.BuildInputCommitment, TargetSHA256: digestText}
	}
	customBytes, err := json.Marshal(alphaTargetDescriptor{
		SchemaVersion: 1, Profile: "ardents-h3-release-v1", Platform: "linux-amd64", Architecture: "amd64",
		Environment: request.Environment, Network: request.Network, ReleaseIdentity: request.Release,
		ReleaseVersion: request.ReleaseVersion, SourceRevision: request.SourceRevision,
		BuildInputCommitment: request.BuildInputCommitment, BuildIdentity: request.BuildIdentity,
		DependencyIdentity: request.DependencyIdentity, SBOMIdentity: request.SBOMIdentity,
		AttestationPolicy: "two-builder", Qualification: request.Qualification, BuildState: request.BuildState,
		BuilderAttestations: attestations, BuildSafetyNoNewAfter: request.BuildSafetyNoNewWorkAfter,
		BuildSafetyTermAfter: request.BuildSafetyTerminateAfter, ProtocolPhase: request.ProtocolPhase,
		ProtocolOverlappedAt: request.ProtocolOverlappedSince, CapacityReady: request.CapacityReady,
		DrainReady: request.DrainReady, EmergencyReason: request.EmergencyReason, EmergencyExpiry: request.EmergencyExpiry,
	})
	if err != nil {
		return nil, fmt.Errorf("encode alpha target identity: %w", err)
	}
	custom := json.RawMessage(customBytes)
	targets := metadata.Targets(request.NotAfter)
	targets.Signed.Version = metadataVersion
	targets.Signed.Targets[alphaEndpointTargetPath] = &metadata.TargetFiles{Length: int64(len(endpoint)),
		Hashes: metadata.Hashes{"sha256": targetDigest[:]}, Path: alphaEndpointTargetPath, Custom: &custom}
	targetSignatureCount := 3
	if request.EmergencyReason != "" {
		targetSignatureCount = 4
	}
	if err := signAlphaMetadata(targets, record, targetSignatureCount); err != nil {
		return nil, err
	}
	targetsBytes, err := targets.ToBytes(false)
	if err != nil {
		return nil, fmt.Errorf("encode TUF targets: %w", err)
	}
	targetsDigest := sha256.Sum256(targetsBytes)

	snapshot := metadata.Snapshot(request.NotAfter)
	snapshot.Signed.Version = metadataVersion
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: metadataVersion, Length: int64(len(targetsBytes)), Hashes: metadata.Hashes{"sha256": targetsDigest[:]}}
	if err := signAlphaMetadata(snapshot, record, 3); err != nil {
		return nil, err
	}
	snapshotBytes, err := snapshot.ToBytes(false)
	if err != nil {
		return nil, fmt.Errorf("encode TUF snapshot: %w", err)
	}
	snapshotDigest := sha256.Sum256(snapshotBytes)

	timestamp := metadata.Timestamp(request.NotAfter)
	timestamp.Signed.Version = metadataVersion
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: metadataVersion, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotDigest[:]}}
	if err := signAlphaMetadata(timestamp, record, 3); err != nil {
		return nil, err
	}
	timestampBytes, err := timestamp.ToBytes(false)
	if err != nil {
		return nil, fmt.Errorf("encode TUF timestamp: %w", err)
	}
	version := fmt.Sprintf("%d", generation)
	return map[string][]byte{"1.root.json": append([]byte(nil), rootBytes...), version + ".targets.json": targetsBytes, version + ".snapshot.json": snapshotBytes, "timestamp.json": timestampBytes}, nil
}

func signAlphaMetadata(value interface {
	Sign(signature.Signer) (*metadata.Signature, error)
}, record seedRecord, count int) error {
	for index := 0; index < count; index++ {
		signer, err := signature.LoadSigner(ed25519.PrivateKey(record.Roles[index].Private), crypto.Hash(0))
		if err != nil {
			return fmt.Errorf("construct TUF signer: %w", err)
		}
		if _, err := value.Sign(signer); err != nil {
			return fmt.Errorf("sign TUF metadata: %w", err)
		}
	}
	return nil
}

func alphaRolePrivate(record seedRecord, index int) ed25519.PrivateKey {
	return ed25519.PrivateKey(record.Roles[index].Private)
}

func alphaRolePublic(record seedRecord, index int) ed25519.PublicKey {
	return alphaRolePrivate(record, index).Public().(ed25519.PublicKey)
}

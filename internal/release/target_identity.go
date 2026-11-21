package release

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/theupdateframework/go-tuf/v2/metadata"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	targetSchemaVersion = 1
	targetProfile       = "ardents-h3-release-v1"
)

type targetIdentityDescriptor struct {
	targetIdentity
	SchemaVersion         int
	Profile               string
	BuilderAttestations   []builderAttestation
	BuildSafetyNoNewAfter time.Time
	BuildSafetyTermAfter  time.Time
	ProtocolOverlappedAt  time.Time
	CapacityReady         bool
	DrainReady            bool
	EmergencyReason       emergencyReason
	EmergencyExpiry       time.Time
}

func customIdentity(target *metadata.TargetFiles) (targetIdentityDescriptor, error) {
	if target == nil {
		return targetIdentityDescriptor{}, errors.New("target is missing")
	}
	if target.Custom == nil {
		return targetIdentityDescriptor{}, errors.New("target identity is missing")
	}
	if err := validateJSONObjects(*target.Custom, true); err != nil {
		return targetIdentityDescriptor{}, err
	}
	var raw struct {
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
		EmergencyReason       emergencyReason      `json:"emergency_reason"`
		EmergencyExpiry       time.Time            `json:"emergency_expiry"`
	}
	decoder := json.NewDecoder(bytes.NewReader(*target.Custom))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return targetIdentityDescriptor{}, fmt.Errorf("decode target identity: %w", err)
	}
	descriptor := targetIdentityDescriptor{
		targetIdentity: targetIdentity{
			Platform: raw.Platform, Architecture: raw.Architecture,
			Environment: raw.Environment, Network: raw.Network,
			ReleaseIdentity: raw.ReleaseIdentity, ReleaseVersion: raw.ReleaseVersion,
			SourceRevision: raw.SourceRevision, BuildInputCommitment: raw.BuildInputCommitment,
			BuildIdentity: raw.BuildIdentity, DependencyIdentity: raw.DependencyIdentity,
			SBOMIdentity: raw.SBOMIdentity, AttestationPolicy: raw.AttestationPolicy,
			Qualification: raw.Qualification, BuildState: raw.BuildState, ProtocolPhase: raw.ProtocolPhase,
		},
		SchemaVersion:         raw.SchemaVersion,
		Profile:               raw.Profile,
		BuilderAttestations:   append([]builderAttestation(nil), raw.BuilderAttestations...),
		BuildSafetyNoNewAfter: raw.BuildSafetyNoNewAfter,
		BuildSafetyTermAfter:  raw.BuildSafetyTermAfter,
		ProtocolOverlappedAt:  raw.ProtocolOverlappedAt,
		CapacityReady:         raw.CapacityReady,
		DrainReady:            raw.DrainReady,
		EmergencyReason:       raw.EmergencyReason,
		EmergencyExpiry:       raw.EmergencyExpiry,
	}
	if err := validateTargetDescriptor(descriptor); err != nil {
		return descriptor, err
	}
	return descriptor, nil
}

func validateTargetDescriptor(value targetIdentityDescriptor) error {
	if value.SchemaVersion != targetSchemaVersion || value.Profile != targetProfile {
		return errors.New("target identity schema or profile is unsupported")
	}
	required := []struct{ name, value string }{
		{"platform", value.Platform}, {"architecture", value.Architecture},
		{"environment", value.Environment}, {"network", value.Network},
		{"release identity", value.ReleaseIdentity}, {"source revision", value.SourceRevision},
		{"build identity", value.BuildIdentity}, {"build input commitment", value.BuildInputCommitment},
		{"dependency identity", value.DependencyIdentity}, {"SBOM identity", value.SBOMIdentity},
		{"attestation policy", value.AttestationPolicy}, {"qualification", value.Qualification},
		{"build state", value.BuildState}, {"protocol phase", value.ProtocolPhase},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("target identity is missing %s", field.name)
		}
	}
	if value.ReleaseVersion <= 0 {
		return errors.New("target identity has an invalid release version")
	}
	if err := validateBuilderAttestations(value); err != nil {
		return err
	}
	if value.AttestationPolicy != "two-builder" {
		return errors.New("target identity has an unsupported attestation policy")
	}
	if _, ok := parseQualification(value.Qualification); !ok {
		return errors.New("target identity has an unsupported qualification state")
	}
	if _, ok := parseBuildState(value.BuildState); !ok {
		return errors.New("target identity has an unsupported build state")
	}
	if _, ok := parseProtocolPhase(value.ProtocolPhase); !ok {
		return errors.New("target identity has an unsupported protocol phase")
	}
	if value.BuildSafetyNoNewAfter.IsZero() || value.BuildSafetyTermAfter.IsZero() ||
		!value.BuildSafetyNoNewAfter.Before(value.BuildSafetyTermAfter) {
		return errors.New("target identity has invalid build safety bounds")
	}
	if (value.EmergencyReason == "") != value.EmergencyExpiry.IsZero() {
		return errors.New("target identity has an incomplete emergency transition")
	}
	return nil
}

func verifyTargetIdentity(target *metadata.TargetFiles, descriptor targetIdentityDescriptor, in Inputs, local LocalEnvironment) (Decision, error) {
	if target == nil {
		return Decision{}, errors.New("target is missing")
	}
	if err := confineTargetPath(in.TargetPath); err != nil {
		return Decision{}, err
	}
	if target.Length != int64(len(in.Artifact)) {
		return Decision{}, errors.New("artifact length does not match the target identity")
	}
	digest, ok := target.Hashes["sha256"]
	if !ok || len(digest) != 32 {
		return Decision{}, errors.New("target identity is missing the SHA-256 digest")
	}
	if err := verifyArtifactDigest(in.Artifact, digest); err != nil {
		return Decision{}, err
	}
	if err := verifyBuilderAttestations(descriptor, digest); err != nil {
		return Decision{}, err
	}
	if local.Platform == "" || descriptor.Platform != local.Platform {
		return Decision{}, ErrIncompatible
	}
	if local.Architecture == "" || descriptor.Architecture != local.Architecture {
		return Decision{}, ErrIncompatible
	}
	if local.Environment == "" || descriptor.Environment != local.Environment {
		return Decision{}, ErrIncompatible
	}
	if local.Network == "" || descriptor.Network != local.Network {
		return Decision{}, ErrIncompatible
	}
	return Decision{
		Path:                       in.TargetPath,
		Length:                     int64(len(in.Artifact)),
		Digest:                     append([]byte(nil), digest...),
		ReferenceTime:              local.RefTime.UTC(),
		BuildSafetyNoNewWorkAfter:  descriptor.BuildSafetyNoNewAfter.UTC(),
		BuildSafetyTerminateAfter:  descriptor.BuildSafetyTermAfter.UTC(),
		ProtocolTransitionDeadline: emergencyDeadlineUTC(descriptor.EmergencyExpiry),
		Platform:                   descriptor.Platform,
		Architecture:               descriptor.Architecture,
		Environment:                descriptor.Environment,
		Network:                    descriptor.Network,
		ReleaseIdentity:            descriptor.ReleaseIdentity,
		ReleaseVersion:             descriptor.ReleaseVersion,
		SourceRevision:             descriptor.SourceRevision,
		BuildInputCommitment:       descriptor.BuildInputCommitment,
		BuildIdentity:              descriptor.BuildIdentity,
		DependencyIdentity:         descriptor.DependencyIdentity,
		SBOMIdentity:               descriptor.SBOMIdentity,
		AttestationPolicy:          descriptor.AttestationPolicy,
		Qualification:              descriptor.Qualification,
		BuildState:                 descriptor.BuildState,
		ProtocolPhase:              descriptor.ProtocolPhase,
	}, nil
}

func emergencyDeadlineUTC(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC()
}

func verifyArtifactDigest(artifact, expected []byte) error {
	actual := sha256.Sum256(artifact)
	if !bytes.Equal(actual[:], expected) {
		return errors.New("artifact digest does not match the target identity")
	}
	return nil
}

func confineTargetPath(targetPath string) error {
	if targetPath == "" {
		return errors.New("target path is empty")
	}
	if strings.Contains(targetPath, `\`) {
		return errors.New("target path uses a non-canonical separator")
	}
	decoded, err := url.PathUnescape(targetPath)
	if err != nil {
		return fmt.Errorf("decode target path: %w", err)
	}
	cleaned := strings.TrimPrefix(path.Clean("/"+decoded), "/")
	if cleaned != decoded {
		return errors.New("target path is not confined")
	}
	for _, segment := range strings.Split(decoded, "/") {
		if segment == "." || segment == ".." {
			return errors.New("target path escapes the offline envelope")
		}
	}
	return nil
}

type builderAttestation struct {
	BuilderIdentity      string `json:"builder_identity"`
	BuildIdentity        string `json:"build_identity"`
	SourceRevision       string `json:"source_revision"`
	BuildInputCommitment string `json:"build_input_commitment"`
	TargetSHA256         string `json:"target_sha256"`
}

func validateBuilderAttestations(value targetIdentityDescriptor) error {
	if len(value.BuilderAttestations) != 2 {
		return errors.New("target identity requires two builder attestations")
	}
	first := value.BuilderAttestations[0]
	second := value.BuilderAttestations[1]
	if first.BuilderIdentity == "" || second.BuilderIdentity == "" ||
		first.BuilderIdentity == second.BuilderIdentity {
		return errors.New("target identity requires two distinct builder identities")
	}
	for _, record := range value.BuilderAttestations {
		if record.BuildIdentity != value.BuildIdentity ||
			record.SourceRevision != value.SourceRevision ||
			record.BuildInputCommitment != value.BuildInputCommitment {
			return errors.New("builder attestation does not match the authenticated build inputs")
		}
		decoded, err := hex.DecodeString(record.TargetSHA256)
		if err != nil || len(decoded) != 32 {
			return errors.New("builder attestation has an invalid target digest")
		}
	}
	return nil
}

func verifyBuilderAttestations(value targetIdentityDescriptor, digest []byte) error {
	expected := hex.EncodeToString(digest)
	for _, record := range value.BuilderAttestations {
		if record.TargetSHA256 != expected {
			return errors.New("builder attestation does not match the target digest")
		}
	}
	return nil
}

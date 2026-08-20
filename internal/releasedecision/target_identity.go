package releasedecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

const (
	targetSchemaVersion = 1
	targetProfile       = "ardents-h3-release-v1"
)

// targetIdentityDescriptor captures the authenticated identity fields
// every target must carry. The package compares them against the
// caller-supplied LocalEnvironment; missing or mismatched fields are
// release-incompatible or release-invalid.
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

// customIdentity reads the target's custom identity block. The TUF
// target file carries an optional "custom" JSON object that the H3
// profile uses to bind the artifact to its source, build, dependency,
// SBOM, attestation, qualification, and policy facts.
func customIdentity(target *metadata.TargetFiles) (targetIdentityDescriptor, error) {
	if target == nil {
		return targetIdentityDescriptor{}, errors.New("target is missing")
	}
	if target.Custom == nil {
		return targetIdentityDescriptor{}, errors.New("target identity is missing")
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

// verifyTargetIdentity compares the candidate target identity against
// the local binding and the artifact bytes. The target length and
// digest are verified by go-tuf; the package verifies path confinement
// and the full identity match.
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
		return Decision{}, errors.New("target platform does not match the local binding")
	}
	if local.Architecture == "" || descriptor.Architecture != local.Architecture {
		return Decision{}, errors.New("target architecture does not match the local binding")
	}
	if local.Environment == "" || descriptor.Environment != local.Environment {
		return Decision{}, errors.New("target environment does not match the local binding")
	}
	if local.Network == "" || descriptor.Network != local.Network {
		return Decision{}, errors.New("target network does not match the local binding")
	}
	return Decision{
		Path:                 in.TargetPath,
		Length:               int64(len(in.Artifact)),
		Digest:               append([]byte(nil), digest...),
		Platform:             descriptor.Platform,
		Architecture:         descriptor.Architecture,
		Environment:          descriptor.Environment,
		Network:              descriptor.Network,
		ReleaseIdentity:      descriptor.ReleaseIdentity,
		ReleaseVersion:       descriptor.ReleaseVersion,
		SourceRevision:       descriptor.SourceRevision,
		BuildInputCommitment: descriptor.BuildInputCommitment,
		BuildIdentity:        descriptor.BuildIdentity,
		DependencyIdentity:   descriptor.DependencyIdentity,
		SBOMIdentity:         descriptor.SBOMIdentity,
		AttestationPolicy:    descriptor.AttestationPolicy,
		Qualification:        descriptor.Qualification,
		BuildState:           descriptor.BuildState,
		ProtocolPhase:        descriptor.ProtocolPhase,
	}, nil
}

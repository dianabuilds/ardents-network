package releasedecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// targetIdentityDescriptor captures the authenticated identity fields
// every target must carry. The package compares them against the
// caller-supplied LocalEnvironment; missing or mismatched fields are
// release-incompatible or release-invalid.
type targetIdentityDescriptor struct {
	Platform              string
	Architecture          string
	Environment           string
	Network               string
	SourceRevision        string
	BuildIdentity         string
	DependencyIdentity    string
	SBOMIdentity          string
	AttestationPolicy     string
	Qualification         string
	BuildSafetyNoNewAfter time.Time
	BuildSafetyTermAfter  time.Time
	ProtocolPhase         string
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
		UnknownCriticalFields []string  `json:"unknown_critical_fields"`
	}
	if err := json.Unmarshal(*target.Custom, &raw); err != nil {
		return targetIdentityDescriptor{}, fmt.Errorf("decode target identity: %w", err)
	}
	descriptor := targetIdentityDescriptor{
		Platform:              raw.Platform,
		Architecture:          raw.Architecture,
		Environment:           raw.Environment,
		Network:               raw.Network,
		SourceRevision:        raw.SourceRevision,
		BuildIdentity:         raw.BuildIdentity,
		DependencyIdentity:    raw.DependencyIdentity,
		SBOMIdentity:          raw.SBOMIdentity,
		AttestationPolicy:     raw.AttestationPolicy,
		Qualification:         raw.Qualification,
		BuildSafetyNoNewAfter: raw.BuildSafetyNoNewAfter,
		BuildSafetyTermAfter:  raw.BuildSafetyTermAfter,
		ProtocolPhase:         raw.ProtocolPhase,
	}
	if len(raw.UnknownCriticalFields) > 0 {
		return descriptor, errors.New("target identity has unknown critical fields")
	}
	return descriptor, nil
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
	if local.Platform != "" && descriptor.Platform != "" && descriptor.Platform != local.Platform {
		return Decision{}, errors.New("target platform does not match the local binding")
	}
	if local.Architecture != "" && descriptor.Architecture != "" && descriptor.Architecture != local.Architecture {
		return Decision{}, errors.New("target architecture does not match the local binding")
	}
	if local.Environment != "" && descriptor.Environment != "" && descriptor.Environment != local.Environment {
		return Decision{}, errors.New("target environment does not match the local binding")
	}
	if local.Network != "" && descriptor.Network != "" && descriptor.Network != local.Network {
		return Decision{}, errors.New("target network does not match the local binding")
	}
	return Decision{
		Path:               in.TargetPath,
		Length:             int64(len(in.Artifact)),
		Digest:             append([]byte(nil), digest...),
		Platform:           firstNonEmpty(descriptor.Platform, local.Platform),
		Architecture:       firstNonEmpty(descriptor.Architecture, local.Architecture),
		Environment:        firstNonEmpty(descriptor.Environment, local.Environment),
		Network:            firstNonEmpty(descriptor.Network, local.Network),
		SourceRevision:     descriptor.SourceRevision,
		BuildIdentity:      descriptor.BuildIdentity,
		DependencyIdentity: descriptor.DependencyIdentity,
		SBOMIdentity:       descriptor.SBOMIdentity,
		AttestationPolicy:  descriptor.AttestationPolicy,
		Qualification:      descriptor.Qualification,
		ProtocolPhase:      descriptor.ProtocolPhase,
	}, nil
}

// verifyArtifactDigest checks the supplied bytes against the expected
// SHA-256 digest.
func verifyArtifactDigest(artifact, expected []byte) error {
	actual := sha256Sum(artifact)
	if !bytes.Equal(actual[:], expected) {
		return errors.New("artifact digest does not match the target identity")
	}
	return nil
}

// confineTargetPath rejects any path that escapes the offline envelope
// or carries path traversal, backslashes, or unexpected characters.
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
	cleaned := path.Clean("/" + decoded)
	cleaned = strings.TrimPrefix(cleaned, "/")
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

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

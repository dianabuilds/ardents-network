package custody

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

type alphaInputsRequest struct {
	Profile                            string
	Cohort, Release                    string
	ReleaseVersion                     int64
	ReferenceTime, NotBefore, NotAfter time.Time
	BuildSafetyNoNewWorkAfter          time.Time
	BuildSafetyTerminateAfter          time.Time
	Environment, Network               string
	SourceRevision                     string
	EndpointDigest, ControlDigest      [32]byte
	BuildInputCommitment               string
	BuildIdentity, DependencyIdentity  string
	SBOMIdentity                       string
	Qualification, BuildState          string
	ProtocolPhase                      string
	ProtocolOverlappedSince            time.Time
	CapacityReady, DrainReady          bool
	EmergencyReason                    string
	EmergencyExpiry                    time.Time
	Builders                           [2]string
	NetworkState                       alphaNetworkRequest
}

type alphaNetworkRequest struct {
	NetworkID, EpochDigest [32]byte
	Profile                string
	Threshold              uint8
	Authorities            []ed25519.PublicKey
	Epoch                  []byte
	Inputs, Materials      [][]byte
}

type alphaInputsRequestJSON struct {
	Schema                    string                  `json:"schema"`
	Profile                   string                  `json:"profile"`
	Cohort                    string                  `json:"cohort"`
	Release                   string                  `json:"release"`
	ReleaseVersion            int64                   `json:"release_version"`
	ReferenceTime             time.Time               `json:"reference_time"`
	NotBefore                 time.Time               `json:"not_before"`
	NotAfter                  time.Time               `json:"not_after"`
	BuildSafetyNoNewWorkAfter time.Time               `json:"build_safety_no_new_work_after"`
	BuildSafetyTerminateAfter time.Time               `json:"build_safety_terminate_after"`
	Environment               string                  `json:"environment"`
	Network                   string                  `json:"network"`
	SourceRevision            string                  `json:"source_revision"`
	EndpointSHA256            string                  `json:"endpoint_sha256"`
	ControlSHA256             string                  `json:"control_sha256"`
	BuildInputCommitment      string                  `json:"build_input_commitment"`
	BuildIdentity             string                  `json:"build_identity"`
	DependencyIdentity        string                  `json:"dependency_identity"`
	SBOMIdentity              string                  `json:"sbom_identity"`
	Qualification             string                  `json:"qualification"`
	BuildState                string                  `json:"build_state"`
	ProtocolPhase             string                  `json:"protocol_phase"`
	ProtocolOverlappedSince   time.Time               `json:"protocol_overlapped_since,omitempty"`
	CapacityReady             bool                    `json:"capacity_ready"`
	DrainReady                bool                    `json:"drain_ready"`
	EmergencyReason           string                  `json:"emergency_reason,omitempty"`
	EmergencyExpiry           time.Time               `json:"emergency_expiry,omitempty"`
	Builders                  []string                `json:"builders"`
	NetworkState              alphaNetworkRequestJSON `json:"network_state"`
}

type alphaNetworkRequestJSON struct {
	NetworkID   string   `json:"network_id"`
	EpochDigest string   `json:"epoch_digest"`
	Profile     string   `json:"profile"`
	Threshold   uint8    `json:"threshold"`
	Authorities []string `json:"authorities"`
	Epoch       []byte   `json:"epoch"`
	Inputs      [][]byte `json:"inputs"`
	Materials   [][]byte `json:"materials"`
}

func parseAlphaInputsRequest(raw, endpoint, control []byte, policy alphaInputPolicy, invokedAt time.Time) (alphaInputsRequest, error) {
	if len(raw) == 0 || len(raw) > maximumAlphaRequestBytes || len(endpoint) == 0 || len(endpoint) > maximumAlphaArtifactBytes ||
		len(control) == 0 || len(control) > maximumAlphaArtifactBytes || !validAlphaPolicy(policy) || invokedAt.IsZero() {
		return alphaInputsRequest{}, ErrInvalid
	}
	var parsed alphaInputsRequestJSON
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return alphaInputsRequest{}, ErrInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return alphaInputsRequest{}, ErrInvalid
	}
	endpointDigest, err := alphaDigest(parsed.EndpointSHA256)
	if err != nil || parsed.EndpointSHA256 != policy.EndpointSHA256 || endpointDigest != sha256.Sum256(endpoint) {
		return alphaInputsRequest{}, ErrInvalid
	}
	controlDigest, err := alphaDigest(parsed.ControlSHA256)
	if err != nil || parsed.ControlSHA256 != policy.ControlSHA256 || controlDigest != sha256.Sum256(control) {
		return alphaInputsRequest{}, ErrInvalid
	}
	networkID, err := alphaDigest(parsed.NetworkState.NetworkID)
	if err != nil || networkID == [32]byte{} {
		return alphaInputsRequest{}, ErrInvalid
	}
	epochDigest, err := alphaDigest(parsed.NetworkState.EpochDigest)
	if err != nil || epochDigest == [32]byte{} {
		return alphaInputsRequest{}, ErrInvalid
	}
	authorities := make([]ed25519.PublicKey, len(parsed.NetworkState.Authorities))
	seen := make(map[[32]byte]struct{}, len(authorities))
	for index, encoded := range parsed.NetworkState.Authorities {
		value, decodeErr := hex.DecodeString(encoded)
		if decodeErr != nil || len(value) != ed25519.PublicKeySize || encoded != strings.ToLower(encoded) {
			return alphaInputsRequest{}, ErrInvalid
		}
		identifier := sha256.Sum256(value)
		if _, exists := seen[identifier]; exists {
			return alphaInputsRequest{}, ErrInvalid
		}
		seen[identifier] = struct{}{}
		authorities[index] = ed25519.PublicKey(value)
	}
	if parsed.Schema != "ardents-alpha-input-request-v1" || parsed.Profile != policy.Profile ||
		!validAlphaToken(parsed.Cohort, 64) || !validAlphaToken(parsed.Release, 64) ||
		parsed.ReleaseVersion <= 0 || parsed.Environment != "alpha" || !validAlphaToken(parsed.Network, 64) ||
		parsed.SourceRevision != policy.SourceRevision || !validLowerHex(parsed.SourceRevision, 40) ||
		!validAlphaText(parsed.BuildInputCommitment, 255) ||
		!validAlphaText(parsed.BuildIdentity, 255) || !validAlphaText(parsed.DependencyIdentity, 255) ||
		!validAlphaText(parsed.SBOMIdentity, 255) || len(parsed.Builders) != 2 || parsed.Builders[0] == parsed.Builders[1] ||
		!validAlphaText(parsed.Builders[0], 255) || !validAlphaText(parsed.Builders[1], 255) ||
		!validAlphaTime(parsed.ReferenceTime) || !validAlphaTime(parsed.NotBefore) || !validAlphaTime(parsed.NotAfter) ||
		!validAlphaTime(parsed.BuildSafetyNoNewWorkAfter) || !validAlphaTime(parsed.BuildSafetyTerminateAfter) ||
		parsed.ReferenceTime.Before(parsed.NotBefore) || !parsed.ReferenceTime.Before(parsed.NotAfter) ||
		!parsed.ReferenceTime.Before(parsed.BuildSafetyNoNewWorkAfter) || !parsed.BuildSafetyNoNewWorkAfter.Before(parsed.BuildSafetyTerminateAfter) ||
		!validAlphaFreshness(invokedAt, parsed.NotAfter, parsed.BuildSafetyNoNewWorkAfter, parsed.EmergencyReason, parsed.EmergencyExpiry) ||
		!validQualification(parsed.Qualification) || !validBuildState(parsed.BuildState) || !validProtocolPhase(parsed.ProtocolPhase) ||
		!validOptionalAlphaTime(parsed.ProtocolOverlappedSince) || !validEmergency(parsed.EmergencyReason, parsed.EmergencyExpiry) ||
		!validAlphaText(parsed.NetworkState.Profile, 64) || len(authorities) == 0 || len(authorities) > 16 ||
		parsed.NetworkState.Threshold == 0 || int(parsed.NetworkState.Threshold) > len(authorities) ||
		len(parsed.NetworkState.Epoch) == 0 || len(parsed.NetworkState.Epoch) > 1<<20 || len(parsed.NetworkState.Inputs) > 64 || len(parsed.NetworkState.Materials) > 64 {
		return alphaInputsRequest{}, ErrInvalid
	}
	for _, group := range [][][]byte{parsed.NetworkState.Inputs, parsed.NetworkState.Materials} {
		for _, member := range group {
			if len(member) == 0 || len(member) > 35<<10 {
				return alphaInputsRequest{}, ErrInvalid
			}
		}
	}
	return alphaInputsRequest{
		Profile: parsed.Profile, Cohort: parsed.Cohort, Release: parsed.Release, ReleaseVersion: parsed.ReleaseVersion,
		ReferenceTime: parsed.ReferenceTime.UTC(), NotBefore: parsed.NotBefore.UTC(), NotAfter: parsed.NotAfter.UTC(),
		BuildSafetyNoNewWorkAfter: parsed.BuildSafetyNoNewWorkAfter.UTC(), BuildSafetyTerminateAfter: parsed.BuildSafetyTerminateAfter.UTC(),
		Environment: parsed.Environment, Network: parsed.Network, SourceRevision: parsed.SourceRevision,
		EndpointDigest: endpointDigest, ControlDigest: controlDigest, BuildInputCommitment: parsed.BuildInputCommitment,
		BuildIdentity: parsed.BuildIdentity, DependencyIdentity: parsed.DependencyIdentity, SBOMIdentity: parsed.SBOMIdentity,
		Qualification: parsed.Qualification, BuildState: parsed.BuildState, ProtocolPhase: parsed.ProtocolPhase,
		ProtocolOverlappedSince: parsed.ProtocolOverlappedSince.UTC(), CapacityReady: parsed.CapacityReady, DrainReady: parsed.DrainReady,
		EmergencyReason: parsed.EmergencyReason, EmergencyExpiry: parsed.EmergencyExpiry.UTC(),
		Builders: [2]string{parsed.Builders[0], parsed.Builders[1]},
		NetworkState: alphaNetworkRequest{NetworkID: networkID, EpochDigest: epochDigest, Profile: parsed.NetworkState.Profile,
			Threshold: parsed.NetworkState.Threshold, Authorities: authorities, Epoch: cloneAlphaBytes(parsed.NetworkState.Epoch),
			Inputs: cloneAlphaGroup(parsed.NetworkState.Inputs), Materials: cloneAlphaGroup(parsed.NetworkState.Materials)},
	}, nil
}

func validAlphaPolicy(policy alphaInputPolicy) bool {
	if policy.Profile == "" || !validLowerHex(policy.SourceRevision, 40) || !validLowerHex(policy.EndpointSHA256, 64) ||
		!validLowerHex(policy.ControlSHA256, 64) || !validLowerHex(policy.EnvelopeSHA256, 64) {
		return false
	}
	return true
}

func alphaDigest(encoded string) ([32]byte, error) {
	var result [32]byte
	value, err := hex.DecodeString(encoded)
	if err != nil || len(value) != len(result) || encoded != strings.ToLower(encoded) {
		return result, ErrInvalid
	}
	copy(result[:], value)
	return result, nil
}

func validLowerHex(value string, length int) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(value) == length && len(decoded)*2 == length && value == strings.ToLower(value)
}

func validAlphaToken(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			if character < 'a' || character > 'z' {
				if character < '0' || character > '9' {
					if character != '.' && character != '_' && character != '-' {
						return false
					}
				}
			}
		}
	}
	return true
}

func validAlphaText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value &&
		!strings.ContainsAny(value, "\x00\r\n\t")
}

func validAlphaTime(value time.Time) bool {
	return !value.IsZero() && value.Equal(value.UTC().Truncate(time.Second))
}

func validOptionalAlphaTime(value time.Time) bool { return value.IsZero() || validAlphaTime(value) }

func validQualification(value string) bool {
	return value == "qualified" || value == "development-only" || value == "revoked" || value == "unavailable"
}

func validBuildState(value string) bool {
	return value == "current" || value == "superseded" || value == "vulnerable" || value == "revoked"
}

func validProtocolPhase(value string) bool {
	return value == "announced" || value == "overlap-supported" || value == "preferred" || value == "required" || value == "retired"
}

func validEmergency(reason string, expiry time.Time) bool {
	if reason == "" {
		return expiry.IsZero()
	}
	return validAlphaTime(expiry) && (reason == "credible-exploitable-flaw" || reason == "compromised-primitive-or-key" || reason == "demonstrated-safety-incompatibility")
}

func alphaInputsFresh(request alphaInputsRequest, at time.Time) bool {
	return validAlphaFreshness(at, request.NotAfter, request.BuildSafetyNoNewWorkAfter, request.EmergencyReason, request.EmergencyExpiry)
}

func validAlphaFreshness(at, notAfter, noNewWorkAfter time.Time, emergencyReason string, emergencyExpiry time.Time) bool {
	return !at.IsZero() && at.Before(notAfter) && at.Before(noNewWorkAfter) &&
		(emergencyReason == "" || at.Before(emergencyExpiry))
}

func cloneAlphaBytes(value []byte) []byte { return append([]byte(nil), value...) }

func cloneAlphaGroup(values [][]byte) [][]byte {
	result := make([][]byte, len(values))
	for index, value := range values {
		result[index] = cloneAlphaBytes(value)
	}
	return result
}

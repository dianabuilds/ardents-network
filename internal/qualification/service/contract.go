package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"time"
)

const schema = "ardents-h3-service-evidence-v1"

// Verdict is the independent terminal machine result.
type Verdict struct {
	Schema         string `json:"schema"`
	Verdict        string `json:"verdict"`
	Reason         string `json:"reason"`
	EvidenceDigest string `json:"evidence_digest"`
}

// Verify decodes and independently checks one bounded Stage 3 evidence value.
func Verify(raw []byte) Verdict {
	invalid := func(reason string) Verdict {
		return Verdict{Schema: "ardents-h3-service-verdict-v1", Verdict: "invalid", Reason: reason}
	}
	if len(raw) == 0 || len(raw) > 4<<20 {
		return invalid("evidence is empty or exceeds 4 MiB")
	}
	var input candidate
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return invalid("evidence JSON is malformed or non-canonical")
	}
	claimed, err := hex.DecodeString(input.EvidenceDigest)
	if err != nil || len(claimed) != sha256.Size {
		return invalid("evidence digest is malformed")
	}
	input.EvidenceDigest = ""
	canonical, err := json.Marshal(input)
	if err != nil {
		return invalid("evidence cannot be canonicalized")
	}
	digest := sha256.Sum256(canonical)
	if !bytes.Equal(claimed, digest[:]) {
		return invalid("evidence digest does not match")
	}
	if err := validateEvidence(input); err != nil {
		return invalid(err.Error())
	}
	if err := validateCandidate(input); err != nil {
		return Verdict{Schema: "ardents-h3-service-verdict-v1", Verdict: "fail", Reason: err.Error(),
			EvidenceDigest: hex.EncodeToString(digest[:])}
	}
	return Verdict{Schema: "ardents-h3-service-verdict-v1", Verdict: "pass",
		Reason:         "bounded Stage 3 Service Connection evidence passed every conjunct",
		EvidenceDigest: hex.EncodeToString(digest[:])}
}

func validateEvidence(input candidate) error {
	commit, commitErr := hex.DecodeString(input.SourceCommit)
	image, imageErr := hex.DecodeString(strings.TrimPrefix(input.ImageID, "sha256:"))
	manifest, manifestErr := hex.DecodeString(input.ManifestDigest)
	if input.Schema != schema || commitErr != nil || len(commit) != 20 || imageErr != nil || len(image) != 32 ||
		!strings.HasPrefix(input.ImageID, "sha256:") || manifestErr != nil || len(manifest) != 32 ||
		input.NetworkID == [32]byte{} || input.AuthorityPublic == [32]byte{} || input.Target == [32]byte{} ||
		input.IntroductionPublic == [32]byte{} || input.RouteManifestDigest == [32]byte{} ||
		len(input.Generations) != 2 || len(input.Topology) == 0 || len(input.Topology) > 256<<10 {
		return errors.New("identity, schema, generation, or secret-handling evidence is incomplete")
	}
	recomputedShortcuts, err := validateTopology([]byte(input.Topology))
	if err != nil {
		return err
	}
	for _, required := range requiredShortcuts {
		observedRouteChain := required == "direct" || required == "shortened"
		if !input.ShortcutsAbsent[required] || (!observedRouteChain && !recomputedShortcuts[required]) {
			return errors.New("forbidden-path evidence is not supported by retained topology: " + required)
		}
	}
	cleanup := input.CleanupObservation
	if !cleanup.Observed || cleanup.Project == "" || !cleanup.FixtureAbsent || len(cleanup.Containers) != 0 ||
		len(cleanup.Networks) != 0 || len(cleanup.Volumes) != 0 || !input.PrivateMaterialAbsent {
		return errors.New("post-cleanup resource observation is incomplete")
	}
	for _, required := range requiredNegatives {
		if _, ok := input.Negatives[required]; !ok || input.NegativeMechanisms[required] != expectedNegativeMechanisms[required] {
			return errors.New("mandatory negative evidence is missing: " + required)
		}
	}
	if !input.OperationObservations["backpressure"] || !input.OperationObservations["cancellation"] ||
		!input.OperationObservations["partial-write"] || input.OperationClasses["cancellation"] != "local timeout or cancellation" ||
		input.OperationClasses["partial-write"] != "abrupt connection loss" ||
		input.OperationCounts["cancellation-accepted"] != 0 || input.OperationCounts["cancellation-received"] != 0 ||
		input.OperationCounts["partial-low"] != 1024 || input.OperationCounts["partial-high"] != 2048 {
		return errors.New("backpressure, cancellation, or partial-write observations are incomplete")
	}
	for _, required := range requiredShortcuts {
		if _, ok := input.ShortcutsAbsent[required]; !ok {
			return errors.New("forbidden-path evidence is missing: " + required)
		}
	}
	for _, required := range requiredCleanup {
		if _, ok := input.Cleanup[required]; !ok {
			return errors.New("cleanup evidence is missing: " + required)
		}
	}
	for _, generation := range input.Generations {
		if len(generation.IntroductionAcknowledgement) == 0 || !generation.PublicationReady ||
			len(generation.Roles) != 6 || len(generation.ContainerIDs) != 12 {
			return errors.New("publication or process evidence is incomplete")
		}
		roles, runtimes := map[string]bool{}, map[string]bool{}
		var sourceID string
		var buildDigest [32]byte
		for _, role := range generation.Roles {
			if role.Role == "" || role.PID < 1 || role.RuntimeID == "" || roles[role.Role] || runtimes[role.RuntimeID] ||
				role.ManifestDigest != input.RouteManifestDigest || role.NetworkID != input.NetworkID ||
				role.EpochDigest == [32]byte{} || role.OpaqueBytes == 0 ||
				role.ReverseOpaqueBytes == 0 || role.SourceID == "" || role.BuildDigest == [32]byte{} ||
				role.OpaqueDigest == [32]byte{} || role.ReverseOpaqueDigest == [32]byte{} {
				return errors.New("route process separation evidence is contradictory")
			}
			if sourceID == "" {
				sourceID, buildDigest = role.SourceID, role.BuildDigest
			} else if role.SourceID != sourceID || role.BuildDigest != buildDigest {
				return errors.New("route source or build observations differ")
			}
			roles[role.Role], runtimes[role.RuntimeID] = true, true
		}
		for _, required := range routeRoles {
			if !roles[required] {
				return errors.New("route process evidence is missing: " + required)
			}
		}
		seen := map[string]bool{}
		for _, identity := range generation.ContainerIDs {
			if identity == "" || seen[identity] {
				return errors.New("container separation evidence is incomplete or repeated")
			}
			seen[identity] = true
		}
		for runtime := range runtimes {
			bound := false
			for identity := range seen {
				bound = bound || strings.HasPrefix(identity, runtime)
			}
			if !bound {
				return errors.New("route runtime identity is not bound to an observed container")
			}
		}
		for index, endpoint := range []endpointEvidence{generation.ClientEndpoint, generation.PublisherEndpoint} {
			grant := []grantEvidence{generation.ClientGrant, generation.PublisherGrant}[index]
			if endpoint.PrincipalCommitment == [32]byte{} || endpoint.SessionCommitment == [32]byte{} ||
				grant.Broker == [32]byte{} || grant.Principal == [32]byte{} || grant.Surface != "connection" ||
				endpoint.PrincipalCommitment != evidenceCommitment("principal", grant.Principal) ||
				endpoint.BrokerCommitment != evidenceCommitment("broker", grant.Broker) ||
				endpoint.GrantCommitment != evidenceGrantCommitment(grant) || endpoint.GrantSurface != grant.Surface ||
				!endpoint.SessionConsumed || endpoint.SessionIssuedAt == 0 ||
				endpoint.SessionExpiresAt-endpoint.SessionIssuedAt != int64(15*time.Second) || endpoint.MemoryHighWater == 0 ||
				endpoint.MemoryHighWater > 512<<20 || math.IsNaN(endpoint.CPUSeconds) || endpoint.CPUSeconds < 0 ||
				endpoint.OpenFilesHighWater == 0 || endpoint.OpenFilesHighWater > 48 || endpoint.GoroutinesHighWater == 0 ||
				endpoint.GoroutinesHighWater > 24 || endpoint.ActiveSessions != 0 || endpoint.TimerHighWater == 0 ||
				endpoint.TimerHighWater > 16 || endpoint.QueueHighWater == 0 || endpoint.QueueHighWater > 64<<10 ||
				endpoint.AcceptedIPCHighWater == 0 || endpoint.AcceptedIPCHighWater > 8 ||
				endpoint.ServiceConnectionsHighWater == 0 || endpoint.ServiceConnectionsHighWater > 2 ||
				endpoint.ControlFilesHighWater == 0 || endpoint.ControlFilesHighWater > 8 {
				return errors.New("endpoint grant, session, or resource observation violates its bound")
			}
		}
	}
	return nil
}

func evidenceCommitment(kind string, value [32]byte) [32]byte {
	return sha256.Sum256(append([]byte("ardents-h3-service-"+kind+"-v1\x00"), value[:]...))
}

func evidenceGrantCommitment(grant grantEvidence) [32]byte {
	value := append([]byte("ardents-h3-service-grant-v1\x00"), grant.Broker[:]...)
	value = append(value, grant.Principal[:]...)
	value = append(value, grant.Surface...)
	return sha256.Sum256(value)
}

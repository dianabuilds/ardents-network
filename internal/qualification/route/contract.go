package route

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var roles = [...]string{"initiator", "introduction", "rendezvous", "responder"}

// Case freezes the independently known manifest commitments for one attempt.
type Case struct {
	RawEvidence        []byte
	EvidenceDigest     [32]byte
	ManifestDigest     [32]byte
	NetworkID          [32]byte
	Generation         string
	Epoch              uint64
	EpochDigest        [32]byte
	Profile            string
	ViewRoot           [32]byte
	SelectionSeed      [32]byte
	SelectionAt        int64
	Candidates         []Candidate
	ExcludedIdentities [][32]byte
	ExcludedFamilies   []string
	ExcludedDomains    []string
	NodeIDs            [4][32]byte
	PublicKeys         [4][32]byte
	Families           [4]string
	Endpoints          [4]string
	ClientPin          [32]byte
	PublisherID        [32]byte
	SourceID           string
	BuildDigest        [32]byte
	ExitedPIDs         [6]int
	ExitedRuntimeIDs   [6]string
	ContainerIDs       [6]string
	CleanupVerified    bool
}

// Candidate is one frozen authenticated Candidate View entry independently
// consumed by the qualification verifier.
type Candidate struct {
	NodeID, PublicKey        [32]byte
	Family, Endpoint, Domain string
	Capacity                 uint16
	ValidFrom, ValidUntil    int64
}

// Commit returns the canonical commitment to a Case manifest without its
// evidence bytes or the two self-referential digest fields.
func Commit(input Case) [32]byte {
	input.RawEvidence = nil
	input.EvidenceDigest = [32]byte{}
	input.ManifestDigest = [32]byte{}
	input.SourceID = ""
	input.BuildDigest = [32]byte{}
	input.ExitedPIDs = [6]int{}
	input.ExitedRuntimeIDs = [6]string{}
	input.ContainerIDs = [6]string{}
	input.CleanupVerified = false
	raw, _ := json.Marshal(input)
	return sha256.Sum256(raw)
}

// Result is the terminal independently recomputed machine classification.
type Result struct {
	Verdict        string `json:"verdict"`
	Reason         string `json:"reason"`
	EvidenceDigest string `json:"evidence_digest,omitempty"`
}

type observation struct {
	Schema              string     `json:"schema"`
	Kind                string     `json:"kind"`
	Role                string     `json:"role"`
	Generation          string     `json:"generation"`
	Error               string     `json:"error"`
	PID                 int        `json:"pid"`
	RuntimeID           string     `json:"runtime_id"`
	SourceID            string     `json:"source_id"`
	BuildDigest         [32]byte   `json:"build_digest"`
	ManifestDigest      [32]byte   `json:"manifest_digest"`
	NetworkID           [32]byte   `json:"network_id"`
	EpochDigest         [32]byte   `json:"epoch_digest"`
	Profile             string     `json:"profile"`
	ViewRoot            [32]byte   `json:"view_root"`
	SelectionSeed       [32]byte   `json:"selection_seed"`
	SelectionAt         int64      `json:"selection_at"`
	ExcludedIdentities  [][32]byte `json:"excluded_identities"`
	ExcludedFamilies    []string   `json:"excluded_families"`
	ExcludedDomains     []string   `json:"excluded_domains"`
	NodeID              [32]byte   `json:"node_id"`
	PreviousPin         [32]byte   `json:"previous_pin"`
	NextNodeID          [32]byte   `json:"next_node_id"`
	Epoch               uint64     `json:"epoch"`
	OpaqueBytes         uint64     `json:"opaque_bytes"`
	OpaqueDigest        [32]byte   `json:"opaque_digest"`
	ReverseOpaqueBytes  uint64     `json:"reverse_opaque_bytes"`
	ReverseOpaqueDigest [32]byte   `json:"reverse_opaque_digest"`
	CanaryDigest        [32]byte   `json:"canary_digest"`
	CanaryLength        uint32     `json:"canary_length"`
	Canary              []byte     `json:"canary"`
	Positions           []position `json:"positions"`
	PeerAuthenticated   bool       `json:"peer_authenticated"`
	DeadlineMillis      uint32     `json:"deadline_millis"`
	Cancelled           bool       `json:"cancelled"`
	Cleanup             bool       `json:"cleanup"`
	Terminal            string     `json:"terminal"`
}

type position struct {
	Role      string   `json:"Role"`
	Family    string   `json:"Family"`
	Endpoint  string   `json:"Endpoint"`
	Domain    string   `json:"Domain"`
	NodeID    [32]byte `json:"NodeID"`
	PublicKey [32]byte `json:"PublicKey"`
	Capacity  uint16   `json:"Capacity"`
}

// Evaluate independently verifies evidence integrity and every frozen
// selection, role-local knowledge, byte, and process conjunct.
func Evaluate(input Case) Result {
	digest := sha256.Sum256(input.RawEvidence)
	digestText := hex.EncodeToString(digest[:])
	if err := validateCase(input, digest); err != nil {
		return Result{Verdict: "invalid", Reason: err.Error(), EvidenceDigest: digestText}
	}
	observations, err := decode(input.RawEvidence)
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error(), EvidenceDigest: digestText}
	}
	if err := verifyComplete(input, observations); err != nil {
		return Result{Verdict: "invalid", Reason: err.Error(), EvidenceDigest: digestText}
	}
	if err := verifyCandidate(input, observations); err != nil {
		return Result{Verdict: "fail", Reason: err.Error(), EvidenceDigest: digestText}
	}
	return Result{Verdict: "pass", Reason: "complete Route evidence passed independent verification", EvidenceDigest: digestText}
}

func validateCase(input Case, digest [32]byte) error {
	if len(input.RawEvidence) == 0 || len(input.RawEvidence) > 256<<10 || input.EvidenceDigest == [32]byte{} ||
		digest != input.EvidenceDigest || input.NetworkID == [32]byte{} || input.Generation == "" ||
		input.Epoch == 0 || input.EpochDigest == [32]byte{} || input.Profile != "h3-route-tracer-v1" ||
		input.ViewRoot == [32]byte{} || input.SelectionSeed == [32]byte{} || input.SelectionAt <= 0 ||
		len(input.Candidates) == 0 || len(input.Candidates) > 64 || input.ClientPin == [32]byte{} ||
		input.PublisherID == [32]byte{} || input.SourceID == "" || input.BuildDigest == [32]byte{} ||
		!input.CleanupVerified || input.ManifestDigest == [32]byte{} || Commit(input) != input.ManifestDigest {
		return errors.New("manifest or evidence integrity is invalid")
	}
	seen := map[string]bool{}
	containerSeen := map[string]bool{}
	for index, pid := range input.ExitedPIDs {
		runtimeID := input.ExitedRuntimeIDs[index]
		containerID := input.ContainerIDs[index]
		if pid < 1 || runtimeID == "" || seen[runtimeID] || len(containerID) < 12 ||
			containerSeen[containerID] || !strings.HasPrefix(containerID, runtimeID) {
			return errors.New("external process-exit evidence is invalid")
		}
		seen[runtimeID] = true
		containerSeen[containerID] = true
	}
	return nil
}

func decode(raw []byte) ([]observation, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	values := make([]observation, 0, 6)
	for scanner.Scan() {
		var value observation
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode bounded role evidence: %w", err)
		}
		values = append(values, value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func verifyComplete(input Case, values []observation) error {
	if len(values) != 6 {
		return errors.New("evidence does not contain exactly six terminal processes")
	}
	seenProcess, seenRole := map[string]bool{}, map[string]bool{}
	exited := map[string]int{}
	for index, runtimeID := range input.ExitedRuntimeIDs {
		exited[runtimeID] = input.ExitedPIDs[index]
	}
	for _, value := range values {
		if value.Schema != "ardents-h3-route-observation-v1" || value.Kind != "complete" || value.PID < 1 ||
			value.RuntimeID == "" || seenProcess[value.RuntimeID] || seenRole[value.Role] || exited[value.RuntimeID] != value.PID ||
			value.NetworkID != input.NetworkID ||
			value.EpochDigest != input.EpochDigest || value.SourceID != input.SourceID || value.BuildDigest != input.BuildDigest ||
			value.ManifestDigest != input.ManifestDigest || value.Terminal != "success" && value.Terminal != "error" ||
			value.Terminal == "success" && value.Error != "" || value.Terminal == "error" && value.Error == "" ||
			value.Cancelled || !value.Cleanup || value.DeadlineMillis == 0 || value.DeadlineMillis > 15_000 {
			return errors.New("process evidence identity, schema, or state binding is incomplete")
		}
		seenProcess[value.RuntimeID], seenRole[value.Role] = true, true
	}
	for _, role := range append(roles[:], "client", "publisher") {
		if !seenRole[role] {
			return errors.New("mandatory role evidence is missing")
		}
	}
	return nil
}

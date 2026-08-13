package route

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

var roles = [...]string{"initiator", "introduction", "rendezvous", "responder"}

// Case freezes the independently known manifest commitments for one attempt.
type Case struct {
	RawEvidence    []byte
	EvidenceDigest [32]byte
	NetworkID      [32]byte
	Generation     string
	Epoch          uint64
	EpochDigest    [32]byte
	NodeIDs        [4][32]byte
	PublicKeys     [4][32]byte
	Families       [4]string
	ClientPin      [32]byte
	PublisherID    [32]byte
}

// Result is the terminal independently recomputed machine classification.
type Result struct {
	Verdict        string `json:"verdict"`
	Reason         string `json:"reason"`
	EvidenceDigest string `json:"evidence_digest,omitempty"`
}

type observation struct {
	Schema       string     `json:"schema"`
	Kind         string     `json:"kind"`
	Role         string     `json:"role"`
	Generation   string     `json:"generation"`
	Error        string     `json:"error"`
	PID          int        `json:"pid"`
	NetworkID    [32]byte   `json:"network_id"`
	EpochDigest  [32]byte   `json:"epoch_digest"`
	NodeID       [32]byte   `json:"node_id"`
	PreviousPin  [32]byte   `json:"previous_pin"`
	NextNodeID   [32]byte   `json:"next_node_id"`
	Epoch        uint64     `json:"epoch"`
	OpaqueBytes  uint64     `json:"opaque_bytes"`
	OpaqueDigest [32]byte   `json:"opaque_digest"`
	CanaryDigest [32]byte   `json:"canary_digest"`
	CanaryLength uint32     `json:"canary_length"`
	Canary       []byte     `json:"canary"`
	Positions    []position `json:"positions"`
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
		input.Epoch == 0 || input.EpochDigest == [32]byte{} || input.ClientPin == [32]byte{} || input.PublisherID == [32]byte{} {
		return errors.New("manifest or evidence integrity is invalid")
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
	seenPID, seenRole := map[int]bool{}, map[string]bool{}
	for _, value := range values {
		if value.Schema != "ardents-h3-route-observation-v1" || value.Kind != "complete" || value.PID < 1 ||
			seenPID[value.PID] || seenRole[value.Role] || value.NetworkID != input.NetworkID || value.EpochDigest != input.EpochDigest {
			return errors.New("process evidence identity, schema, or state binding is incomplete")
		}
		seenPID[value.PID], seenRole[value.Role] = true, true
	}
	for _, role := range append(roles[:], "client", "publisher") {
		if !seenRole[role] {
			return errors.New("mandatory role evidence is missing")
		}
	}
	return nil
}

func verifyCandidate(input Case, values []observation) error {
	byRole := make(map[string]observation, len(values))
	for _, value := range values {
		byRole[value.Role] = value
		if value.Error != "" {
			return errors.New("candidate process reported a terminal error")
		}
	}
	client, publisher := byRole["client"], byRole["publisher"]
	if client.Generation != input.Generation || client.Epoch != input.Epoch || len(client.Positions) != 4 ||
		client.CanaryLength != 32 || len(client.Canary) != 32 || sha256.Sum256(client.Canary) != client.CanaryDigest ||
		publisher.NodeID != input.PublisherID || publisher.CanaryLength != 32 || publisher.CanaryDigest != client.CanaryDigest ||
		len(publisher.Positions) != 0 || len(publisher.Canary) != 0 {
		return errors.New("client plan or publisher canary evidence violates the frozen contract")
	}
	identities, keys, families := map[[32]byte]bool{}, map[[32]byte]bool{}, map[string]bool{}
	for index, expectedRole := range roles {
		position, node := client.Positions[index], byRole[expectedRole]
		if position.Role != expectedRole || position.Domain != expectedRole || position.NodeID != input.NodeIDs[index] ||
			position.PublicKey != input.PublicKeys[index] || position.Family != input.Families[index] || position.Capacity == 0 ||
			identities[position.NodeID] || keys[position.PublicKey] || families[position.Family] || node.NodeID != position.NodeID ||
			node.OpaqueBytes == 0 || node.OpaqueDigest == [32]byte{} || node.CanaryDigest != [32]byte{} || len(node.Canary) != 0 || len(node.Positions) != 0 {
			return errors.New("route selection or role-local evidence violates identity/family/domain separation")
		}
		previous := input.ClientPin
		if index > 0 {
			previous = input.PublicKeys[index-1]
		}
		next := input.PublisherID
		if index < 3 {
			next = input.NodeIDs[index+1]
		}
		if node.PreviousPin != previous || node.NextNodeID != next {
			return errors.New("node received more or different Route adjacency than its role permits")
		}
		identities[position.NodeID], keys[position.PublicKey], families[position.Family] = true, true, true
	}
	if publisher.PreviousPin != input.PublicKeys[3] {
		return errors.New("publisher attachment is not bound to the selected Responder")
	}
	return nil
}

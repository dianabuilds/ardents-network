package route_test

import (
	"crypto/sha256"
	"encoding/json"
	"testing"

	qualification "github.com/dianabuilds/ardents-network/internal/qualification/route"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestEvidenceIntegritySeparatesPassFailAndInvalid(t *testing.T) {
	input := evidenceCase(t)
	if result := qualification.Evaluate(input); result.Verdict != "pass" {
		t.Fatalf("complete evidence did not pass: %+v", result)
	}
	corrupt := input
	corrupt.RawEvidence = append([]byte(nil), input.RawEvidence...)
	corrupt.RawEvidence[10] ^= 1
	if result := qualification.Evaluate(corrupt); result.Verdict != "invalid" {
		t.Fatalf("corrupt evidence = %s, want invalid", result.Verdict)
	}
	missing := input
	lines := splitLines(input.RawEvidence)
	missing.RawEvidence = joinLines(lines[:5])
	missing.EvidenceDigest = sha256.Sum256(missing.RawEvidence)
	if result := qualification.Evaluate(missing); result.Verdict != "invalid" {
		t.Fatalf("missing process evidence = %s, want invalid", result.Verdict)
	}
	violated := input
	var publisher route.Evidence
	if err := json.Unmarshal(lines[5], &publisher); err != nil {
		t.Fatal(err)
	}
	publisher.CanaryDigest[0] ^= 1
	lines[5], _ = json.Marshal(publisher)
	violated.RawEvidence = joinLines(lines)
	violated.EvidenceDigest = sha256.Sum256(violated.RawEvidence)
	if result := qualification.Evaluate(violated); result.Verdict != "fail" {
		t.Fatalf("complete wrong-byte evidence = %s, want fail", result.Verdict)
	}
	short := input
	lines = splitLines(input.RawEvidence)
	var client route.Evidence
	if err := json.Unmarshal(lines[0], &client); err != nil {
		t.Fatal(err)
	}
	client.Positions = client.Positions[:3]
	lines[0], _ = json.Marshal(client)
	short.RawEvidence = joinLines(lines)
	short.EvidenceDigest = sha256.Sum256(short.RawEvidence)
	if result := qualification.Evaluate(short); result.Verdict != "fail" {
		t.Fatalf("complete short-path evidence = %s, want fail", result.Verdict)
	}
}

func evidenceCase(t *testing.T) qualification.Case {
	t.Helper()
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	input := qualification.Case{NetworkID: [32]byte{1}, Generation: "generation", Epoch: 1,
		EpochDigest: [32]byte{2}, ClientPin: [32]byte{30}, PublisherID: [32]byte{40}}
	canary := make([]byte, 32)
	for index := range canary {
		canary[index] = byte(index + 1)
	}
	digest := sha256.Sum256(canary)
	client := route.Evidence{Schema: "ardents-h3-route-observation-v1", Kind: "complete", Role: "client", PID: 1,
		NetworkID: input.NetworkID, Generation: input.Generation, Epoch: input.Epoch, EpochDigest: input.EpochDigest,
		CanaryLength: 32, CanaryDigest: digest, Canary: canary}
	for index, role := range roles {
		input.NodeIDs[index], input.PublicKeys[index], input.Families[index] = [32]byte{byte(index + 3)}, [32]byte{byte(index + 10)}, role+"-family"
		client.Positions = append(client.Positions, route.Position{Role: role, Domain: role, NodeID: input.NodeIDs[index],
			PublicKey: input.PublicKeys[index], Family: input.Families[index], Endpoint: "127.0.0.1:4101", Capacity: 1})
	}
	values := []route.Evidence{client}
	for index, role := range roles {
		previous := input.ClientPin
		if index > 0 {
			previous = input.PublicKeys[index-1]
		}
		next := input.PublisherID
		if index < 3 {
			next = input.NodeIDs[index+1]
		}
		values = append(values, route.Evidence{Schema: client.Schema, Kind: "complete", Role: role, PID: index + 2,
			NetworkID: input.NetworkID, EpochDigest: input.EpochDigest, NodeID: input.NodeIDs[index],
			PreviousPin: previous, NextNodeID: next, OpaqueBytes: 100, OpaqueDigest: [32]byte{50}})
	}
	values = append(values, route.Evidence{Schema: client.Schema, Kind: "complete", Role: "publisher", PID: 6,
		NetworkID: input.NetworkID, EpochDigest: input.EpochDigest, NodeID: input.PublisherID,
		PreviousPin: input.PublicKeys[3], CanaryLength: 32, CanaryDigest: digest})
	lines := make([][]byte, len(values))
	for index := range values {
		lines[index], _ = json.Marshal(values[index])
	}
	input.RawEvidence = joinLines(lines)
	input.EvidenceDigest = sha256.Sum256(input.RawEvidence)
	return input
}

func splitLines(raw []byte) [][]byte {
	var result [][]byte
	start := 0
	for index, value := range raw {
		if value == '\n' {
			result = append(result, append([]byte(nil), raw[start:index]...))
			start = index + 1
		}
	}
	return result
}

func joinLines(lines [][]byte) []byte {
	var result []byte
	for _, line := range lines {
		result = append(result, line...)
		result = append(result, '\n')
	}
	return result
}

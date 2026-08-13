package service

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestVerifierSeparatesPassFailAndInvalid(t *testing.T) {
	input := validCandidate(t)
	raw := committed(t, input)
	if verdict := Verify(raw); verdict.Verdict != "pass" {
		t.Fatalf("valid evidence did not pass: %+v", verdict)
	}
	mutated := append([]byte(nil), raw...)
	for index := range mutated {
		if mutated[index] == '1' {
			mutated[index] = '2'
			break
		}
	}
	if verdict := Verify(mutated); verdict.Verdict != "invalid" {
		t.Fatalf("mutated evidence was not invalid: %+v", verdict)
	}
	input.Generations[0].ClientApplication.SentDigest[0]++
	if verdict := Verify(committed(t, input)); verdict.Verdict != "fail" {
		t.Fatalf("complete wrong-byte evidence was not fail: %+v", verdict)
	}
}

func validCandidate(t *testing.T) candidate {
	t.Helper()
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	var authority [32]byte
	copy(authority[:], public)
	input := candidate{Schema: schema, SourceCommit: "0123456789abcdef", ImageID: "sha256:image",
		ManifestDigest: "manifest", NetworkID: [32]byte{1}, AuthorityPublic: authority,
		Target: targetFor(authority), Generations: make([]generationEvidence, 2),
		Negatives: map[string]bool{}, ShortcutsAbsent: map[string]bool{}, Cleanup: map[string]bool{}, PrivateMaterialAbsent: true}
	for _, name := range requiredNegatives {
		input.Negatives[name] = true
	}
	for _, name := range requiredShortcuts {
		input.ShortcutsAbsent[name] = true
	}
	for _, name := range requiredCleanup {
		input.Cleanup[name] = true
	}
	for index := range input.Generations {
		instancePublic, _, keyErr := ed25519.GenerateKey(nil)
		if keyErr != nil {
			t.Fatal(keyErr)
		}
		var instance [32]byte
		copy(instance[:], instancePublic)
		credential := publicCredential{AuthorityPublic: authority, Target: input.Target, InstancePublic: instance,
			NetworkID: input.NetworkID, Generation: uint64(index + 1), NotBefore: 1, NotAfter: 4_000_000_000, Capabilities: 3}
		copy(credential.Signature[:], ed25519.Sign(private, credentialBody(credential)))
		sentClient := sha256.Sum256([]byte{byte(index), 1})
		sentPublisher := sha256.Sum256([]byte{byte(index), 2})
		endpoint := endpointEvidence{Class: "clean service connection close", AuthenticatedTarget: input.Target,
			Generation: uint64(index + 1), AcceptedBytes: 64 << 10, ReceivedBytes: 64 << 10}
		generation := generationEvidence{Generation: uint64(index + 1), Credential: credential,
			IntroductionAcknowledgement: [32]byte{byte(index + 1)}, PublicationReady: true,
			ClientEndpoint: endpoint, PublisherEndpoint: endpoint,
			ClientApplication: applicationEvidence{Schema: "ardents-h3-stream-application-v1", Role: "client",
				Terminal: "success", SentBytes: 64 << 10, ReceivedBytes: 64 << 10,
				SentDigest: sentClient, ReceivedDigest: sentPublisher},
			PublisherApplication: applicationEvidence{Schema: "ardents-h3-stream-application-v1", Role: "publisher",
				Terminal: "success", SentBytes: 64 << 10, ReceivedBytes: 64 << 10,
				SentDigest: sentPublisher, ReceivedDigest: sentClient}, ContainerIDs: make([]string, 12)}
		for roleIndex, role := range routeRoles {
			generation.Roles = append(generation.Roles, roleEvidence{Role: role, PID: index*10 + roleIndex + 1,
				RuntimeID: string(rune('a' + index*10 + roleIndex)), Terminal: "success", Cleanup: true})
		}
		for containerIndex := range generation.ContainerIDs {
			generation.ContainerIDs[containerIndex] = string(rune('A' + index*10 + containerIndex))
		}
		input.Generations[index] = generation
	}
	return input
}

func committed(t *testing.T, input candidate) []byte {
	t.Helper()
	input.EvidenceDigest = ""
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	input.EvidenceDigest = hex.EncodeToString(digest[:])
	raw, err = json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

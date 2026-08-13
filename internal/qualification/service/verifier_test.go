package service

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
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
	introductionPublic, introductionPrivate, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	var introduction [32]byte
	copy(introduction[:], introductionPublic)
	input := candidate{Schema: schema, SourceCommit: "0123456789abcdef0123456789abcdef01234567",
		ImageID: "sha256:" + string(make([]byte, 0)), NetworkID: [32]byte{1}, AuthorityPublic: authority,
		IntroductionPublic: introduction, RouteManifestDigest: [32]byte{2},
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
	input.ImageID = "sha256:" + hex.EncodeToString(make([]byte, 32))
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
			Generation: uint64(index + 1), AcceptedBytes: 64 << 10, ReceivedBytes: 64 << 10,
			ConnectionCanary: [32]byte{byte(index + 3)}}
		generation := generationEvidence{Generation: uint64(index + 1), Credential: credential,
			IntroductionAcknowledgement: signedReceipt(credential, input, introductionPrivate), PublicationReady: true,
			ClientEndpoint: endpoint, PublisherEndpoint: endpoint,
			ClientApplication: applicationEvidence{Schema: "ardents-h3-stream-application-v1", Role: "client",
				Terminal: "success", SentBytes: 64 << 10, ReceivedBytes: 64 << 10,
				SentDigest: sentClient, ReceivedDigest: sentPublisher},
			PublisherApplication: applicationEvidence{Schema: "ardents-h3-stream-application-v1", Role: "publisher",
				Terminal: "success", SentBytes: 64 << 10, ReceivedBytes: 64 << 10,
				SentDigest: sentPublisher, ReceivedDigest: sentClient}, ContainerIDs: make([]string, 12)}
		for roleIndex, role := range routeRoles {
			runtime := string(rune('a' + index*10 + roleIndex))
			generation.Roles = append(generation.Roles, roleEvidence{Role: role, PID: index*10 + roleIndex + 1,
				RuntimeID: runtime, Terminal: "success", Cleanup: true, ManifestDigest: input.RouteManifestDigest,
				NetworkID: input.NetworkID, OpaqueBytes: 1})
			generation.ContainerIDs[roleIndex] = runtime + "-container"
		}
		for containerIndex := len(routeRoles); containerIndex < len(generation.ContainerIDs); containerIndex++ {
			generation.ContainerIDs[containerIndex] = "extra-" + string(rune('A'+index*10+containerIndex))
		}
		input.Generations[index] = generation
	}
	credentialDigest := sha256.Sum256(append(credentialJSON(input.Generations[0].Credential),
		credentialJSON(input.Generations[1].Credential)...))
	commitment := make([]byte, 0, 32*6)
	for _, field := range [][32]byte{input.RouteManifestDigest, input.NetworkID, input.AuthorityPublic,
		input.IntroductionPublic, input.Target, credentialDigest} {
		commitment = append(commitment, field[:]...)
	}
	input.ManifestDigest = hexDigest(sha256.Sum256(commitment))
	return input
}

func signedReceipt(credential publicCredential, input candidate, private ed25519.PrivateKey) []byte {
	body := make([]byte, 149)
	copy(body[:4], "ASIA")
	body[4] = 1
	copy(body[5:37], credential.Target[:])
	binary.BigEndian.PutUint64(body[37:45], credential.Generation)
	binary.BigEndian.PutUint64(body[45:53], uint64(credential.NotAfter))
	copy(body[53:85], input.NetworkID[:])
	body[85], body[117] = 1, 1
	return append(body, ed25519.Sign(private, append([]byte("ardents-h3-introduction-ack-v1\x00"), body...))...)
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

package service

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
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

func TestVerifierRecomputesOwnedResourcesRouteAndHostileBoundary(t *testing.T) {
	tests := map[string]func(*candidate){
		"timer unavailable":    func(value *candidate) { value.Generations[0].ClientEndpoint.TimerHighWater = 0 },
		"IPC exceeds bound":    func(value *candidate) { value.Generations[0].ClientEndpoint.AcceptedIPCHighWater = 9 },
		"shortened route":      func(value *candidate) { value.Generations[0].Roles[1].NextNodeID = [32]byte{5} },
		"route epoch differs":  func(value *candidate) { value.Generations[0].Roles[2].EpochDigest = [32]byte{8} },
		"route digest differs": func(value *candidate) { value.Generations[0].Roles[3].OpaqueDigest = [32]byte{8} },
		"route endpoint differs": func(value *candidate) {
			value.Generations[0].Roles[0].Positions[1].Endpoint = "172.31.20.99:4605"
		},
		"route port differs": func(value *candidate) {
			value.Generations[0].Roles[0].Positions[0].Endpoint = "172.31.20.11:9999"
		},
		"hostile topology mount": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology, "  hostile-sibling:\n    network_mode: none",
				"  hostile-sibling:\n    network_mode: none\n    volumes:\n      - type: volume\n        source: client_app\n        target: /run/ardents/client-app", 1)
		},
		"operator bind mount": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology, "- type: volume\n        source: administration",
				"- type: bind\n        source: /var/run/docker.sock", 1)
		},
		"client Docker socket mount": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology, "        target: /run/ardents/client-app",
				"        target: /run/ardents/client-app\n      - type: bind\n        source: /var/run/docker.sock\n        target: /var/run/docker.sock", 1)
		},
		"client workload writable": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology, "        target: /run/ardents/workload/client.hex\n        read_only: true",
				"        target: /run/ardents/workload/client.hex\n        read_only: false", 1)
		},
		"application ALL_PROXY": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology, "  client-app:\n    network_mode: none",
				"  client-app:\n    network_mode: none\n    environment:\n      ALL_PROXY: socks5://proxy:1080", 1)
		},
		"unrelated internal network": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology, "networks:\n  route_net:\n    internal: true",
				"networks:\n  route_net:\n  unrelated:\n    internal: true", 1)
		},
		"internal label decoy": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology, "  route_net:\n    internal: true",
				"  route_net:\n    internal: false\n    labels:\n      proof: \"internal: true\"", 1)
		},
		"mount label decoy": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology,
				"    volumes:\n      - type: volume\n        source: administration\n        target: /run/ardents/admin",
				"    labels:\n      type: volume\n      source: administration\n      target: /run/ardents/admin", 1)
		},
		"hostile inline mount": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology, "  hostile-sibling:\n    network_mode: none",
				"  hostile-sibling:\n    network_mode: none\n    volumes: [client_app:/run/ardents/client-app]", 1)
		},
		"operator short mount": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology, "        target: /run/ardents/admin",
				"        target: /run/ardents/admin\n      - client_app:/run/ardents/client-app", 1)
		},
		"operator ambient network": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology, "  publication-operator:\n    network_mode: none",
				"  publication-operator:\n    network_mode: none\n    networks:\n      route_net:", 1)
		},
		"network mode label decoy": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology, "  hostile-sibling:\n    network_mode: none",
				"  hostile-sibling:\n    network_mode: bridge\n    labels:\n      proof: \"network_mode: none\"", 1)
		},
		"route IP label decoy": func(value *candidate) {
			value.Topology = strings.Replace(value.Topology,
				"      route_net:\n        ipv4_address: 172.31.20.11",
				"      route_net:\n        ipv4_address: 172.31.20.99\n    labels:\n      proof: \"ipv4_address: 172.31.20.11\"", 1)
		},
		"hostile mount": func(value *candidate) {
			value.Generations[0].HostileSibling.MountDestinations = []string{"/run/ardents/client-app"}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			value := validCandidate(t)
			mutate(&value)
			if verdict := Verify(committed(t, value)); verdict.Verdict == "pass" {
				t.Fatalf("unsupported evidence was not rejected: %+v", verdict)
			}
		})
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
		Target: targetFor(authority), Topology: validTopology(), Generations: make([]generationEvidence, 2),
		Negatives: map[string]bool{}, NegativeMechanisms: map[string]string{}, ShortcutsAbsent: map[string]bool{}, Cleanup: map[string]bool{}, PrivateMaterialAbsent: true,
		OperationObservations: map[string]bool{"backpressure": true, "cancellation": true, "partial-write": true},
		OperationClasses:      map[string]string{"cancellation": "local timeout or cancellation", "partial-write": "abrupt connection loss"},
		OperationCounts:       map[string]uint32{"cancellation-accepted": 0, "cancellation-received": 0, "partial-low": 1024, "partial-high": 2048},
		CleanupObservation: cleanupObservation{Observed: true, Project: "stage-3-test", FixtureAbsent: true,
			Containers: []string{}, Networks: []string{}, Volumes: []string{}}}
	for _, name := range requiredNegatives {
		input.Negatives[name] = true
		input.NegativeMechanisms[name] = expectedNegativeMechanisms[name]
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
		clientSeed, publisherSeed := [32]byte{byte(index + 1)}, [32]byte{byte(index + 3)}
		sentClient := sha256.Sum256(expectedWorkload(clientSeed))
		sentPublisher := sha256.Sum256(expectedWorkload(publisherSeed))
		endpoint := endpointEvidence{Class: "clean service connection close", AuthenticatedTarget: input.Target,
			Generation: uint64(index + 1), AcceptedBytes: 64 << 10, ReceivedBytes: 64 << 10,
			ConnectionCanary: [32]byte{byte(index + 3)}, PrincipalCommitment: [32]byte{4},
			SessionCommitment: [32]byte{5}, GrantSurface: "connection", SessionConsumed: true,
			MemoryHighWater: 1 << 20, CPUSeconds: 1, OpenFilesHighWater: 4, GoroutinesHighWater: 2,
			TimerHighWater: 1, QueueHighWater: 16 << 10, SessionIssuedAt: 1,
			SessionExpiresAt: 1 + int64(15*time.Second), AcceptedIPCHighWater: 2,
			ServiceConnectionsHighWater: 1, ControlFilesHighWater: 2}
		clientGrant := grantEvidence{Broker: [32]byte{byte(index + 10)}, Principal: [32]byte{byte(index + 11)}, Surface: "connection"}
		publisherGrant := grantEvidence{Broker: [32]byte{byte(index + 12)}, Principal: [32]byte{byte(index + 13)}, Surface: "connection"}
		endpointFor := func(grant grantEvidence) endpointEvidence {
			value := endpoint
			value.PrincipalCommitment = evidenceCommitment("principal", grant.Principal)
			value.BrokerCommitment = evidenceCommitment("broker", grant.Broker)
			value.GrantCommitment = evidenceGrantCommitment(grant)
			return value
		}
		generation := generationEvidence{Generation: uint64(index + 1), Credential: credential,
			IntroductionAcknowledgement: signedReceipt(credential, input, introductionPrivate), PublicationReady: true,
			ClientEndpoint: endpointFor(clientGrant), PublisherEndpoint: endpointFor(publisherGrant),
			ClientGrant: clientGrant, PublisherGrant: publisherGrant,
			ClientApplication: applicationEvidence{Schema: "ardents-h3-stream-application-v1", Role: "client",
				Terminal: "success", SentBytes: 64 << 10, ReceivedBytes: 64 << 10,
				SentDigest: sentClient, ReceivedDigest: sentPublisher,
				ResultClass: "clean service connection close", AuthenticatedTarget: input.Target,
				SendSeed: clientSeed, ExpectSeed: publisherSeed},
			PublisherApplication: applicationEvidence{Schema: "ardents-h3-stream-application-v1", Role: "publisher",
				Terminal: "success", SentBytes: 64 << 10, ReceivedBytes: 64 << 10,
				SentDigest: sentPublisher, ReceivedDigest: sentClient,
				ResultClass: "clean service connection close", AuthenticatedTarget: input.Target,
				SendSeed: publisherSeed, ExpectSeed: clientSeed},
			ContainerIDs: make([]string, 12)}
		for roleIndex, role := range routeRoles {
			runtime := string(rune('a' + index*10 + roleIndex))
			receipt := roleEvidence{Role: role, PID: index*10 + roleIndex + 1,
				RuntimeID: runtime, Terminal: "success", Cleanup: true, ManifestDigest: input.RouteManifestDigest,
				NetworkID: input.NetworkID, EpochDigest: [32]byte{9}, OpaqueBytes: 64 << 10, SourceID: "source@version",
				BuildDigest: [32]byte{6}, OpaqueDigest: [32]byte{byte(index + 7)}, ReverseOpaqueBytes: 64 << 10,
				ReverseOpaqueDigest: [32]byte{byte(index + 8)}}
			if roleIndex == 0 {
				for position := 0; position < 4; position++ {
					digit := string(rune('1' + position))
					receipt.Positions = append(receipt.Positions, routePositionEvidence{Role: routeRoles[position+1],
						NodeID: [32]byte{byte(position + 1)}, Endpoint: "172.31.20.1" + digit + ":460" + digit})
				}
			} else {
				receipt.NodeID = [32]byte{byte(roleIndex)}
				if roleIndex < len(routeRoles)-1 {
					receipt.NextNodeID = [32]byte{byte(roleIndex + 1)}
				}
			}
			generation.Roles = append(generation.Roles, receipt)
			generation.ContainerIDs[roleIndex] = runtime + strings.Repeat("a", 64-len(runtime))
		}
		for containerIndex := len(routeRoles); containerIndex < len(generation.ContainerIDs); containerIndex++ {
			generation.ContainerIDs[containerIndex] = strings.Repeat(string(rune('0'+containerIndex%10)), 64)
		}
		generation.HostileSibling = hostileObservation{RuntimeID: generation.ContainerIDs[11], ExitCode: 1,
			Output: "dial unix /run/ardents/not-granted/app.sock: no such file or directory"}
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

func validTopology() string {
	return "services:\n" +
		"  client:\n    networks:\n      route_net:\n        ipv4_address: 172.31.20.10\n  hostile-sibling:\n    network_mode: none\n  negative-suite:\n    image: test\n" +
		"  publication-operator:\n    network_mode: none\n    volumes:\n      - type: volume\n        source: administration\n        target: /run/ardents/admin\n" +
		"  verifier:\n    image: test\n  volume-init:\n    image: test\n" +
		"  initiator:\n    networks:\n      route_net:\n        ipv4_address: 172.31.20.11\n" +
		"  introduction:\n    networks:\n      route_net:\n        ipv4_address: 172.31.20.12\n" +
		"  rendezvous:\n    networks:\n      route_net:\n        ipv4_address: 172.31.20.13\n" +
		"  responder:\n    networks:\n      route_net:\n        ipv4_address: 172.31.20.14\n" +
		"  publisher:\n    networks:\n      route_net:\n        ipv4_address: 172.31.20.16\n" +
		"  client-app:\n    network_mode: none\n    volumes:\n" +
		"      - type: volume\n        source: client_app\n        target: /run/ardents/client-app\n" +
		"      - type: bind\n        source: C:\\fixture/client-seed.hex\n        target: /run/ardents/workload/client.hex\n        read_only: true\n" +
		"      - type: bind\n        source: C:\\fixture/publisher-seed.hex\n        target: /run/ardents/workload/publisher.hex\n        read_only: true\n" +
		"  publisher-app:\n    network_mode: none\n    volumes:\n" +
		"      - type: volume\n        source: publisher_app\n        target: /run/ardents/publisher-app\n" +
		"      - type: bind\n        source: C:\\fixture/publisher-seed.hex\n        target: /run/ardents/workload/publisher.hex\n        read_only: true\n" +
		"      - type: bind\n        source: C:\\fixture/client-seed.hex\n        target: /run/ardents/workload/client.hex\n        read_only: true\n" +
		"  client-endpoint:\n    network_mode: none\n  publisher-endpoint:\n    network_mode: none\n" +
		"networks:\n  route_net:\n    internal: true\n"
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

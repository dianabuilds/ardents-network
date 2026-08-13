package recovery

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestVerifyAcceptsTwoCompleteDirectionalCells(t *testing.T) {
	evidence := validEvidence()
	if result := Verify(evidence); result.Verdict != "pass" {
		t.Fatalf("complete evidence rejected: %+v", result)
	}
}

func TestVerifyRejectsMutationMissingEvidenceAndCandidateFailure(t *testing.T) {
	for name, mutate := range map[string]func(*Evidence){
		"mutated bytes":    func(value *Evidence) { value.Cells[0].ObservedDigest[0]++ },
		"missing negative": func(value *Evidence) { delete(value.Negatives, "deadline") },
		"reconnect":        func(value *Evidence) { value.Cells[1].ApplicationReconnected = true },
		"late canary":      func(value *Evidence) { value.Cells[0].CanaryAtNanos += int64(6 * time.Second) },
		"secret cleanup":   func(value *Evidence) { value.Cleanup.PrivateMaterialAbsent = false },
	} {
		t.Run(name, func(t *testing.T) {
			value := validEvidence()
			mutate(&value)
			if result := Verify(value); result.Verdict == "pass" {
				t.Fatalf("mutation passed: %+v", result)
			}
		})
	}
}

func validEvidence() Evidence {
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	var authority [32]byte
	copy(authority[:], private.Public().(ed25519.PublicKey))
	target := sha256.Sum256(append([]byte("ardents-h3-service-target-v1\x00"), authority[:]...))
	manifest := PublicManifest{RouteManifest: [32]byte{4}, NetworkID: [32]byte{3}, AuthorityPublic: authority,
		IntroductionPublic: [32]byte{10}, Target: target, InstancePublic: [32]byte{2}, ClientPrincipal: [32]byte{8},
		PublisherPrincipal: [32]byte{9}, CredentialGeneration: 1, CredentialNotBefore: 1, CredentialNotAfter: 100,
		CredentialCapabilities: 3, RouteProfile: "profile", WorkSafetyNotAfter: 90, WorkSafetyMaximum: 100,
		NoNewRecoveryAfter: 80}
	copy(manifest.CredentialSignature[:], ed25519.Sign(private, credentialBody(manifest)))
	manifestDigest := publicManifestDigest(manifest)
	topology := []byte("services:\n" +
		"  client-app:\n    network_mode: none\n  publisher-app:\n    network_mode: none\n" +
		"  client-endpoint:\n    network_mode: none\n  publisher-endpoint:\n    network_mode: none\n" +
		"  recovery-verifier:\n    network_mode: none\n    command: [/run/ardents/evidence.json]\n    volumes: [/run/ardents/evidence.json]\n" +
		"  client:\n    networks: [route_net]\n  publisher:\n    networks: [route_net]\n" +
		"  initiator:\n    networks: [route_net]\n  introduction:\n    networks: [route_net]\n" +
		"  rendezvous:\n    networks: [route_net]\n  responder:\n    networks: [route_net]\n" +
		"networks:\n  route_net:\n    internal: true\n")
	value := Evidence{Schema: schema, SourceCommit: "0123456789012345678901234567890123456789", ImageID: "sha256:image", VerifierImageID: "sha256:image",
		Topology: topology, TopologyDigest: hexDigest(topology), Manifest: manifest,
		ManifestDigest: hex.EncodeToString(manifestDigest[:]), Claim: "S4.1 local development evidence only",
		Target: target, Instance: [32]byte{2}, NetworkID: [32]byte{3}, CandidateView: [32]byte{4},
		AuthorityPublic: authority,
		ClientPrincipal: [32]byte{8}, PublisherPrincipal: [32]byte{9}, RouteProfile: "profile",
		CredentialGeneration: 1, CredentialNotBefore: 1, CredentialNotAfter: 100, WorkSafetyNotAfter: 90,
		WorkSafetyMaximum: 100, NoNewRecoveryAfter: 80,
		BinaryDigests: map[string]string{"ardents-route": strings.Repeat("a", 64), "ardents-service": strings.Repeat("b", 64),
			"ardents-stream-app": strings.Repeat("c", 64), "ardents-recovery-qualify": strings.Repeat("d", 64)},
		RequestedNanos: int64(10 * time.Minute), CampaignNanos: int64(10 * time.Minute),
		Negatives: map[string]Negative{}, Cleanup: Cleanup{DockerEmpty: true, FixtureAbsent: true, PrivateMaterialAbsent: true}}
	value.IsolationContext = sha256.Sum256(append([]byte("isolation\x00"), manifestDigest[:]...))
	value.DestinationBinding = sha256.Sum256(append([]byte("destination\x00"), target[:]...))
	for _, name := range negativeNames {
		value.Negatives[name] = Negative{TerminalCount: 1, Class: "terminal", WithinNanos: int64(time.Second), Passed: true,
			ContainerID: name + "-container"}
	}
	for name, kind := range map[string]string{"replayed-attachment": "recovery-replayed-attachment",
		"stale-attachment": "recovery-stale-attachment", "cross-binding": "recovery-cross-binding"} {
		negative := value.Negatives[name]
		negative.InjectionKind, negative.InjectionDigest = kind, strings.Repeat("e", 64)
		negative.AttackAttempts, negative.RouteGeneration = 1, 1
		if name == "replayed-attachment" {
			negative.AttackAttempts, negative.RecoveryCount, negative.RouteGeneration = 2, 1, 2
		}
		value.Negatives[name] = negative
	}
	value.Negatives["endpoint-restart"] = Negative{TerminalCount: 1, Class: "terminal", WithinNanos: int64(time.Second),
		Passed: true, ContainerID: "endpoint-container", InjectedResource: "publisher-endpoint", BeforeProcess: "101", AfterProcess: "202"}
	for index, direction := range []string{"client-to-publisher", "publisher-to-client"} {
		seed := [32]byte{byte(index + 1)}
		planned := (uint32(17) + uint32(seed[0]%16)) * 16_381
		cell := Cell{Direction: direction, ClientProcess: "client", PublisherProcess: "publisher",
			ClientApplicationProcess: "client-app", PublisherApplicationProcess: "publisher-app", InitialCarrier: "one",
			ReplacementCarrier: "two", FaultService: "rendezvous", FaultContainer: "container", FaultNetwork: "network",
			FaultNetworkAbsent: true, Seed: seed, Bytes: streamBytes, PlannedFaultOffset: planned,
			CellManifestDigest: cellManifestDigest(direction, seed, planned), FaultOffset: planned, DeliveredBeforeFault: planned,
			CanaryOffset: planned + 32, LastDeliveryNanos: 1, FaultAtNanos: 2,
			CanaryAtNanos: int64(time.Second), TerminalAtNanos: int64(2 * time.Second), ClientRouteGeneration: 2,
			PublisherRouteGeneration: 2, ClientRecoveryCount: 1, PublisherRecoveryCount: 1,
			ClientApplicationAccepts: 1, PublisherApplicationAccepts: 1, ClientRouteAccepts: 2, PublisherRouteAccepts: 2,
			ClientContinuity: [32]byte{9}, PublisherContinuity: [32]byte{9}, Ordered: true, Unique: true,
			SameConnection: true, FailedResourceUnavailable: true, TerminalClean: true, QueueHighWater: queueLimit}
		cell.MemoryHighWater, cell.OpenFilesHighWater, cell.GoroutinesHighWater, cell.TimerHighWater = 1, 1, 1, 1
		cell.ExternalStatsObserved = true
		cell.CarrierForwardBytes, cell.CarrierReverseBytes = 1, 1
		cell.BaselineClientTraffic, cell.BaselinePublisherTraffic = 1, 1
		cell.ResourceSamples = []ResourceSample{
			{AtNanos: 1, ClientRSS: 1, PublisherRSS: 1, ClientCPUPercent: 1, PublisherCPUPercent: 1,
				ClientReceived: 1, ClientSent: 1, PublisherReceived: 1, PublisherSent: 1},
			{AtNanos: int64(time.Second), ClientRSS: 1, PublisherRSS: 1, ClientCPUPercent: 1, PublisherCPUPercent: 1,
				ClientReceived: 2, ClientSent: 2, PublisherReceived: 2, PublisherSent: 2},
			{AtNanos: int64(2 * time.Second), ClientRSS: 1, PublisherRSS: 1, ClientCPUPercent: 1, PublisherCPUPercent: 1,
				ClientReceived: 3, ClientSent: 3, PublisherReceived: 3, PublisherSent: 3},
		}
		cell.ExpectedDigest, cell.ObservedDigest = workloadDigest(seed, streamBytes), workloadDigest(seed, streamBytes)
		cell.Canary = workloadRange(seed, cell.CanaryOffset)
		value.Cells = append(value.Cells, cell)
	}
	return value
}

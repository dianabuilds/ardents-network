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
		"fault setup moved recovery clock": func(value *Evidence) {
			cell := &value.Cells[0]
			cell.CarrierObservedNanos, cell.FaultAtNanos = int64(6*time.Second), int64(6*time.Second)+1
			cell.FaultCompletedNanos, cell.CanaryAtNanos = int64(6*time.Second)+3, int64(6*time.Second)+4
			cell.ReplacementObservedNanos, cell.TerminalAtNanos = int64(6*time.Second)+5, int64(7*time.Second)
			cell.OldCarrierRetiredNanos = int64(6*time.Second) + 2
		},
		"fault setup moved terminal clock": func(value *Evidence) {
			cell := &value.Cells[0]
			cell.FaultAtNanos, cell.FaultCompletedNanos = int64(4*time.Second), int64(4*time.Second)+2
			cell.CanaryAtNanos, cell.ReplacementObservedNanos = int64(5*time.Second), int64(5*time.Second)+1
			cell.TerminalAtNanos = int64(16 * time.Second)
			cell.OldCarrierRetiredNanos = int64(4*time.Second) + 1
		},
		"wrong Carrier Attachment deadline": func(value *Evidence) {
			value.Cells[0].CarrierAttachmentDeadlineNanos = int64(9 * time.Second)
		},
		"wrong workload chunk delay": func(value *Evidence) {
			value.Cells[0].ChunkDelayNanos = int64(20 * time.Millisecond)
		},
		"old Carrier not retired": func(value *Evidence) {
			value.Cells[0].OldCarrierRetired = false
		},
		"old Carrier retired after restoration": func(value *Evidence) {
			value.Cells[0].OldCarrierRetiredNanos = value.Cells[0].FaultCompletedNanos + 1
		},
		"secret cleanup":      func(value *Evidence) { value.Cleanup.PrivateMaterialAbsent = false },
		"fault mismatch":      func(value *Evidence) { value.Cells[0].FaultedCarrier = strings.Repeat("3", 64) },
		"retirement mismatch": func(value *Evidence) { value.Cells[0].RetiredCarrier = strings.Repeat("3", 64) },
		"route pid changed": func(value *Evidence) {
			value.Cells[0].RecoveredRoutePIDs["rendezvous"]++
		},
		"wrong fault network": func(value *Evidence) { value.Cells[0].FaultNetwork = "campaign_route_net" },
		"malformed observer":  func(value *Evidence) { value.Cells[0].ReplacementObserver.ContainerID = "observer" },
		"reused observer": func(value *Evidence) {
			value.Cells[1].ReplacementObserver = value.Cells[0].ReplacementObserver
		},
		"controller not removed": func(value *Evidence) {
			value.Cells[0].FaultControllerRemoved = false
		},
		"controller endpoint overlap": func(value *Evidence) {
			value.Cells[0].ClientProcess = value.Cells[0].FaultController
		},
		"reused removed controller": func(value *Evidence) {
			value.Cells[1].FaultController = value.Cells[0].FaultController
		},
		"observer privilege widened": func(value *Evidence) {
			value.Cells[0].ReplacementObserver.CapDrop = nil
		},
		"observer privileged": func(value *Evidence) {
			value.Cells[0].ReplacementObserver.Privileged = true
		},
		"observer capability added": func(value *Evidence) {
			value.Cells[0].ReplacementObserver.CapAdd = []string{"SYS_ADMIN"}
		},
		"observer host pid": func(value *Evidence) {
			value.Cells[0].ReplacementObserver.PIDMode = "host"
		},
		"observer host ipc": func(value *Evidence) {
			value.Cells[0].ReplacementObserver.IPCMode = "host"
		},
		"observer endpoint overlap": func(value *Evidence) {
			value.Cells[0].ClientProcess = value.Cells[0].ReplacementObserver.ContainerID
		},
		"controller authority widened": func(value *Evidence) {
			value.Topology = []byte(strings.Replace(string(value.Topology), "cap_add: [NET_ADMIN]", "cap_add: [NET_ADMIN, SYS_ADMIN]", 1))
			value.TopologyDigest = hexDigest(value.Topology)
		},
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
		"  client:\n    networks: [route_net]\n    restart: \"no\"\n  publisher:\n    networks: [route_net]\n    restart: \"no\"\n" +
		"  initiator:\n    networks: [route_net]\n    restart: \"no\"\n  introduction:\n    networks: [route_net]\n    restart: \"no\"\n" +
		"  rendezvous:\n    networks: [route_net, carrier_net]\n    ipv4_address: 172.31.21.13\n    restart: \"no\"\n" +
		"  responder:\n    networks: [route_net, carrier_net]\n    ipv4_address: 172.31.21.14\n    restart: \"no\"\n" +
		"  carrier-fault:\n    command: [ardents-qualify, carrier-fault, wait]\n    network_mode: service:rendezvous\n" +
		"    cap_add: [NET_ADMIN]\n    cap_drop: [ALL]\n    security_opt: [no-new-privileges:true]\n" +
		"    read_only: true\n    restart: \"no\"\n    user: \"0:0\"\n    cpus: 0.25\n    mem_limit: \"33554432\"\n    pids_limit: 16\n" +
		"networks:\n  route_net:\n    internal: true\n  carrier_net:\n    internal: true\n")
	value := Evidence{Schema: schema, SourceCommit: "0123456789012345678901234567890123456789", ImageID: "sha256:image", VerifierImageID: "sha256:image",
		Topology: topology, TopologyDigest: hexDigest(topology), Manifest: manifest,
		ManifestDigest: hex.EncodeToString(manifestDigest[:]), Claim: "S4.1 local development evidence only",
		Target: target, Instance: [32]byte{2}, NetworkID: [32]byte{3}, CandidateView: [32]byte{4},
		AuthorityPublic: authority,
		ClientPrincipal: [32]byte{8}, PublisherPrincipal: [32]byte{9}, RouteProfile: "profile",
		CredentialGeneration: 1, CredentialNotBefore: 1, CredentialNotAfter: 100, WorkSafetyNotAfter: 90,
		WorkSafetyMaximum: 100, NoNewRecoveryAfter: 80,
		BinaryDigests: map[string]string{"ardents-route": strings.Repeat("a", 64), "ardents-service": strings.Repeat("b", 64),
			"ardents-qualify": strings.Repeat("e", 64), "ardents-stream-app": strings.Repeat("c", 64),
			"ardents-recovery-qualify": strings.Repeat("d", 64)},
		RequestedNanos: int64(10 * time.Minute), CampaignNanos: int64(10 * time.Minute),
		Negatives: map[string]Negative{}, Cleanup: cleanup{DockerEmpty: true, FixtureAbsent: true, PrivateMaterialAbsent: true}}
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
		planned := (uint32(184) + uint32(seed[0]%8)) * 16_381
		cell := Cell{Direction: direction, ClientProcess: "client", PublisherProcess: "publisher",
			ClientApplicationProcess: "client-app", PublisherApplicationProcess: "publisher-app", InitialCarrier: strings.Repeat("1", 64),
			ReplacementCarrier: strings.Repeat("2", 64), FaultService: "rendezvous-responder-carrier", FaultContainer: "rendezvous-container",
			InitialCarrierLocal: "172.31.21.13:50001", InitialCarrierRemote: "172.31.21.14:4604",
			ReplacementCarrierLocal: "172.31.21.13:50002", ReplacementCarrierRemote: "172.31.21.14:4604",
			FaultedCarrier: strings.Repeat("1", 64), RetiredCarrier: strings.Repeat("1", 64),
			InitialCarrierInode: 1, ReplacementCarrierInode: 2,
			InitialCarrierInterface: "eth1", ReplacementCarrierInterface: "eth1",
			InitialCarrierInterfaceIndex: 3, ReplacementCarrierInterfaceIndex: 4,
			FaultNetwork: "ardents-recovery-test_carrier_net", FaultController: strings.Repeat(string(rune('e'+index)), 64),
			FaultControllerRemoved: true, FaultResourceAbsent: true,
			InitialRouteContainers: map[string]string{}, RecoveredRouteContainers: map[string]string{},
			InitialRoutePIDs: map[string]uint32{}, RecoveredRoutePIDs: map[string]uint32{},
			Seed: seed, Bytes: streamBytes, PlannedFaultOffset: planned,
			CellManifestDigest: cellManifestDigest(direction, seed, planned), FaultOffset: planned, DeliveredBeforeFault: planned,
			CanaryOffset: planned + 32, LastDeliveryNanos: 1, CarrierObservedNanos: 2, FaultAtNanos: 3,
			FaultCompletedNanos: 10, CarrierCutAfterNanos: 1, AbsenceAfterNanos: 2,
			CarrierAttachmentDeadlineNanos: int64(10 * time.Second), ChunkDelayNanos: int64(30 * time.Millisecond),
			OldCarrierRetiredNanos:   8,
			CanaryAtNanos:            int64(time.Second),
			ReplacementObservedNanos: int64(time.Second) + 1, TerminalAtNanos: int64(2 * time.Second), ClientRouteGeneration: 2,
			PublisherRouteGeneration: 2, ClientRecoveryCount: 1, PublisherRecoveryCount: 1,
			ClientApplicationAccepts: 1, PublisherApplicationAccepts: 1, ClientRouteAccepts: 2, PublisherRouteAccepts: 2,
			ClientContinuity: [32]byte{9}, PublisherContinuity: [32]byte{9}, Ordered: true, Unique: true,
			SameConnection: true, OldCarrierRetired: true, FailedResourceUnavailable: true, TerminalClean: true, QueueHighWater: queueLimit}
		observerID := strings.Repeat(string(rune('a'+index)), 64)
		cell.ReplacementObserver = ObserverProcess{ContainerID: observerID, ImageID: value.ImageID,
			NetworkMode: "container:rendezvous-container", User: "65532:65532", IPCMode: "private",
			Command: []string{"/usr/local/bin/ardents-qualify", "carrier-fault", "observe"},
			CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges"}, ReadOnly: true, Removed: true,
			PidsLimit: 16, MemoryLimit: 32 << 20, NanoCPUs: 250_000_000}
		cell.MemoryHighWater, cell.OpenFilesHighWater, cell.GoroutinesHighWater, cell.TimerHighWater = 1, 1, 1, 1
		for roleIndex, role := range []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"} {
			cell.InitialRouteContainers[role], cell.RecoveredRouteContainers[role] = role+"-container", role+"-container"
			cell.InitialRoutePIDs[role], cell.RecoveredRoutePIDs[role] = uint32(roleIndex+1), uint32(roleIndex+1)
		}
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

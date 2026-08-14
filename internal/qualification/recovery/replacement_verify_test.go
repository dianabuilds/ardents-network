package recovery

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestVerifyAcceptsCompleteS42ReplacementEvidence(t *testing.T) {
	value := validS42Evidence(t)
	if result := Verify(value); result.Verdict != "pass" {
		t.Fatalf("complete S4.2 evidence rejected: %+v", result)
	}
}

func TestVerifyRejectsS42ReplacementMutations(t *testing.T) {
	for name, mutate := range map[string]func(*replacementEvidence){
		"missing isolated role": func(value *replacementEvidence) { value.Cells = value.Cells[1:] },
		"leg changed rendezvous": func(value *replacementEvidence) {
			value.Cells[0].Events[0].RendezvousAfter = value.Cells[0].Events[0].Replacement
		},
		"rendezvous reused": func(value *replacementEvidence) {
			value.Cells[2].Events[0].RendezvousAfter = value.Cells[2].Events[0].RendezvousBefore
		},
		"late canary": func(value *replacementEvidence) { value.Cells[0].Events[0].CanaryNanos += int64(6 * time.Second) },
		"failed candidate available": func(value *replacementEvidence) {
			value.Cells[0].Events[0].FailedResource.Running = true
		},
		"missing fresh Introduction": func(value *replacementEvidence) {
			value.Cells[2].Events[0].IntroductionSetupReceipt = [32]byte{}
		},
		"sealed setup changed Rendezvous": func(value *replacementEvidence) {
			value.Cells[2].Proposals[2].IntroductionProof.RendezvousNode[0]++
		},
		"sealed setup changed reachability": func(value *replacementEvidence) {
			value.Cells[2].Proposals[2].IntroductionProof.RendezvousReachability[0]++
		},
		"sealed setup changed expiry": func(value *replacementEvidence) {
			value.Cells[2].Proposals[2].IntroductionProof.ExpiresAtNanos++
		},
		"sealed setup changed receipt": func(value *replacementEvidence) {
			value.Cells[2].Proposals[2].IntroductionReceipt[0]++
		},
		"opaque relay observation missing": func(value *replacementEvidence) {
			value.Cells[2].Events[0].IntroductionOpaqueBytes = 0
		},
		"proposal process identity is short": func(value *replacementEvidence) {
			value.Cells[0].Proposals[0].Processes[0].ContainerID = "short"
		},
		"missing common process commitment": func(value *replacementEvidence) {
			process := value.Cells[0].Routes[0].Processes["initiator"]
			process.Host.Commitment = [32]byte{}
			value.Cells[0].Routes[0].Processes["initiator"] = process
		},
		"changed process observation time": func(value *replacementEvidence) {
			process := value.Cells[0].Routes[0].Processes["initiator"]
			process.ObservedAtNanos++
			value.Cells[0].Routes[0].Processes["initiator"] = process
		},
		"process scope changed outside campaign": func(value *replacementEvidence) {
			process := value.Cells[0].Routes[0].Processes["initiator"]
			process.Host.Scope[0]++
			process.Host.Commitment = processRefCommitment(process.Host)
			value.Cells[0].Routes[0].Processes["initiator"] = process
		},
		"host machine scope changed": func(value *replacementEvidence) {
			value.HostScope.Machine[0]++
			value.HostScope.Commitment = hostScopeCommitment(value.HostScope)
		},
		"mixed process adapter": func(value *replacementEvidence) {
			process := value.Cells[0].Routes[0].Processes["initiator"]
			process.Host.Adapter = "native-test-v1"
			process.Host.Commitment = processRefCommitment(process.Host)
			process.HostObservation = processObservationCommitment(process.Host,
				[]byte(process.AdapterProjection), process.PID, true, process.ObservedAtNanos)
			value.Cells[0].Routes[0].Processes["initiator"] = process
		},
		"Docker process projection changed": func(value *replacementEvidence) {
			process := value.Cells[0].Routes[0].Processes["initiator"]
			process.AdapterProjection = `{"Image":"wrong","Path":"/wrong","Project":"wrong","Service":"initiator"}`
			process.HostObservation = processObservationCommitment(process.Host,
				[]byte(process.AdapterProjection), process.PID, true, process.ObservedAtNanos)
			value.Cells[0].Routes[0].Processes["initiator"] = process
		},
		"Docker process projection is not canonical": func(value *replacementEvidence) {
			process := value.Cells[0].Routes[0].Processes["initiator"]
			comma := strings.IndexByte(process.AdapterProjection, ',')
			process.AdapterProjection = "{" + process.AdapterProjection[1:comma] + "," + process.AdapterProjection[1:]
			process.HostObservation = processObservationCommitment(process.Host,
				[]byte(process.AdapterProjection), process.PID, true, process.ObservedAtNanos)
			value.Cells[0].Routes[0].Processes["initiator"] = process
		},
		"host clock reset between cells": func(value *replacementEvidence) {
			value.Cells[1].HostStartedAtNanos = 1
		},
		"next cell overlaps prior host observation": func(value *replacementEvidence) {
			receipt := &value.Cells[0].Proposals[0].Stopped[0]
			receipt.State.ObservedAtNanos = value.Cells[1].HostStartedAtNanos + 1
			receipt.State.Commitment = processStateCommitment(receipt.State)
			receipt.ObservedAtNanos = receipt.State.ObservedAtNanos - value.Cells[0].HostStartedAtNanos
		},
		"process observed after its fault": func(value *replacementEvidence) {
			cell := &value.Cells[0]
			process := cell.Routes[0].Processes["initiator"]
			process.ObservedAtNanos = cell.Events[0].FailedResource.Fault.InvocationStartedNanos + 1
			process.HostObservation = processObservationCommitment(process.Host,
				[]byte(process.AdapterProjection), process.PID, true, process.ObservedAtNanos)
			cell.Routes[0].Processes["initiator"] = process
			cell.Events[0].Failed = process
			cell.Proposals[0].Processes[0] = process
		},
		"missing common fault receipt": func(value *replacementEvidence) {
			value.Cells[0].Events[0].FailedResource.Fault = processFaultEvidence{}
		},
		"missing common state receipt": func(value *replacementEvidence) {
			value.Cells[0].Events[0].FailedResource.State = processStateEvidence{}
		},
		"proposal process still running": func(value *replacementEvidence) {
			value.Cells[0].Proposals[0].Stopped[0].Running = true
		},
		"proposal stop predates terminal": func(value *replacementEvidence) {
			value.Cells[0].Proposals[0].Stopped[0].ObservedAtNanos = value.Cells[0].TerminalNanos - 1
		},
		"proposal process overlaps Endpoint": func(value *replacementEvidence) {
			value.Cells[0].Proposals[0].Processes[0].ContainerID = value.Cells[0].ClientProcess
			value.Cells[0].Proposals[0].Processes[0].Incarnation = value.Cells[0].ClientProcess + "@2026-01-01T00:00:00Z"
			value.Cells[0].Proposals[0].Stopped[0].ContainerID = value.Cells[0].ClientProcess
		},
		"committed proposal process substituted": func(value *replacementEvidence) {
			process := &value.Cells[0].Proposals[0].Processes[0]
			process.ContainerID = fmt.Sprintf("%064x", 99_001)
			process.Incarnation = process.ContainerID + "@2026-01-01T00:00:00Z"
			value.Cells[0].Proposals[0].Stopped[0].ContainerID = process.ContainerID
		},
		"abandoned proposal process reused": func(value *replacementEvidence) {
			reused := value.Cells[2].Proposals[1].Processes[1].ContainerID
			process := &value.Cells[2].Proposals[1].Processes[0]
			process.ContainerID = reused
			process.Incarnation = reused + "@2026-01-01T00:00:00Z"
			value.Cells[2].Proposals[1].Stopped[0].ContainerID = reused
		},
		"same proposed Node changed process": func(value *replacementEvidence) {
			value.Cells[4].Proposals[3].Processes[0].Incarnation =
				value.Cells[4].Proposals[3].Processes[0].ContainerID + "@2027-01-01T00:00:00Z"
		},
		"cooperative proposal sequence": func(value *replacementEvidence) {
			value.Cells[2].Proposals = append(value.Cells[2].Proposals[:1], value.Cells[2].Proposals[2:]...)
		},
		"missing final canary":  func(value *replacementEvidence) { value.Cells[0].FinalCanary = [32]byte{} },
		"unacknowledged range":  func(value *replacementEvidence) { value.Cells[0].ClientAcknowledgedBytes-- },
		"queue overflow":        func(value *replacementEvidence) { value.Cells[0].ClientQueueHighWater = 256<<10 + 1 },
		"Application reconnect": func(value *replacementEvidence) { value.Cells[0].ApplicationReconnected = true },
		"sequential failure count": func(value *replacementEvidence) {
			value.Cells[4].Events = value.Cells[4].Events[:2]
		},
		"sequential duration":          func(value *replacementEvidence) { value.Cells[4].TerminalNanos = int64(9 * time.Minute) },
		"candidate identity duplicate": func(value *replacementEvidence) { value.Candidates[1].NodeID = value.Candidates[0].NodeID },
	} {
		t.Run(name, func(t *testing.T) {
			value := validS42Evidence(t)
			extension := decodeReplacementTest(t, value.S42)
			mutate(&extension)
			var err error
			value.S42, err = json.Marshal(extension)
			if err != nil {
				t.Fatal(err)
			}
			if result := Verify(value); result.Verdict == "pass" {
				t.Fatalf("S4.2 mutation passed: %+v", result)
			}
		})
	}
}

func validS42Evidence(t *testing.T) Evidence {
	t.Helper()
	value := validEvidence()
	value.Claim = "S4.2 four-position local development tracer only; does not qualify split-leg/Introduction topology"
	value.RequestedNanos = int64(20 * time.Minute)
	value.CampaignNanos = int64(21 * time.Minute)
	value.Topology = s42Topology(value.Topology)
	value.TopologyDigest = hexDigest(value.Topology)
	seed := [32]byte{7}
	sets := s42CandidateSets(seed)
	extension := replacementEvidence{}
	for _, role := range replacementRoleNames {
		extension.Candidates = append(extension.Candidates, sets[role]...)
	}
	extension.RouteCase = s42RouteCase(value, seed, sets)
	routeDigest, err := commitRouteCase(extension.RouteCase)
	if err != nil {
		t.Fatal(err)
	}
	value.CandidateView, value.Manifest.RouteManifest = routeDigest, routeDigest
	manifestDigest := publicManifestDigest(value.Manifest)
	value.ManifestDigest = fmt.Sprintf("%x", manifestDigest)
	value.IsolationContext = sha256.Sum256(append([]byte("isolation\x00"), manifestDigest[:]...))
	serial := 1000
	hostScope := testHostScope(value.SourceCommit, value.ImageID, value.ManifestDigest)
	extension.HostScope = hostScope
	nextID := func() string { serial++; return fmt.Sprintf("%064x", serial) }
	hostStartedAt := int64(100)
	for _, direction := range []string{"client-to-publisher", "publisher-to-client"} {
		for _, role := range replacementRoleNames {
			cell := s42Cell(value.ImageID, direction, "isolated-"+role, []string{role}, sets, seed,
				extension.RouteCase, routeDigest, hostScope, hostStartedAt, nextID, t)
			extension.Cells = append(extension.Cells, cell)
			hostStartedAt += cell.TerminalNanos + 10
		}
		cell := s42Cell(value.ImageID, direction, "sequential-three",
			[]string{"initiator", "rendezvous", "responder"}, sets, seed, extension.RouteCase, routeDigest,
			hostScope, hostStartedAt, nextID, t)
		extension.Cells = append(extension.Cells, cell)
		hostStartedAt += cell.TerminalNanos + 10
	}
	value.S42, err = json.Marshal(extension)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func s42RouteCase(value Evidence, seed [32]byte, sets map[string][]replacementCandidate) routeCase {
	result := routeCase{NetworkID: value.NetworkID, Generation: "generation-1", Epoch: 1,
		EpochDigest: [32]byte{51}, Profile: "h3-route-tracer-v1", ViewRoot: [32]byte{52},
		SelectionSeed: seed, SelectionAt: 1, ClientPin: [32]byte{53}, PublisherID: [32]byte{54}}
	for index, role := range replacementRoleNames {
		selected := sets[role][0]
		result.NodeIDs[index], result.PublicKeys[index], result.Families[index], result.Endpoints[index] =
			selected.NodeID, selected.PublicKey, selected.Family, selected.Endpoint
		for _, candidate := range sets[role] {
			result.Candidates = append(result.Candidates, routeCandidate{NodeID: candidate.NodeID,
				PublicKey: candidate.PublicKey, Family: candidate.Family, Endpoint: candidate.Endpoint,
				Domain: candidate.Role, Capacity: 1, ValidFrom: 1, ValidUntil: 100})
		}
	}
	return result
}

var replacementRoleNames = []string{"initiator", "introduction", "rendezvous", "responder"}

func s42CandidateSets(seed [32]byte) map[string][]replacementCandidate {
	result := make(map[string][]replacementCandidate, 4)
	for roleIndex, role := range replacementRoleNames {
		for candidate := range 3 {
			result[role] = append(result[role], replacementCandidate{Role: role,
				Family:   fmt.Sprintf("%s-family-%d", role, candidate),
				Endpoint: fmt.Sprintf("172.31.20.%d:%d", 11+roleIndex+candidate*10, 4601+roleIndex),
				NodeID:   [32]byte{byte(1 + roleIndex*3 + candidate)}, PublicKey: [32]byte{byte(31 + roleIndex*3 + candidate)}})
		}
		sort.Slice(result[role], func(left, right int) bool {
			return testCandidateRank(seed, role, result[role][left].NodeID) <
				testCandidateRank(seed, role, result[role][right].NodeID)
		})
	}
	return result
}

func testCandidateRank(seed [32]byte, role string, node [32]byte) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-h3-route-select-v1\x00"))
	_, _ = hash.Write(seed[:])
	_, _ = hash.Write([]byte(role))
	_, _ = hash.Write(node[:])
	return string(hash.Sum(nil))
}

func s42Cell(imageID, direction, mode string, failures []string, sets map[string][]replacementCandidate,
	seed [32]byte, routeCase routeCase, routeManifest [32]byte, hostScope hostScopeEvidence,
	hostStartedAt int64, nextID func() string, t *testing.T) replacementCell {
	proposalIndexes := [][4]int{{0, 0, 0, 0}, {1, 1, 0, 1}, {2, 2, 2, 2}, {1, 1, 2, 1}}
	proposals := []int{0, 1}
	if mode == "isolated-rendezvous" {
		proposals = []int{0, 2}
	} else if mode == "sequential-three" {
		proposals = []int{0, 1, 2, 3}
	}
	selections := make([]map[string]replacementCandidate, 0, len(proposals))
	for _, proposal := range proposals {
		selection := make(map[string]replacementCandidate, len(replacementRoleNames))
		for roleIndex, role := range replacementRoleNames {
			selection[role] = sets[role][proposalIndexes[proposal][roleIndex]]
		}
		selections = append(selections, selection)
	}
	processes := map[[32]byte]candidateProcess{}
	cell := replacementCell{Direction: direction, Mode: mode, Seed: seed, Bytes: streamBytes,
		ExpectedDigest: workloadDigest(seed, streamBytes), ObservedDigest: workloadDigest(seed, streamBytes),
		ClientProcess: nextID(), PublisherProcess: nextID(),
		ClientApplicationProcess: nextID(), PublisherApplicationProcess: nextID(),
		ClientRouteGeneration: uint64(len(selections)), PublisherRouteGeneration: uint64(len(selections)),
		ClientRecoveryCount: uint32(len(failures)), PublisherRecoveryCount: uint32(len(failures)),
		ClientApplicationAccepts: 1, PublisherApplicationAccepts: 1,
		ClientRouteAccepts: uint32(len(selections)), PublisherRouteAccepts: uint32(len(selections)),
		ClientContinuity: [32]byte{9}, PublisherContinuity: [32]byte{9}, Ordered: true, Unique: true,
		SameConnection: true, TerminalClean: true, ClientQueueHighWater: 1, PublisherQueueHighWater: 1}
	cell.HostStartedAtNanos = hostStartedAt
	cell.TerminalNanos = int64(6 * time.Second)
	if mode == "sequential-three" {
		cell.TerminalNanos = int64(10*time.Minute + time.Second)
	}
	if direction == "client-to-publisher" {
		cell.ClientAcceptedBytes, cell.ClientAcknowledgedBytes = streamBytes, streamBytes
		cell.PublisherReceivedBytes = streamBytes
	} else {
		cell.PublisherAcceptedBytes, cell.PublisherAcknowledgedBytes = streamBytes, streamBytes
		cell.ClientReceivedBytes = streamBytes
	}
	cell.FinalCanaryOffset = streamBytes - 32
	cell.FinalCanary = workloadRange(seed, cell.FinalCanaryOffset)
	cell.FinalCanaryObservedNanos = cell.TerminalNanos + 1
	for generation, selection := range selections {
		routeGeneration := routeGeneration{Generation: uint64(generation + 1), Processes: map[string]candidateProcess{}}
		for _, role := range replacementRoleNames {
			candidate := selection[role]
			process, ok := processes[candidate.NodeID]
			if !ok {
				container := nextID()
				incarnation := container + "@2026-08-14T08:04:05Z"
				process = testObservedProcess(t, candidateProcess{Service: role, ContainerID: container,
					Incarnation: incarnation, PID: uint32(10 + len(processes)), ObservedAtNanos: hostStartedAt + 1,
					NodeID: candidate.NodeID, PublicKey: candidate.PublicKey,
					Host: testProcessRef(hostScope, container, incarnation)}, hostScope, imageID)
				processes[candidate.NodeID] = process
			}
			routeGeneration.Processes[role] = process
		}
		cell.Routes = append(cell.Routes, routeGeneration)
	}
	proposalCount := 2
	if mode == "isolated-rendezvous" {
		proposalCount = 3
	} else if mode == "sequential-three" {
		proposalCount = 4
	}
	for proposalIndex := range proposalCount {
		proposal := replacementProposal{Attachment: uint32(proposalIndex + 1), Terminal: "success", Committed: true}
		if mode == "isolated-rendezvous" && proposalIndex == 1 {
			proposal.Terminal, proposal.Committed = "error", false
		}
		if proposalIndex == 2 {
			selected := map[string]replacementCandidate{}
			for roleIndex, role := range replacementRoleNames {
				selected[role] = sets[role][proposalIndexes[proposalIndex][roleIndex]]
			}
			proposal.IntroductionProof, proposal.IntroductionReceipt = s42IntroductionProof(routeCase,
				routeManifest, selected)
		}
		for roleIndex, role := range replacementRoleNames {
			candidate := sets[role][proposalIndexes[proposalIndex][roleIndex]]
			proposal.NodeIDs[roleIndex], proposal.PublicKeys[roleIndex] = candidate.NodeID, candidate.PublicKey
			process, ok := processes[candidate.NodeID]
			if !ok {
				container := nextID()
				incarnation := container + "@2026-08-14T08:04:05Z"
				process = testObservedProcess(t, candidateProcess{Service: role, ContainerID: container,
					Incarnation: incarnation, PID: uint32(10 + len(processes)), ObservedAtNanos: hostStartedAt + 1,
					NodeID: candidate.NodeID, PublicKey: candidate.PublicKey,
					Host: testProcessRef(hostScope, container, incarnation)}, hostScope, imageID)
				processes[candidate.NodeID] = process
			}
			proposal.Processes[roleIndex] = process
			proposal.Stopped[roleIndex] = testStoppedReceipt(process, cell.HostStartedAtNanos,
				cell.TerminalNanos+1, 0)
			count := 0
			switch proposalIndex {
			case 1:
				if role != "rendezvous" {
					count = 1
				}
			case 2:
				count = 2
			case 3:
				count = 1
				if role == "rendezvous" {
					count = 2
				}
			}
			for candidateIndex := range count {
				proposal.ExcludedIdentities = append(proposal.ExcludedIdentities, sets[role][candidateIndex].NodeID)
			}
		}
		cell.Proposals = append(cell.Proposals, proposal)
	}
	for index, failure := range failures {
		before, after := cell.Routes[index], cell.Routes[index+1]
		introductionAttachment := uint32(0)
		for generation := 0; generation <= index+1; generation++ {
			if cell.Routes[generation].Processes["introduction"] == after.Processes["introduction"] {
				introductionAttachment++
			}
		}
		offset := uint32(17 * 16_381)
		last := int64(time.Second)
		if mode == "sequential-three" {
			offset = uint32((index + 1) * 64 * 16_381)
			last = int64((index+1)*2) * int64(time.Minute)
		}
		cell.Events = append(cell.Events, replacementEvent{Role: failure, Layer: "leg",
			GenerationBefore: before.Generation, GenerationAfter: after.Generation,
			Failed: before.Processes[failure], Replacement: after.Processes[failure],
			RendezvousBefore: before.Processes["rendezvous"], RendezvousAfter: after.Processes["rendezvous"],
			Introduction: after.Processes["introduction"], IntroductionAttachment: introductionAttachment,
			FaultOffset: offset, CanaryOffset: offset, Canary: workloadRange(seed, offset),
			LastDeliveryNanos: last, FaultAtNanos: last + 1, CanaryNanos: last + int64(time.Second),
			FailedResource: testStoppedReceipt(before.Processes[failure], cell.HostStartedAtNanos,
				cell.TerminalNanos+1, last+1)})
		if failure == "rendezvous" {
			cell.Events[index].Layer = "rendezvous"
			cell.Events[index].IntroductionSetupAttachment = 3
			cell.Events[index].IntroductionSetupReceipt = cell.Proposals[2].IntroductionReceipt
			cell.Events[index].IntroductionOpaqueBytes = 1
			cell.Events[index].IntroductionOpaqueDigest = [32]byte{78}
		}
	}
	cell.ResourceSamples = s42Samples(cell.TerminalNanos)
	last := cell.ResourceSamples[len(cell.ResourceSamples)-1]
	cell.FinalTraffic = last
	cell.FinalTraffic.AtNanos = cell.TerminalNanos + 1
	cell.BaselineFinalTraffic = ResourceSample{AtNanos: 2, ClientReceived: 1, ClientSent: 1,
		PublisherReceived: 1, PublisherSent: 1}
	cell.BaselineTerminalNanos = 1
	cell.BaselineClientTraffic, cell.BaselinePublisherTraffic = 2, 2
	cell.BaselineClientRoute, cell.BaselinePublisherRoute = nextID(), nextID()
	cell.BaselineClientTrafficObserver = s42Observer(imageID, nextID(), cell.BaselineClientRoute)
	cell.BaselinePublisherTrafficObserver = s42Observer(imageID, nextID(), cell.BaselinePublisherRoute)
	cell.ClientRoute, cell.PublisherRoute = nextID(), nextID()
	cell.ClientTrafficObserver = s42Observer(imageID, nextID(), cell.ClientRoute)
	cell.PublisherTrafficObserver = s42Observer(imageID, nextID(), cell.PublisherRoute)
	return cell
}

func s42IntroductionProof(value routeCase, manifest [32]byte,
	selected map[string]replacementCandidate) (introductionProof, [32]byte) {
	proof := introductionProof{ManifestDigest: manifest, NetworkID: value.NetworkID, EpochDigest: value.EpochDigest,
		ViewRoot: value.ViewRoot, ProfileDigest: sha256.Sum256([]byte(value.Profile)),
		CapabilitiesDigest: sha256.Sum256([]byte("ardents-h3-recovery-setup-capabilities-v1\x00tls13|single-use|no-application-data")),
		IntroductionNode:   selected["introduction"].NodeID, RendezvousNode: selected["rendezvous"].NodeID,
		RendezvousReachability: sha256.Sum256(append([]byte("ardents-h3-rendezvous-reachability-v1\x00"),
			selected["rendezvous"].Endpoint...)), JoinHandle: [32]byte{70}, EndpointHandshake: [32]byte{71},
		Reply: [32]byte{72}, ExpiresAtNanos: time.Unix(value.SelectionAt, 0).Add(time.Hour).UnixNano()}
	body := make([]byte, 397)
	copy(body[:5], "ASIS\x02")
	fields := [][32]byte{proof.ManifestDigest, proof.NetworkID, proof.EpochDigest, proof.ViewRoot, proof.ProfileDigest,
		proof.CapabilitiesDigest, proof.IntroductionNode, proof.RendezvousNode, proof.RendezvousReachability,
		proof.JoinHandle, proof.EndpointHandshake}
	for index, field := range fields {
		copy(body[5+index*32:5+(index+1)*32], field[:])
	}
	binary.BigEndian.PutUint64(body[357:365], uint64(proof.ExpiresAtNanos))
	proof.TranscriptContext = sha256.Sum256(append([]byte("ardents-h3-sealed-introduction-transcript-v2\x00"), body[:365]...))
	copy(body[365:], proof.TranscriptContext[:])
	receipt := append([]byte("ardents-h3-sealed-introduction-v2\x00"), body...)
	receipt = append(receipt, proof.Reply[:]...)
	return proof, sha256.Sum256(receipt)
}

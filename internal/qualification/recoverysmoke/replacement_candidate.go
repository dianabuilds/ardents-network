package recoverysmoke

import (
	"sort"
	"strings"
	"time"
)

const (
	replacementRSSLimit       = uint64(512 << 20)
	replacementTrafficLimit   = uint64(8 << 20)
	replacementBitrateLimit   = float64(25_000_000)
	replacementQueueHighWater = 256 << 10
)

func replacementCandidateResult(cell replacementCell) (string, string) {
	failed := "replacement candidate violated the cell contract"
	events, routes := len(cell.Events), len(cell.Routes)
	if cell.ObservedDigest != cell.ExpectedDigest || routes != events+1 ||
		cell.ClientRouteGeneration != uint64(routes) || cell.PublisherRouteGeneration != uint64(routes) ||
		cell.ClientRecoveryCount != uint32(events) || cell.PublisherRecoveryCount != uint32(events) ||
		cell.ClientApplicationAccepts != 1 || cell.PublisherApplicationAccepts != 1 ||
		cell.ClientRouteAccepts != uint32(routes) || cell.PublisherRouteAccepts != uint32(routes) ||
		!cell.Ordered || !cell.Unique || !cell.SameConnection || cell.ApplicationReconnected || !cell.TerminalClean {
		return "fail", failed
	}
	clientSend, clientReceive, publisherSend, publisherReceive := cell.Bytes, uint32(0), uint32(0), cell.Bytes
	if cell.Direction == "publisher-to-client" {
		clientSend, clientReceive, publisherSend, publisherReceive = 0, cell.Bytes, cell.Bytes, 0
	}
	if cell.ClientAcceptedBytes != clientSend || cell.ClientAcknowledgedBytes != clientSend ||
		cell.ClientReceivedBytes != clientReceive || cell.PublisherAcceptedBytes != publisherSend ||
		cell.PublisherAcknowledgedBytes != publisherSend || cell.PublisherReceivedBytes != publisherReceive ||
		cell.ClientQueueHighWater > replacementQueueHighWater ||
		cell.PublisherQueueHighWater > replacementQueueHighWater ||
		clientSend > 0 && cell.ClientQueueHighWater == 0 || publisherSend > 0 && cell.PublisherQueueHighWater == 0 {
		return "fail", failed
	}
	if cell.FinalCanaryOffset != cell.Bytes-32 || cell.FinalCanary != workloadCanary(cell.Seed, cell.FinalCanaryOffset) ||
		cell.FinalCanaryObservedNanos < cell.TerminalNanos ||
		cell.FinalCanaryObservedNanos-cell.TerminalNanos > int64(30*time.Second) {
		return "fail", failed
	}
	for index, event := range cell.Events {
		if !replacementCandidateEvent(cell, index, event) {
			return "fail", failed
		}
	}
	for index, proposal := range cell.Proposals {
		committed := cell.Mode != "isolated-rendezvous" && cell.Mode != "overlap" || index != 1
		terminal := "success"
		if cell.Mode == "overlap" && index == 1 {
			terminal = "error"
		}
		if proposal.Committed != committed || proposal.Terminal != terminal {
			return "fail", failed
		}
	}
	if !replacementCandidateResources(cell) {
		return "fail", failed
	}
	if cell.Mode == "overlap" {
		if events != 1 || cell.TerminalNanos > int64(time.Minute) || !candidateOverlapEvidence(cell) {
			return "fail", failed
		}
	} else if cell.Mode == "sequential-three" {
		if events != 3 || cell.TerminalNanos < int64(10*time.Minute) || cell.TerminalNanos > int64(13*time.Minute) {
			return "fail", failed
		}
	} else if !strings.HasPrefix(cell.Mode, "isolated-") || events != 1 ||
		cell.Mode != "isolated-"+cell.Events[0].Role || cell.TerminalNanos > int64(time.Minute) {
		return "fail", failed
	}
	return "pass", "replacement candidate satisfied the cell contract"
}

func candidateOverlapEvidence(cell replacementCell) bool {
	value := cell.Overlap
	event := cell.Events[0]
	return value.SocketDigest != "" && value.LocalAddress != "" && value.RemoteAddress == "172.31.20.16:4606" &&
		value.InterfaceName != "" && value.Inode != 0 && value.InterfaceIndex > 0 && value.ObservedAtNanos > event.FaultAtNanos &&
		value.FaultAtNanos >= value.ObservedAtNanos && value.FaultAtNanos-event.FaultAtNanos <= int64(time.Second) &&
		value.FaultCompletedNanos >= value.FaultAtNanos && value.CarrierCutAfterNanos > 0 &&
		value.AbsenceAfterNanos >= value.CarrierCutAfterNanos && value.Absent && value.ObserverRemoved &&
		event.CanaryNanos-event.LastDeliveryNanos <= int64(8*time.Second)
}

func replacementCandidateEvent(cell replacementCell, index int, event replacementEvent) bool {
	recoveryLimit := int64(5 * time.Second)
	if cell.Mode == "overlap" {
		recoveryLimit = int64(8 * time.Second)
	}
	if index >= len(cell.FaultOffsets) || event.FaultOffset != cell.FaultOffsets[index] ||
		event.CanaryOffset != event.FaultOffset || event.Canary != workloadCanary(cell.Seed, event.CanaryOffset) ||
		event.LastDeliveryNanos <= 0 || event.FaultAtNanos < event.LastDeliveryNanos ||
		event.CanaryNanos <= event.FaultAtNanos ||
		event.CanaryNanos-event.LastDeliveryNanos > recoveryLimit || event.CanaryNanos > cell.TerminalNanos {
		return false
	}
	before, after := cell.Routes[index], cell.Routes[index+1]
	if cell.Mode == "overlap" {
		return event.Layer == "overlap" && !sameProcessIncarnation(event.RendezvousBefore, event.RendezvousAfter) &&
			event.IntroductionSetupReceipt != [32]byte{} && event.IntroductionSetupAttachment == 3 &&
			event.IntroductionOpaqueBytes > 0 && event.IntroductionOpaqueDigest != [32]byte{} &&
			event.CanaryNanos-event.LastDeliveryNanos <= int64(8*time.Second)
	}
	if event.Role == "rendezvous" {
		return event.Layer == "rendezvous" &&
			!sameProcessIncarnation(event.RendezvousBefore, event.RendezvousAfter) &&
			event.IntroductionSetupReceipt != [32]byte{} && event.IntroductionSetupAttachment == 3 &&
			event.IntroductionOpaqueBytes > 0 && event.IntroductionOpaqueDigest != [32]byte{}
	}
	return event.Layer == "leg" && sameProcessIncarnation(event.RendezvousBefore, before.Processes["rendezvous"]) &&
		sameProcessIncarnation(event.RendezvousAfter, after.Processes["rendezvous"])
}

func replacementCandidateResources(cell replacementCell) bool {
	if len(cell.ResourceSamples) < 3 {
		return false
	}
	clientRSS, publisherRSS := make([]uint64, 0, len(cell.ResourceSamples)), make([]uint64, 0, len(cell.ResourceSamples))
	clientCPU, publisherCPU := make([]float64, 0, len(cell.ResourceSamples)), make([]float64, 0, len(cell.ResourceSamples))
	var clientTotal, publisherTotal float64
	for index, sample := range cell.ResourceSamples {
		if sample.ClientRSS == 0 || sample.PublisherRSS == 0 || sample.ClientCPUPercent < 0 || sample.PublisherCPUPercent < 0 {
			return false
		}
		if index > 0 {
			previous := cell.ResourceSamples[index-1]
			interval := sample.AtNanos - previous.AtNanos
			if interval < int64(900*time.Millisecond) || interval > int64(1500*time.Millisecond) ||
				replacementTrafficBitrate(previous.AtNanos, sample.AtNanos,
					[]uint64{counterDelta(sample.ClientReceived, previous.ClientReceived),
						counterDelta(sample.ClientSent, previous.ClientSent),
						counterDelta(sample.PublisherReceived, previous.PublisherReceived),
						counterDelta(sample.PublisherSent, previous.PublisherSent)}) > replacementBitrateLimit {
				return false
			}
		}
		clientRSS, publisherRSS = append(clientRSS, sample.ClientRSS), append(publisherRSS, sample.PublisherRSS)
		clientCPU, publisherCPU = append(clientCPU, sample.ClientCPUPercent), append(publisherCPU, sample.PublisherCPUPercent)
		clientTotal += sample.ClientCPUPercent
		publisherTotal += sample.PublisherCPUPercent
	}
	if percentileResource(clientRSS, .95) > replacementRSSLimit || percentileResource(publisherRSS, .95) > replacementRSSLimit ||
		clientTotal/float64(len(clientCPU)) > 50 || publisherTotal/float64(len(publisherCPU)) > 50 ||
		percentileCPU(clientCPU, .95) > 100 || percentileCPU(publisherCPU, .95) > 100 {
		return false
	}
	clientTraffic := cell.FinalTraffic.ClientReceived + cell.FinalTraffic.ClientSent
	publisherTraffic := cell.FinalTraffic.PublisherReceived + cell.FinalTraffic.PublisherSent
	return directReplacementTraffic(clientTraffic, cell.Bytes) && directReplacementTraffic(publisherTraffic, cell.Bytes)
}

func replacementTrafficBitrate(first, second int64, deltas []uint64) float64 {
	highest := uint64(0)
	for _, value := range deltas {
		highest = max(highest, value)
	}
	return float64(highest) * 8 * float64(time.Second) / float64(second-first)
}

func percentileResource(values []uint64, fraction float64) uint64 {
	copyValues := append([]uint64(nil), values...)
	sort.Slice(copyValues, func(left, right int) bool { return copyValues[left] < copyValues[right] })
	return copyValues[percentilePosition(len(copyValues), fraction)]
}

func percentileCPU(values []float64, fraction float64) float64 {
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	return copyValues[percentilePosition(len(copyValues), fraction)]
}

func percentilePosition(length int, fraction float64) int {
	return max(0, min(length-1, int(float64(length)*fraction+0.999999)-1))
}

func directReplacementTraffic(value uint64, bytes uint32) bool {
	return value > 0 && value <= uint64(bytes)+replacementTrafficLimit
}

func counterDelta(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}
	return current - previous
}

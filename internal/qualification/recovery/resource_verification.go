package recovery

import (
	"sort"
	"time"
)

const endpointRSSLimit = uint64(512 << 20)
const recoveryTrafficAllowance = uint64(8 << 20)
const carrierBitrateLimit = float64(25_000_000)

func verifyResources(cell Cell) Result {
	if len(cell.ResourceSamples) < 3 || cell.BaselineClientTraffic == 0 || cell.BaselinePublisherTraffic == 0 {
		return invalid("paired baseline or one-second resource samples are incomplete")
	}
	clientRSS, publisherRSS := make([]uint64, 0, len(cell.ResourceSamples)), make([]uint64, 0, len(cell.ResourceSamples))
	clientCPU, publisherCPU := make([]float64, 0, len(cell.ResourceSamples)), make([]float64, 0, len(cell.ResourceSamples))
	var clientCPUTotal, publisherCPUTotal float64
	for index, sample := range cell.ResourceSamples {
		if sample.AtNanos <= 0 || sample.ClientRSS == 0 || sample.PublisherRSS == 0 ||
			sample.ClientCPUPercent < 0 || sample.PublisherCPUPercent < 0 {
			return invalid("host resource sample is malformed")
		}
		if index > 0 {
			previous := cell.ResourceSamples[index-1]
			interval := sample.AtNanos - previous.AtNanos
			if interval <= 0 || interval > int64(2*time.Second) || trafficBitrate(previous, sample) > carrierBitrateLimit {
				return fail("one-second carrier sampling or bitrate gate failed")
			}
		}
		clientRSS, publisherRSS = append(clientRSS, sample.ClientRSS), append(publisherRSS, sample.PublisherRSS)
		clientCPU, publisherCPU = append(clientCPU, sample.ClientCPUPercent), append(publisherCPU, sample.PublisherCPUPercent)
		clientCPUTotal += sample.ClientCPUPercent
		publisherCPUTotal += sample.PublisherCPUPercent
	}
	if percentileUint64(clientRSS, 0.95) > endpointRSSLimit || percentileUint64(publisherRSS, 0.95) > endpointRSSLimit ||
		clientCPUTotal/float64(len(clientCPU)) > 50 || publisherCPUTotal/float64(len(publisherCPU)) > 50 ||
		percentileFloat(clientCPU, 0.95) > 100 || percentileFloat(publisherCPU, 0.95) > 100 {
		return fail("endpoint RSS or CPU gate failed")
	}
	last := cell.ResourceSamples[len(cell.ResourceSamples)-1]
	clientTraffic, publisherTraffic := last.ClientReceived+last.ClientSent, last.PublisherReceived+last.PublisherSent
	if trafficExcess(clientTraffic, cell.BaselineClientTraffic) > recoveryTrafficAllowance ||
		trafficExcess(publisherTraffic, cell.BaselinePublisherTraffic) > recoveryTrafficAllowance {
		return fail("recovery episode exceeded paired endpoint carrier allowance")
	}
	return Result{Verdict: "pass"}
}

func trafficBitrate(previous, current ResourceSample) float64 {
	interval := float64(current.AtNanos - previous.AtNanos)
	if interval <= 0 {
		return carrierBitrateLimit + 1
	}
	deltas := []uint64{
		delta(current.ClientReceived, previous.ClientReceived), delta(current.ClientSent, previous.ClientSent),
		delta(current.PublisherReceived, previous.PublisherReceived), delta(current.PublisherSent, previous.PublisherSent),
	}
	var highest uint64
	for _, value := range deltas {
		highest = max(highest, value)
	}
	return float64(highest) * 8 * float64(time.Second) / interval
}

func delta(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}
	return current - previous
}
func trafficExcess(current, baseline uint64) uint64 { return delta(current, baseline) }

func percentileUint64(values []uint64, fraction float64) uint64 {
	copyValues := append([]uint64(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	return copyValues[percentileIndex(len(copyValues), fraction)]
}

func percentileFloat(values []float64, fraction float64) float64 {
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	return copyValues[percentileIndex(len(copyValues), fraction)]
}

func percentileIndex(length int, fraction float64) int {
	index := int(float64(length)*fraction+0.999999) - 1
	return max(0, min(length-1, index))
}

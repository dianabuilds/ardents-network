package recovery

import "time"

func verifyStressProcesses(values map[string]processObservationEvidence, scope hostScopeEvidence,
	services []string) Result {
	if len(values) != len(services) {
		return invalid("S4.3 host process set is incomplete")
	}
	identities := map[string]bool{}
	for _, service := range services {
		value, ok := values[service]
		if !ok || !validProcessObservation(value, scope) ||
			!validDockerProcessObservation(value, scope, service) || identities[value.Host.Identity] {
			return invalid("S4.3 host process identity or Adapter projection is invalid")
		}
		identities[value.Host.Identity] = true
	}
	return Result{Verdict: "pass"}
}

func verifyStressResourceInterval(samples []ResourceSample, measurement int64) Result {
	if len(samples) < 3 || samples[0].AtNanos > int64(1500*time.Millisecond) ||
		samples[len(samples)-1].AtNanos > measurement ||
		measurement-samples[len(samples)-1].AtNanos > int64(2*time.Second) {
		return invalid("S4.3 resource samples do not cover the impaired interval")
	}
	bitrates := make([]float64, 0, len(samples)-1)
	for index := 1; index < len(samples); index++ {
		bitrates = append(bitrates, trafficBitrate(samples[index-1], samples[index]))
	}
	if percentileFloat(bitrates, .95) > 20_000_000 {
		return fail("S4.3 p95 carrier bitrate exceeded 80% of the 25 Mbit/s link budget")
	}
	return Result{Verdict: "pass"}
}

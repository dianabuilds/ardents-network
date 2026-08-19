package blockedverify

import "math"

func mergeFinalRuntimeResources(files []finalRawTelemetry, value *finalResourceObservation) bool {
	for _, file := range files {
		if file.Role != "bridge" || file.Kind != "runtime.jsonl" {
			continue
		}
		var samples []finalBridgeRuntime
		if !validFinalRuntimeStream(file.Data) || !decodeFinalTelemetryLines(file.Data, &samples) {
			return false
		}
		for _, sample := range samples {
			if sample.Resource.Goroutines > math.MaxUint16 || sample.Resource.Timers > math.MaxUint16 ||
				sample.Resource.QueueItems > math.MaxUint16 || sample.Resource.QueueBytes > math.MaxUint32 {
				return false
			}
			value.GoroutinesPeak = max(value.GoroutinesPeak, uint16(sample.Resource.Goroutines))
			value.TimersPeak = max(value.TimersPeak, uint16(sample.Resource.Timers))
			value.QueueItemsPeak = max(value.QueueItemsPeak, uint16(sample.Resource.QueueItems))
			value.QueueBytesPeak = max(value.QueueBytesPeak, uint32(sample.Resource.QueueBytes))
		}
		value.EvidenceDropped = 0
		return true
	}
	return false
}

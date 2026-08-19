package blockedverify

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

type finalPressureInjection struct {
	Schema                string `json:"schema"`
	Ordinal               uint16 `json:"ordinal"`
	CorpusSHA256          string `json:"corpus_sha256"`
	ScheduledOffsetMillis uint64 `json:"scheduled_offset_millis"`
	StartedOffsetMillis   uint64 `json:"started_offset_millis"`
	Bytes                 uint16 `json:"bytes"`
	Connected             bool   `json:"connected"`
}

type finalPressureStateObservation struct {
	Schema       string `json:"schema"`
	Ordinal      uint16 `json:"ordinal"`
	State        string `json:"state"`
	OffsetMillis uint64 `json:"offset_millis"`
}

func validFinalPressureInjectionStream(raw []byte) bool {
	var values []finalPressureInjection
	if !decodeFinalTelemetryLines(raw, &values) || len(values) < 1 || len(values) > 23 {
		return false
	}
	for index, value := range values {
		if value.Schema != "ardents-h3-pressure-injection-v1" || value.Ordinal != uint16(index) ||
			value.Bytes != 128 || !value.Connected || value.StartedOffsetMillis < value.ScheduledOffsetMillis ||
			value.StartedOffsetMillis > value.ScheduledOffsetMillis+250 {
			return false
		}
	}
	return true
}

func validFinalPressureStateStream(raw []byte) bool {
	var values []finalPressureStateObservation
	if !decodeFinalTelemetryLines(raw, &values) || len(values) != 3 {
		return false
	}
	for index, value := range values {
		if value.Schema != "ardents-h3-pressure-state-v1" || value.Ordinal != uint16(index) ||
			index > 0 && value.OffsetMillis <= values[index-1].OffsetMillis {
			return false
		}
	}
	return true
}

func finalPressureInjectionBasis(files []finalRawTelemetry, cellSeed string, count int) (
	[]finalPressureInjection, []finalPressureStateObservation, bool,
) {
	var injections []finalPressureInjection
	var states []finalPressureStateObservation
	for _, file := range files {
		switch file.Kind {
		case "pressure-injection.jsonl":
			if !validFinalPressureInjectionStream(file.Data) || !decodeFinalTelemetryLines(file.Data, &injections) {
				return nil, nil, false
			}
		case "pressure-state.jsonl":
			if !validFinalPressureStateStream(file.Data) || !decodeFinalTelemetryLines(file.Data, &states) {
				return nil, nil, false
			}
		}
	}
	seed, err := hex.DecodeString(cellSeed)
	if err != nil || len(seed) != 32 || len(injections) != count || len(states) != 3 {
		return nil, nil, false
	}
	derived := sha256.Sum256(append(append(append([]byte(nil), seed...), 0), "partial-handshakes"...))
	for index, injection := range injections {
		if index > 0 && injection.ScheduledOffsetMillis != injections[0].ScheduledOffsetMillis+uint64(index*500) {
			return nil, nil, false
		}
		prefix := finalPressureCorpus(derived[:], index)
		digest := sha256.Sum256(prefix)
		if injection.CorpusSHA256 != hex.EncodeToString(digest[:]) {
			return nil, nil, false
		}
	}
	return injections, states, true
}

func finalPressureCorpus(seed []byte, index int) []byte {
	prefix := make([]byte, 128)
	prefix[0], prefix[1], prefix[2], prefix[3], prefix[4] = 22, 3, 3, 0x10, 0
	ordinal := make([]byte, 8)
	binary.BigEndian.PutUint64(ordinal, uint64(index))
	for offset, blockIndex := 5, byte(0); offset < len(prefix); blockIndex++ {
		input := append(append(append([]byte(nil), seed...), ordinal...), blockIndex)
		block := sha256.Sum256(input)
		offset += copy(prefix[offset:], block[:])
	}
	return prefix
}

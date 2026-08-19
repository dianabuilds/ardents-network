package blockedverify

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestFinalPressureInjectionBasisBindsSeedCorpusAndCadence(t *testing.T) {
	cellSeed := strings.Repeat("02", 32)
	seed, err := hex.DecodeString(cellSeed)
	if err != nil {
		t.Fatal(err)
	}
	derived := sha256.Sum256(append(append(append([]byte(nil), seed...), 0), "partial-handshakes"...))
	injections := make([]finalPressureInjection, 20)
	for index := range injections {
		corpus := finalPressureCorpus(derived[:], index)
		digest := sha256.Sum256(corpus)
		injections[index] = finalPressureInjection{Schema: "ardents-h3-pressure-injection-v1",
			Ordinal: uint16(index), CorpusSHA256: hex.EncodeToString(digest[:]),
			ScheduledOffsetMillis: 2_000 + uint64(index*500), StartedOffsetMillis: 2_000 + uint64(index*500),
			Bytes: 128, Connected: true}
	}
	states := []finalPressureStateObservation{
		{Schema: "ardents-h3-pressure-state-v1", Ordinal: 0, State: "NORMAL", OffsetMillis: 1_000},
		{Schema: "ardents-h3-pressure-state-v1", Ordinal: 1, State: "PROTECT", OffsetMillis: 12_000},
		{Schema: "ardents-h3-pressure-state-v1", Ordinal: 2, State: "NORMAL", OffsetMillis: 133_000},
	}
	files := []finalRawTelemetry{
		{Role: "pressure", Kind: "pressure-injection.jsonl", Data: telemetryLines(t, injections)},
		{Role: "bridge", Kind: "pressure-state.jsonl", Data: telemetryLines(t, states)},
	}
	if _, _, ok := finalPressureInjectionBasis(files, cellSeed, 20); !ok {
		t.Fatal("valid manifest-seeded injection basis was rejected")
	}
	injections[7].CorpusSHA256 = strings.Repeat("0", 64)
	files[0].Data = telemetryLines(t, injections)
	if _, _, ok := finalPressureInjectionBasis(files, cellSeed, 20); ok {
		t.Fatal("changed injection corpus was accepted")
	}
}

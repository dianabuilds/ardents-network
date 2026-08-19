package blockedverify

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalTelemetryRejectsUnknownOrEmptyFiles(t *testing.T) {
	cell := "profile/C1/00"
	valid := fixtureFinalTelemetryIndex(cell)
	if !validFinalRawTelemetry(valid, cell) {
		t.Fatal("known bounded telemetry was rejected")
	}
	valid = valid[:5]
	if validFinalRawTelemetry(valid, cell) {
		t.Fatal("empty telemetry was accepted")
	}
}

func TestFinalTelemetryP4RequiresAllOrderedStreams(t *testing.T) {
	cell := "pressure/P4"
	valid := fixtureFinalTelemetryIndex(cell)
	if len(valid) != 21 || !validFinalRawTelemetry(valid, cell) {
		t.Fatal("complete P4 telemetry inventory was rejected")
	}
	valid[7], valid[8] = valid[8], valid[7]
	if validFinalRawTelemetry(valid, cell) {
		t.Fatal("reordered P4 telemetry inventory was accepted")
	}
}

func TestFinalTelemetryValidatesWorkerPressureAndRecoveryMeasurements(t *testing.T) {
	pressure := finalPressureCell{Schema: "ardents-h3-final-pressure-v1", ID: "P2", Terminal: "normal"}
	recovery := finalRecoveryMeasurement{Schema: "ardents-h3-final-recovery-episode-v1", Episode: 3,
		ServiceClass: "abrupt connection loss", RecoveryClass: "bridge-deadline-exceeded",
		Attempt: strings.Repeat("a", 64), ContactStarts: 1, Cleanup: true}
	files := []finalRawTelemetry{{Kind: "pressure.json", Data: telemetryLines(t, []finalPressureCell{pressure})},
		{Kind: "recovery.json", Data: telemetryLines(t, []finalRecoveryMeasurement{recovery})}}
	if !validFinalTelemetryStreams(files) {
		t.Fatal("valid final worker measurements were rejected")
	}
	files[1].Data = telemetryLines(t, []finalRecoveryMeasurement{{Schema: recovery.Schema, Episode: 5}})
	if validFinalTelemetryStreams(files) {
		t.Fatal("out-of-range recovery episode was accepted")
	}
}

func TestFinalPressureRecomputesAdmissionFromRawInput(t *testing.T) {
	cellSeed := strings.Repeat("01", 32)
	outcomes := fixtureFinalAdmissionOffers(t, cellSeed, "offers")
	input := finalPressureInput{Schema: "ardents-h3-final-pressure-input-v1", Progress: true,
		Admission: finalAdmissionInput{Schema: "ardents-h3-s5-admission-result-v1",
			Offers: 100, Refused: 100, MaximumMillis: 250, Outcomes: outcomes}}
	pressure := finalPressureCell{Schema: "ardents-h3-final-pressure-v1", ID: "P1", Terminal: "normal",
		Offers: 100, Refused: 100, MaximumRefusalMillis: 250, Progress: true}
	files := []finalRawTelemetry{
		{Role: "bridge", Kind: "resource.jsonl", Data: resourceTelemetryLines(t, 1<<20, 3, 2, 2)},
		{Role: "bridge", Kind: "pressure-input.json", Data: telemetryLines(t, []finalPressureInput{input})},
		{Role: "bridge", Kind: "pressure.json", Data: telemetryLines(t, []finalPressureCell{pressure})},
	}
	if _, ok := finalPressureFromTelemetry(files, "P1", cellSeed); !ok {
		t.Fatal("valid raw admission basis did not reproduce P1")
	}
	pressure.Refused--
	files[2].Data = telemetryLines(t, []finalPressureCell{pressure})
	if _, ok := finalPressureFromTelemetry(files, "P1", cellSeed); ok {
		t.Fatal("changed P1 refusal aggregate was accepted")
	}
	pressure.Refused++
	input.Admission.Outcomes[11].ScheduledOffsetMillis++
	files[1].Data = telemetryLines(t, []finalPressureInput{input})
	files[2].Data = telemetryLines(t, []finalPressureCell{pressure})
	if _, ok := finalPressureFromTelemetry(files, "P1", cellSeed); ok {
		t.Fatal("changed P1 offer cadence was accepted")
	}
}

func fixtureFinalAdmissionOffers(t *testing.T, cellSeed, label string) []finalAdmissionOffer {
	t.Helper()
	seed, err := hex.DecodeString(cellSeed)
	if err != nil {
		t.Fatal(err)
	}
	derived := sha256.Sum256(append(append(append([]byte(nil), seed...), 0), label...))
	result := make([]finalAdmissionOffer, 100)
	for index := range result {
		ordinal := make([]byte, 8)
		binary.BigEndian.PutUint64(ordinal, uint64(index))
		canary := sha256.Sum256(append(append([]byte(nil), derived[:]...), ordinal...))
		digest := sha256.Sum256(canary[:])
		result[index] = finalAdmissionOffer{Ordinal: uint16(index), CanarySHA256: hex.EncodeToString(digest[:]),
			ScheduledOffsetMillis: uint32(index * 100), StartedOffsetMillis: uint32(index * 100),
			RefusalMillis: 250, Refused: true}
	}
	return result
}

func TestLoadFinalTelemetryV2RejectsChangedStream(t *testing.T) {
	root := t.TempDir()
	cell := finalCellObservation{ID: "profile/C1/00"}
	files := fixtureFinalTelemetryIndex(cell.ID)
	for index := range files {
		path := filepath.Join(root, filepath.FromSlash(files[index].Artifact.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		raw := []byte("sample\n")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		files[index].Artifact.SHA256 = hex.EncodeToString(digest[:])
		files[index].Artifact.Bytes = int64(len(raw))
	}
	index := finalRawTelemetryEvidence{Schema: "ardents-h3-final-raw-telemetry-v2", CellID: cell.ID, Files: files}
	raw, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	indexPath := filepath.Join(root, filepath.FromSlash(finalTelemetryEvidencePath(cell.ID)))
	if err := os.WriteFile(indexPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	cell.TelemetryEvidence = artifactCommitment{Path: finalTelemetryEvidencePath(cell.ID),
		SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(raw))}
	if _, reason := loadFinalRawTelemetry(root, cell); reason != "" {
		t.Fatalf("valid telemetry v2 reason=%s", reason)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(files[0].Artifact.Path)), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, reason := loadFinalRawTelemetry(root, cell); reason == "" {
		t.Fatal("changed telemetry stream was accepted")
	}
}

func TestFinalTelemetryRejectsMissingOrCoalescedStreamRecords(t *testing.T) {
	resource := []finalResourceSample{{Schema: "ardents-h3-process-resource-v1", Ordinal: 0},
		{Schema: "ardents-h3-process-resource-v1", Ordinal: 1, OffsetMillis: 1_000}}
	carrier := []finalCarrierSample{{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 0, Boundary: "before"},
		{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 1, OffsetMillis: 1_000, Bytes: 10},
		{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 2, OffsetMillis: 2_000, Bytes: 20, Boundary: "after"}}
	files := []finalRawTelemetry{{Role: "bridge", Kind: "resource.jsonl", Data: telemetryLines(t, resource)},
		{Role: "bridge", Kind: "carrier.jsonl", Data: telemetryLines(t, carrier)}}
	if !validFinalTelemetryStreams(files) {
		t.Fatal("valid raw telemetry streams were rejected")
	}
	resource[1].OffsetMillis = 10
	files[0].Data = telemetryLines(t, resource)
	if validFinalTelemetryStreams(files) {
		t.Fatal("coalesced resource stream was accepted")
	}
}

func TestFinalTelemetryReproducesBridgeHelpersWithoutChargingPublisher(t *testing.T) {
	carrier := sustainedCarrierSamples(0)
	for index := range carrier {
		carrier[index].Bytes = 0
	}
	files := []finalRawTelemetry{
		{Role: "endpoint", Kind: "resource.jsonl", Data: resourceTelemetryLines(t, 3<<20, 7, 3, 5)},
		{Role: "endpoint", Kind: "tree.jsonl", Data: treeTelemetryLines(t, 601, 0, 0)},
		{Role: "bridge", Kind: "resource.jsonl", Data: resourceTelemetryLines(t, 1<<20, 3, 1, 2)},
		{Role: "bridge", Kind: "tree.jsonl", Data: treeTelemetryLines(t, 601, 0, 0)},
		{Role: "bridge", Kind: "carrier.jsonl", Data: telemetryLines(t, carrier)},
		{Role: "bridge", Kind: "runtime.jsonl", Data: runtimeTelemetryLines(t)},
		{Role: "publisher", Kind: "resource.jsonl", Data: resourceTelemetryLines(t, 2<<20, 5, 2, 4)},
	}
	want := finalResourceObservation{HelperRSSP95MiB: 1, HelperFDPeak: 3, HelperSocketPeak: 1,
		ThreadsPeak: 2, AdapterRSSP95MiB: 3, AdapterFDPeak: 7, AdapterSocketPeak: 3,
		ReservePercent: 100, Samples: 600, SamplesComplete: true,
		Collected: []string{"endpoint-cpu", "endpoint-rss", "adapter-rss", "adapter-fds", "adapter-sockets",
			"adapter-state", "bridge-cpu", "bridge-memory", "helper-rss", "helper-fds", "helper-sockets",
			"swap-oom", "threads", "goroutines", "timers", "queues", "durable-state", "evidence",
			"traffic", "descendants", "capabilities", "reserve"}}
	if !reproducesFinalRoleResources(files, want, 0, 600_000) {
		t.Fatal("published sustained resources did not reproduce all three raw role streams")
	}
	want.HelperFDPeak--
	if reproducesFinalRoleResources(files, want, 0, 600_000) {
		t.Fatal("changed helper resource aggregate was accepted")
	}
}

func TestFinalTelemetryRejectsShortSustainedRoleResources(t *testing.T) {
	files := []finalRawTelemetry{
		{Role: "endpoint", Kind: "resource.jsonl", Data: resourceTelemetryLines(t, 1, 1, 1, 1)},
		{Role: "bridge", Kind: "resource.jsonl", Data: resourceTelemetryLines(t, 1, 1, 1, 1)},
		{Role: "publisher", Kind: "resource.jsonl", Data: resourceTelemetryLines(t, 1, 1, 1, 1)},
	}
	files[2].Data = shortResourceTelemetryLines(t)
	if reproducesFinalRoleResources(files, finalResourceObservation{Collected: requiredResourceObservations()}, 0, 600_000) {
		t.Fatal("599 seconds plus a post-cleanup record reproduced a ten-minute resource window")
	}
}

func TestFinalTelemetryResourceP95ExcludesOutsideWindow(t *testing.T) {
	carrier := sustainedCarrierSamples(0)
	for index := range carrier {
		carrier[index].Bytes = 0
	}
	files := []finalRawTelemetry{
		{Role: "endpoint", Kind: "resource.jsonl", Data: windowedResourceTelemetryLines(t, 1<<20)},
		{Role: "endpoint", Kind: "tree.jsonl", Data: treeTelemetryLines(t, 701, 0, 0)},
		{Role: "bridge", Kind: "resource.jsonl", Data: windowedResourceTelemetryLines(t, 10<<20)},
		{Role: "bridge", Kind: "tree.jsonl", Data: treeTelemetryLines(t, 701, 0, 0)},
		{Role: "bridge", Kind: "carrier.jsonl", Data: telemetryLines(t, carrier)},
		{Role: "bridge", Kind: "runtime.jsonl", Data: runtimeTelemetryLines(t)},
		{Role: "publisher", Kind: "resource.jsonl", Data: windowedResourceTelemetryLines(t, 1<<20)},
	}
	want := finalResourceObservation{HelperRSSP95MiB: 10, AdapterRSSP95MiB: 1,
		ReservePercent: 100, Samples: 600, SamplesComplete: true,
		Collected: requiredResourceObservations()}
	if !reproducesFinalRoleResources(files, want, 100_000, 700_000) {
		t.Fatal("active-window p95 did not reproduce with low setup samples")
	}
}

func treeTelemetryLines(t *testing.T, count int, cpu float64, rss uint64) []byte {
	t.Helper()
	values := make([]finalTreeSample, 0, count)
	for ordinal := range count {
		values = append(values, finalTreeSample{Schema: "ardents-h3-tree-resource-v1",
			Ordinal: uint16(ordinal), OffsetMillis: uint64(ordinal * 1_000), CPUCores: cpu, RSSBytes: rss})
	}
	return telemetryLines(t, values)
}

func runtimeTelemetryLines(t *testing.T) []byte {
	t.Helper()
	return telemetryLines(t, []finalBridgeRuntime{{Schema: "ardents-h3-bridge-runtime-v1"}})
}

func TestFinalCarrierDeltaUsesBeforeAndAfterCounters(t *testing.T) {
	carrier := sustainedCarrierSamples(7)
	delta, ok := finalCarrierDelta([]finalRawTelemetry{{Role: "endpoint", Kind: "carrier.jsonl",
		Data: telemetryLines(t, carrier)}}, "endpoint", 0, 600_000)
	if !ok || delta != 601 {
		t.Fatalf("delta=%d ok=%t", delta, ok)
	}
}

func TestFinalCarrierDeltaRejectsSparseTenMinuteStream(t *testing.T) {
	carrier := []finalCarrierSample{{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 0, Boundary: "before"},
		{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 1, OffsetMillis: 1_000},
		{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 2, OffsetMillis: 600_000, Boundary: "after"}}
	if _, ok := finalCarrierDelta([]finalRawTelemetry{{Role: "endpoint", Kind: "carrier.jsonl",
		Data: telemetryLines(t, carrier)}}, "endpoint", 0, 600_000); ok {
		t.Fatal("sparse ten-minute carrier stream was accepted")
	}
}

func TestFinalCapacityResourcesReproduceEveryEndpointRoot(t *testing.T) {
	resource := telemetryLines(t, []finalResourceSample{{Schema: "ardents-h3-process-resource-v1", Processes: 2}})
	tree := treeTelemetryLines(t, 1, 0, 0)
	files := make([]finalRawTelemetry, 0, 19)
	for root := range 4 {
		files = append(files, finalRawTelemetry{Root: uint16(root), Role: "endpoint", Kind: "resource.jsonl", Data: resource},
			finalRawTelemetry{Root: uint16(root), Role: "endpoint", Kind: "tree.jsonl", Data: tree})
	}
	carrier := []finalCarrierSample{{Schema: "ardents-h3-carrier-counter-v1", Boundary: "before"},
		{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 1, OffsetMillis: 1_000},
		{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 2, OffsetMillis: 2_000, Boundary: "after"}}
	files = append(files, finalRawTelemetry{Role: "bridge", Kind: "resource.jsonl", Data: resource},
		finalRawTelemetry{Role: "bridge", Kind: "tree.jsonl", Data: tree},
		finalRawTelemetry{Role: "bridge", Kind: "carrier.jsonl", Data: telemetryLines(t, carrier)},
		finalRawTelemetry{Role: "bridge", Kind: "runtime.jsonl", Data: runtimeTelemetryLines(t)},
		finalRawTelemetry{Role: "publisher", Kind: "resource.jsonl", Data: resource},
		finalRawTelemetry{Role: "publisher", Kind: "tree.jsonl", Data: tree})
	want := finalResourceObservation{ReservePercent: 100, Samples: 6, SamplesComplete: true,
		Collected: requiredResourceObservations()}
	if !reproducesFinalCapacityResources(files, want, false) {
		t.Fatal("valid reference capacity resources were not independently reproduced")
	}
	files = files[2:]
	if reproducesFinalCapacityResources(files, want, false) {
		t.Fatal("missing capacity Endpoint root was accepted")
	}
}

func sustainedCarrierSamples(initial uint64) []finalCarrierSample {
	result := []finalCarrierSample{{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 0,
		Bytes: initial, Boundary: "before"}}
	for ordinal := 1; ordinal <= 600; ordinal++ {
		result = append(result, finalCarrierSample{Schema: "ardents-h3-carrier-counter-v1",
			Ordinal: uint16(ordinal), OffsetMillis: uint64(ordinal * 1_000), Bytes: initial + uint64(ordinal)})
	}
	return append(result, finalCarrierSample{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 601,
		OffsetMillis: 601_000, Bytes: initial + 601, Boundary: "after"})
}

func telemetryLines[T any](t *testing.T, values []T) []byte {
	t.Helper()
	var result []byte
	for _, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, raw...)
		result = append(result, '\n')
	}
	return result
}

func resourceTelemetryLines(t *testing.T, rss uint64, fds, sockets, threads uint16) []byte {
	t.Helper()
	values := make([]finalResourceSample, 0, 602)
	for ordinal := range 601 {
		values = append(values, finalResourceSample{Schema: "ardents-h3-process-resource-v1",
			Ordinal: uint16(ordinal), OffsetMillis: uint64(ordinal * 1_000), RSSBytes: rss,
			FDs: fds, Sockets: sockets, Processes: 2, Threads: threads})
	}
	values = append(values, finalResourceSample{Schema: "ardents-h3-process-resource-v1", Ordinal: 601,
		OffsetMillis: 601_000, RSSBytes: rss, FDs: fds, Sockets: sockets, Processes: 2,
		Threads: threads, Boundary: "post-cleanup"})
	return telemetryLines(t, values)
}

func shortResourceTelemetryLines(t *testing.T) []byte {
	t.Helper()
	values := make([]finalResourceSample, 0, 601)
	for ordinal := range 600 {
		values = append(values, finalResourceSample{Schema: "ardents-h3-process-resource-v1",
			Ordinal: uint16(ordinal), OffsetMillis: uint64(ordinal * 1_000), Processes: 2})
	}
	values = append(values, finalResourceSample{Schema: "ardents-h3-process-resource-v1", Ordinal: 600,
		OffsetMillis: 600_000, Processes: 2, Boundary: "post-cleanup"})
	return telemetryLines(t, values)
}

func windowedResourceTelemetryLines(t *testing.T, highRSS uint64) []byte {
	t.Helper()
	values := make([]finalResourceSample, 0, 702)
	for ordinal := range 701 {
		rss := uint64(1 << 20)
		if ordinal >= 669 && ordinal < 700 {
			rss = highRSS
		}
		values = append(values, finalResourceSample{Schema: "ardents-h3-process-resource-v1",
			Ordinal: uint16(ordinal), OffsetMillis: uint64(ordinal * 1_000), RSSBytes: rss, Processes: 2})
	}
	values = append(values, finalResourceSample{Schema: "ardents-h3-process-resource-v1", Ordinal: 701,
		OffsetMillis: 701_000, Processes: 2, Boundary: "post-cleanup"})
	return telemetryLines(t, values)
}

func fixtureFinalTelemetryIndex(cell string) []finalRawTelemetry {
	layout := finalTelemetryLayout(cell)
	result := make([]finalRawTelemetry, 0, len(layout))
	for index, slot := range layout {
		result = append(result, finalRawTelemetry{Root: slot.root, Role: slot.role, Kind: slot.kind,
			Artifact: artifactCommitment{Path: finalTelemetryStreamPath(cell, index),
				SHA256: strings.Repeat("a", 64), Bytes: 7}})
	}
	return result
}

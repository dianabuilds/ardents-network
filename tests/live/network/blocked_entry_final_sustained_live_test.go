//go:build live

package network_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const finalSustainedBytes = uint32(800_000_000)

type finalSustainedCellEvidence struct {
	Schema                string                      `json:"schema"`
	Direction             string                      `json:"direction"`
	DirectBeforeMbit      float64                     `json:"direct_before_mbit"`
	DirectAfterMbit       float64                     `json:"direct_after_mbit"`
	DirectBeforeValid     bool                        `json:"direct_before_valid"`
	DirectAfterValid      bool                        `json:"direct_after_valid"`
	Runs                  []finalSustainedRunEvidence `json:"runs"`
	EndpointCarrierRatio  float64                     `json:"endpoint_carrier_ratio"`
	PublisherCarrierRatio float64                     `json:"publisher_carrier_ratio"`
	DirectPairID          string                      `json:"direct_pair_id"`
	DirectBefore          finalDirectRunEvidence      `json:"direct_before"`
	DirectAfter           finalDirectRunEvidence      `json:"direct_after"`
	EndpointCarrierBytes  uint64                      `json:"endpoint_carrier_bytes"`
	PublisherCarrierBytes uint64                      `json:"publisher_carrier_bytes"`
	DeliveredBytes        uint64                      `json:"delivered_bytes"`
}

type finalDirectRunEvidence struct {
	StartedOffsetMillis  uint64 `json:"started_offset_millis"`
	FinishedOffsetMillis uint64 `json:"finished_offset_millis"`
	DurationMillis       uint32 `json:"duration_millis"`
	DeliveredBytes       uint64 `json:"delivered_bytes"`
	Digest               string `json:"digest"`
	PairID               string `json:"pair_id"`
	Complete             bool   `json:"complete"`
}

func TestBlockedEntryFinalSustainedEvidence(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository, toolImage := repositoryRoot(t), liveToolImage(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s55-sustained-%d:test", time.Now().UnixNano()))
	buildFixture := newBlockedEntryFixture(t, client, server)
	buildProject := finalProjectName(fmt.Sprintf("ardents-s55-sustained-build-%d", time.Now().UnixNano()))
	build := blockedCompose(repository, buildProject, image, buildFixture, "final-sustained")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	if ownedImage {
		if output, err := build(ctx, "build", "endpoint"); err != nil {
			cancel()
			t.Fatalf("build final sustained image: %v\n%s", err, output)
		}
	}
	cancel()
	if ownedImage {
		t.Cleanup(func() { removeBlockedPressureImage(t, image, buildProject) })
	}
	timeline, err := finalWorkerTimelineOrigin()
	if err != nil {
		t.Fatal(err)
	}
	if selected := os.Getenv("ARDENTS_FINAL_CELL"); selected != "" {
		runSelectedFinalSustainedCell(t, repository, image, toolImage, client, server, selected, timeline)
		return
	}
	for _, direction := range []string{"endpoint-to-publisher", "publisher-to-endpoint"} {
		t.Run(direction, func(t *testing.T) {
			runFinalSustainedDirection(t, repository, image, toolImage, client, server, direction, timeline)
		})
	}
}

func runSelectedFinalSustainedCell(t *testing.T, repository, image, toolImage, client, server,
	cell string, timeline time.Time,
) {
	t.Helper()
	parts := strings.Split(cell, "/")
	if len(parts) != 3 || parts[0] != "sustained" ||
		parts[1] != "endpoint-to-publisher" && parts[1] != "publisher-to-endpoint" {
		t.Fatalf("invalid selected sustained cell %q", cell)
	}
	direction, started := parts[1], time.Now()
	if parts[2] == "direct-before" || parts[2] == "direct-after" {
		fixture := newBlockedEntryFixture(t, client, server)
		bindFinalFixturePairSeed(t, fixture, "sustained/"+direction+"/direct-before",
			"sustained/"+direction+"/direct-after", "direct-stream")
		project := finalProjectName(fmt.Sprintf("ardents-s55-direct-selected-%d", time.Now().UnixNano()))
		compose := blockedCompose(repository, project, image, fixture, "final-sustained")
		cleanup := blockedProjectCleanup(t, compose, project)
		t.Cleanup(cleanup)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		pairDigest := sha256.Sum256([]byte("ardents-h3-final-direct-pair-v1/" + direction))
		pairID := hex.EncodeToString(pairDigest[:])
		directMbit, direct := runFinalDirectBaseline(t, ctx, compose, toolImage, direction, fixture.root, pairID, timeline)
		armFinalWorkerTerminal("complete")
		cleanup()
		emitFinalWorkerSustained(t, cell, "complete", started, finalWorkerSustained{Direction: direction,
			Kind: parts[2], DirectMbit: directMbit, Direct: &direct}, fixture.root)
		return
	}
	var run int
	if _, err := fmt.Sscanf(parts[2], "run-%d", &run); err != nil || run < 0 || run > 4 {
		t.Fatalf("invalid selected sustained run %q", cell)
	}
	armFinalWorkerTerminal("complete")
	runResult, endpointBytes, publisherBytes, root := runFinalSustainedCarrier(t, repository, image, toolImage, client, server,
		direction, run, timeline)
	emitFinalWorkerSustained(t, cell, "complete", started, finalWorkerSustained{Direction: direction,
		Kind: parts[2], Run: &runResult, EndpointCarrierBytes: endpointBytes, PublisherCarrierBytes: publisherBytes}, root)
}

func runFinalSustainedDirection(t *testing.T, repository, image, toolImage, client, server,
	direction string, timeline time.Time,
) {
	t.Helper()
	baselineFixture := newBlockedEntryFixture(t, client, server)
	directBeforeCell := "sustained/" + direction + "/direct-before"
	directAfterCell := "sustained/" + direction + "/direct-after"
	bindFinalFixturePairSeed(t, baselineFixture, directBeforeCell, directAfterCell, "direct-stream")
	project := finalProjectName(fmt.Sprintf("ardents-s55-direct-%d", time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, baselineFixture, "final-sustained")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 115*time.Minute)
	defer cancel()
	pairDigest := sha256.Sum256([]byte("ardents-h3-final-direct-pair-v1/" + direction))
	pairID := hex.EncodeToString(pairDigest[:])
	directBeforeStarted := time.Now()
	directBefore, beforeEvidence := runFinalDirectBaseline(t, ctx, compose, toolImage, direction,
		baselineFixture.root, pairID, timeline)
	emitFinalWorkerCell(t, directBeforeCell, "complete", directBeforeStarted, baselineFixture.root)
	cell := finalSustainedCellEvidence{Schema: "ardents-h3-final-sustained-v1", Direction: direction,
		DirectBeforeMbit: directBefore, DirectBeforeValid: true, DirectPairID: pairID, DirectBefore: beforeEvidence}
	for run := range 5 {
		runStarted := time.Now()
		result, endpointBytes, publisherBytes, root := runFinalSustainedCarrier(t, repository, image, toolImage,
			client, server, direction, run, timeline)
		emitFinalWorkerCell(t, fmt.Sprintf("sustained/%s/run-%d", direction, run), "complete", runStarted, root)
		cell.Runs = append(cell.Runs, result)
		cell.EndpointCarrierBytes += endpointBytes
		cell.PublisherCarrierBytes += publisherBytes
		cell.DeliveredBytes += result.DeliveredBytes
	}
	cell.EndpointCarrierRatio = float64(cell.EndpointCarrierBytes) / float64(cell.DeliveredBytes)
	cell.PublisherCarrierRatio = float64(cell.PublisherCarrierBytes) / float64(cell.DeliveredBytes)
	directAfterStarted := time.Now()
	cell.DirectAfterMbit, cell.DirectAfter = runFinalDirectBaseline(t, ctx, compose, toolImage, direction,
		baselineFixture.root, pairID, timeline)
	emitFinalWorkerCell(t, directAfterCell, "complete", directAfterStarted, baselineFixture.root)
	cell.DirectAfterValid = true
	assertFinalSustainedCell(t, cell)
	if err := json.NewEncoder(os.Stdout).Encode(cell); err != nil {
		t.Fatal(err)
	}
	cleanup()
}

func runFinalSustainedCarrier(t *testing.T, repository, image, toolImage, client, server,
	direction string, run int, timeline time.Time,
) (finalSustainedRunEvidence, uint64, uint64, string) {
	t.Helper()
	fixture := newBlockedEntryFixture(t, client, server)
	bindFinalFixtureSeed(t, fixture, fmt.Sprintf("sustained/%s/run-%d", direction, run), "sustained-stream")
	rewriteBlockedWorkload(t, fixture, direction, finalSustainedBytes)
	chunkCount := (uint64(finalSustainedBytes) + 16_380) / 16_381
	delay := (10 * time.Minute) / time.Duration(chunkCount-1)
	clientSend, clientReceive := uint32(0), finalSustainedBytes
	publisherSend, publisherReceive := finalSustainedBytes, uint32(0)
	receiver := "client-app"
	if direction == "endpoint-to-publisher" {
		clientSend, clientReceive, publisherSend, publisherReceive = finalSustainedBytes, 0, 0, finalSustainedBytes
		receiver = "publisher-app"
	}
	for name, value := range map[string]string{
		"ARDENTS_BLOCKED_WORKLOAD": "sustained", "ARDENTS_BLOCKED_START_FILE": "/run/input/sustained.start",
		"ARDENTS_BLOCKED_CLIENT_SEND_BYTES":       fmt.Sprint(clientSend),
		"ARDENTS_BLOCKED_CLIENT_RECEIVE_BYTES":    fmt.Sprint(clientReceive),
		"ARDENTS_BLOCKED_PUBLISHER_SEND_BYTES":    fmt.Sprint(publisherSend),
		"ARDENTS_BLOCKED_PUBLISHER_RECEIVE_BYTES": fmt.Sprint(publisherReceive),
		"ARDENTS_STREAM_PROGRESS":                 "1", "ARDENTS_STREAM_CHUNK_DELAY": delay.String(),
		"ARDENTS_STREAM_LIFETIME": "15m"} {
		t.Setenv(name, value)
	}
	project := finalProjectName(fmt.Sprintf("ardents-s55-sustained-%s-%d-%d", direction, run, time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "final-sustained")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Minute)
	defer cancel()
	startBlockedPressureWork(t, ctx, compose)
	for _, service := range []string{"endpoint-resource-collector", "bridge-resource-collector",
		"publisher-resource-collector"} {
		startLiveContainer(t, ctx, compose, service)
	}
	for _, service := range []string{"endpoint-carrier-collector", "bridge-carrier-collector",
		"publisher-carrier-collector"} {
		startLiveContainer(t, ctx, compose, service)
	}
	for _, role := range []string{"endpoint", "bridge", "publisher"} {
		waitForBlockedHostFile(t, ctx, filepath.Join(fixture.root, "sync", role, "resource-ready"))
		waitForBlockedHostFile(t, ctx, filepath.Join(fixture.root, "sync", role, "carrier-ready"))
	}
	applyFinalBlockedNetwork(t, ctx, compose, toolImage)
	for _, role := range []string{"endpoint", "bridge", "publisher"} {
		writeLiveFile(t, filepath.Join(fixture.root, "sync", role, "carrier-start"), []byte("start\n"))
		waitForBlockedHostFile(t, ctx, filepath.Join(fixture.root, "sync", role, "carrier-started"))
	}
	started := time.Now()
	for _, role := range []string{"client-app", "publisher-app"} {
		writeLiveFile(t, filepath.Join(fixture.root, "input", role, "sustained.start"), []byte("start\n"))
	}
	result := monitorFinalSustained(t, ctx, compose, receiver, finalSustainedBytes, started, timeline)
	clientApp, publisherApp := waitForApplication(t, ctx, compose, "client-app"),
		waitForApplication(t, ctx, compose, "publisher-app")
	if clientApp.Terminal != "success" || publisherApp.Terminal != "success" {
		t.Fatalf("final sustained Application failed: client=%+v publisher=%+v", clientApp, publisherApp)
	}
	received := clientApp
	if direction == "endpoint-to-publisher" {
		received = publisherApp
	}
	result.DeliveredBytes = uint64(received.ReceivedBytes)
	result.Digest = hex.EncodeToString(received.ReceivedDigest[:])
	assertBlockedPressureServices(t, fixture, compose, ctx)
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		if route := waitForKind(t, ctx, compose, role, "complete"); route.Terminal != "success" || !route.Cleanup {
			t.Fatalf("final sustained %s Route = %+v", role, route)
		}
	}
	publishFinalWorkerTerminal()
	writeLiveFile(t, filepath.Join(fixture.root, "sync", "bridge", "bridge-stop"), []byte("stop\n"))
	for _, role := range []string{"endpoint", "bridge", "publisher"} {
		waitForBlockedHostFile(t, ctx,
			filepath.Join(fixture.root, "sync", role, "resource-cleanup-captured"))
	}
	for _, role := range []string{"endpoint", "bridge", "publisher"} {
		writeLiveFile(t, filepath.Join(fixture.root, "sync", role, "resource-stop"), []byte("stop\n"))
		waitBlockedContainer(t, ctx, compose, role+"-resource-collector")
	}
	windowFinished := result.WindowEndsMillis[len(result.WindowEndsMillis)-1]
	resources, err := mergeFinalBridgeHelperResources(filepath.Join(fixture.root, "sync", "bridge", "resource.jsonl"),
		result.Resources, result.StartedOffsetMillis, windowFinished)
	if err != nil {
		t.Fatal(err)
	}
	result.Resources = resources
	if err := admitFinalPublisherResources(filepath.Join(fixture.root, "sync", "publisher", "resource.jsonl"),
		result.StartedOffsetMillis, windowFinished); err != nil {
		t.Fatal(err)
	}
	resources, err = mergeFinalAdapterResources(filepath.Join(fixture.root, "sync", "endpoint", "resource.jsonl"),
		result.Resources, result.StartedOffsetMillis, windowFinished)
	if err != nil {
		t.Fatal(err)
	}
	result.Resources = resources
	for _, sample := range bridgeRuntimeSamples(t, ctx, compose) {
		result.Resources.GoroutinesPeak = max(result.Resources.GoroutinesPeak, uint16(sample.Goroutines))
		result.Resources.TimersPeak = max(result.Resources.TimersPeak, uint16(sample.Timers))
		result.Resources.QueueItemsPeak = max(result.Resources.QueueItemsPeak, uint16(sample.QueueItems))
		result.Resources.QueueBytesPeak = max(result.Resources.QueueBytesPeak, uint32(sample.QueueBytes))
	}
	result.Resources.Collected = finalResourceObservations()
	for _, role := range []string{"endpoint", "bridge", "publisher"} {
		writeLiveFile(t, filepath.Join(fixture.root, "sync", role, "resource-release"), []byte("release\n"))
	}
	for _, service := range blockedContainers("C1") {
		waitBlockedContainer(t, ctx, compose, service)
	}
	carrier := make(map[string][]blockedCarrierSample, 3)
	for _, role := range []string{"endpoint", "bridge", "publisher"} {
		writeLiveFile(t, filepath.Join(fixture.root, "sync", role, "carrier-stop"), []byte("stop\n"))
		waitBlockedContainer(t, ctx, compose, role+"-carrier-collector")
		samples, readErr := readBlockedCarrierSamples(filepath.Join(fixture.root, "sync", role, "carrier.jsonl"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		carrier[role] = samples
	}
	result.FinishedOffsetMillis = uint64(time.Since(timeline).Milliseconds())
	result.Resources = mergeFinalCarrierReserve(result.Resources, carrier["bridge"])
	cleanup()
	return result, finalCarrierDelta(carrier["endpoint"]), finalCarrierDelta(carrier["publisher"]), fixture.root
}

func assertFinalSustainedCell(t *testing.T, cell finalSustainedCellEvidence) {
	t.Helper()
	drift := math.Max(cell.DirectBeforeMbit, cell.DirectAfterMbit) /
		math.Min(cell.DirectBeforeMbit, cell.DirectAfterMbit)
	if cell.DirectBeforeMbit <= 0 || cell.DirectAfterMbit <= 0 || drift > 1.10 || len(cell.Runs) != 5 {
		t.Fatalf("final sustained direct pairing is invalid: %+v", cell)
	}
	var windows []float64
	for _, run := range cell.Runs {
		if !run.Complete || len(run.WindowsMbit) != 10 {
			t.Fatalf("final sustained run is incomplete: %+v", run)
		}
		windows = append(windows, run.WindowsMbit...)
		assertFinalResourceEvidence(t, run.Resources)
	}
	baseline := (cell.DirectBeforeMbit + cell.DirectAfterMbit) / 2
	if percentile(windows, .05) < math.Min(10, baseline/2) {
		t.Fatalf("final sustained p05 %.3f is below threshold %.3f", percentile(windows, .05), math.Min(10, baseline/2))
	}
	assertFiniteRatio(t, "Endpoint", cell.EndpointCarrierRatio)
	assertFiniteRatio(t, "publisher", cell.PublisherCarrierRatio)
}

func assertFinalResourceEvidence(t *testing.T, value finalResourceEvidence) {
	t.Helper()
	if !value.SamplesComplete || value.Samples < 590 || value.EndpointCPUMean > .5 ||
		value.EndpointCPUP95 > 1 || value.EndpointRSSP95MiB > 512 || value.BridgeCPUMean > 1.12 ||
		value.BridgeCPUP95 > 1.28 || value.BridgeMemoryP95MiB > 896 || value.HelperRSSP95MiB > 128 ||
		value.HelperFDPeak > 64 || value.HelperSocketPeak > 32 || value.SwapEvents != 0 || value.OOMEvents != 0 ||
		value.ReservePercent < 20 {
		t.Fatalf("final sustained resource gate failed: %+v", value)
	}
}

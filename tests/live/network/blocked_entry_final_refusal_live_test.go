//go:build live

package network_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/streamworkload"
)

const finalPressureBytes = uint32(37_500_000)

type finalPressureEvidence struct {
	Schema               string                        `json:"schema"`
	ID                   string                        `json:"id"`
	Terminal             string                        `json:"terminal"`
	BaselineSockets      uint16                        `json:"baseline_sockets,omitempty"`
	Injected             uint16                        `json:"injected,omitempty"`
	PeakSockets          uint16                        `json:"peak_sockets,omitempty"`
	Offers               uint16                        `json:"offers,omitempty"`
	Refused              uint16                        `json:"refused,omitempty"`
	HighSamples          uint16                        `json:"high_samples,omitempty"`
	LowSamples           uint16                        `json:"low_samples,omitempty"`
	Batches              uint16                        `json:"batches,omitempty"`
	Units                uint16                        `json:"units,omitempty"`
	StreamMbit           uint16                        `json:"stream_mbit,omitempty"`
	DurationMillis       uint32                        `json:"duration_millis,omitempty"`
	CadenceMillis        uint32                        `json:"cadence_millis,omitempty"`
	PartialBytes         uint16                        `json:"partial_bytes,omitempty"`
	RatePerSecond        uint16                        `json:"rate_per_second,omitempty"`
	MaximumRefusalMillis uint32                        `json:"maximum_refusal_millis,omitempty"`
	ExitMillis           uint32                        `json:"exit_millis,omitempty"`
	Progress             bool                          `json:"progress"`
	Protect              bool                          `json:"protect"`
	Drain                bool                          `json:"drain"`
	Normal               bool                          `json:"normal"`
	Cleanup              bool                          `json:"cleanup"`
	UpwardTrend          bool                          `json:"upward_trend"`
	OOMEvents            uint16                        `json:"oom_events,omitempty"`
	Residuals            uint16                        `json:"residuals,omitempty"`
	Reconciliations      []finalReconciliationEvidence `json:"reconciliations,omitempty"`
}

type finalReconciliationEvidence struct {
	Batch                uint16 `json:"batch"`
	AllocationDelta      int32  `json:"allocation_delta"`
	FDDelta              int32  `json:"fd_delta"`
	SocketDelta          int32  `json:"socket_delta"`
	GoroutineDelta       int32  `json:"goroutine_delta"`
	TimerDelta           int32  `json:"timer_delta"`
	StateBytesDelta      int64  `json:"state_bytes_delta"`
	EvidenceRecordsDelta int32  `json:"evidence_records_delta"`
	CleanupSockets       uint16 `json:"cleanup_sockets"`
	CleanupDescendants   uint16 `json:"cleanup_descendants"`
	CleanupStateBytes    uint64 `json:"cleanup_state_bytes"`
	Residuals            uint16 `json:"residuals"`
}

type finalRefusalBatch struct {
	admission blockedAdmissionResult
	progress  bool
	oom       uint16
	reconcile finalReconciliationEvidence
	root      string
}

func TestBlockedEntryFinalProjectedAdmissionAndChurn(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository, toolImage := repositoryRoot(t), liveToolImage(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s55-refusal-%d:test", time.Now().UnixNano()))
	buildFixture := newBlockedEntryFixture(t, client, server)
	buildProject := finalProjectName(fmt.Sprintf("ardents-s55-refusal-build-%d", time.Now().UnixNano()))
	build := blockedCompose(repository, buildProject, image, buildFixture, "final-pressure")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	if ownedImage {
		if output, err := build(ctx, "build", "endpoint"); err != nil {
			cancel()
			t.Fatalf("build final refusal image: %v\n%s", err, output)
		}
	}
	cancel()
	if ownedImage {
		t.Cleanup(func() { removeBlockedPressureImage(t, image, buildProject) })
	}
	if selected := os.Getenv("ARDENTS_FINAL_CELL"); selected != "" {
		runSelectedFinalRefusalCell(t, repository, image, toolImage, client, server, selected)
		return
	}
	firstStarted := time.Now()
	first := runFinalRefusalBatch(t, repository, image, toolImage, client, server, 0, 100)
	emitFinalPressure(t, finalPressureEvidence{Schema: "ardents-h3-final-pressure-v1", ID: "P0",
		Terminal: "normal", Units: 4, StreamMbit: 10, DurationMillis: 30_000,
		Progress: first.progress, Cleanup: true, OOMEvents: first.oom})
	emitFinalPressure(t, finalPressureEvidence{Schema: "ardents-h3-final-pressure-v1", ID: "P1",
		Terminal: "normal", Offers: 100, Refused: first.admission.Refused, CadenceMillis: 100,
		DurationMillis: 10_000, MaximumRefusalMillis: first.admission.MaximumMillis,
		Progress: first.progress, Cleanup: true, OOMEvents: first.oom})
	emitFinalWorkerCell(t, "pressure/P0", "normal", firstStarted, first.root)
	emitFinalWorkerCell(t, "pressure/P1", "normal", firstStarted, first.root)
	churn := finalPressureEvidence{Schema: "ardents-h3-final-pressure-v1", ID: "P4", Terminal: "normal",
		Offers: 1_000, CadenceMillis: 100, DurationMillis: 100_000, Batches: 10, Progress: true, Cleanup: true}
	churnStarted := time.Now()
	var churnRoots []string
	for batch := range 10 {
		observed := runFinalRefusalBatch(t, repository, image, toolImage, client, server, batch+1, 100)
		churnRoots = append(churnRoots, observed.root)
		churn.Refused += observed.admission.Refused
		churn.MaximumRefusalMillis = max(churn.MaximumRefusalMillis, observed.admission.MaximumMillis)
		churn.Progress = churn.Progress && observed.progress
		churn.OOMEvents += observed.oom
		observed.reconcile.Batch = uint16(batch)
		churn.Reconciliations = append(churn.Reconciliations, observed.reconcile)
	}
	churn.UpwardTrend = !exactLiveReconciliations(churn.Reconciliations)
	emitFinalPressure(t, churn)
	emitFinalWorkerCell(t, "pressure/P4", "normal", churnStarted, churnRoots...)
}

func runFinalRefusalBatch(t *testing.T, repository, image, toolImage, client, server string,
	batch, offers int,
) finalRefusalBatch {
	t.Helper()
	fixture := newBlockedEntryFixture(t, client, server)
	workCell, workLabel, offerCell, offerLabel := "pressure/P0", "established-work", "pressure/P1", "offers"
	if batch > 0 {
		workCell, offerCell = "pressure/P4", "pressure/P4"
		workLabel, offerLabel = fmt.Sprintf("batch-%02d-work", batch-1), fmt.Sprintf("batch-%02d-offers", batch-1)
	}
	bindFinalFixtureSeed(t, fixture, workCell, workLabel)
	bindFinalOfferSeed(t, fixture, offerCell, offerLabel)
	rewriteBlockedCapacity(t, fixture, 4)
	rewriteBlockedWorkload(t, fixture, "publisher-to-endpoint", finalPressureBytes)
	chunks := (uint64(finalPressureBytes) + 16_380) / 16_381
	delay := (30 * time.Second) / time.Duration(chunks-1)
	for name, value := range map[string]string{
		"ARDENTS_CAPACITY_WORKLOAD": "sustained", "ARDENTS_CAPACITY_SEND_BYTES": "0",
		"ARDENTS_CAPACITY_RECEIVE_BYTES": fmt.Sprint(finalPressureBytes),
		"ARDENTS_CAPACITY_CHUNK_DELAY":   delay.String(), "ARDENTS_BLOCKED_WORKLOAD": "sustained",
		"ARDENTS_BLOCKED_CLIENT_SEND_BYTES": "0", "ARDENTS_BLOCKED_CLIENT_RECEIVE_BYTES": fmt.Sprint(finalPressureBytes),
		"ARDENTS_BLOCKED_PUBLISHER_SEND_BYTES":    fmt.Sprint(finalPressureBytes),
		"ARDENTS_BLOCKED_PUBLISHER_RECEIVE_BYTES": "0", "ARDENTS_STREAM_PROGRESS": "1",
		"ARDENTS_STREAM_CHUNK_DELAY": delay.String(), "ARDENTS_STREAM_LIFETIME": "5m",
		"ARDENTS_CAPACITY_OFFERS": fmt.Sprint(offers), "ARDENTS_CAPACITY_CADENCE": "100ms"} {
		t.Setenv(name, value)
	}
	project := finalProjectName(fmt.Sprintf("ardents-s55-refusal-%02d-%d", batch, time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "final-pressure")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	t.Cleanup(func() { removeCapacityProjectObjects(t, project) })
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	startBlockedNetwork(t, ctx, compose, "C1")
	waitForBlockedJSON(t, ctx, compose, "bridge", adapterReady)
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		waitForKind(t, ctx, compose, role, "ready")
	}
	startLiveContainer(t, ctx, compose, "bridge-resource-collector")
	startLiveService(t, ctx, compose, "publisher-service", "publisher")
	runLiveOneShot(t, ctx, compose, "publication-operator")
	applyFinalBridgeInfrastructure(t, ctx, compose, toolImage)
	units := startBlockedCapacityUnits(t, ctx, project, image, toolImage, fixture, 4, workCell, workLabel)
	before := waitCapacityProgress(t, ctx, units, nil)
	beforeAdmission := waitBridgeAdmission(t, ctx, compose, func(sample resource.Sample) bool {
		return sample.AdmissionActive == 4
	})
	capturePressureResourceBoundary(t, ctx, fixture.root, "baseline")
	var admission blockedAdmissionResult
	afterAdmission := beforeAdmission
	if offers > 0 {
		if output, err := compose(ctx, "up", "-d", "--no-build", "--no-deps", "capacity-probe"); err != nil {
			t.Fatalf("start projected-admission probe: %v\n%s", err, output)
		}
		waitBlockedContainer(t, ctx, compose, "capacity-probe")
		readHostJSON(t, filepath.Join(fixture.root, "sync", "capacity-probe", "admission-result.json"), &admission)
		afterAdmission = waitBridgeAdmission(t, ctx, compose, func(sample resource.Sample) bool {
			return sample.AdmissionRefused >= beforeAdmission.AdmissionRefused+uint64(admission.Offers)
		})
	}
	capturePressureResourceBoundary(t, ctx, fixture.root, "after-churn")
	allocationDelta := int32(afterAdmission.AdmissionAccepted - beforeAdmission.AdmissionAccepted)
	after := waitCapacityProgress(t, ctx, units, before)
	for _, unit := range units {
		application := waitNamedApplication(t, ctx, unit.application)
		if application.Terminal != "success" || application.ReceivedBytes != finalPressureBytes ||
			application.DurationMillis < 29_900 || application.DurationMillis > 32_000 {
			t.Fatalf("pressure Application %s = %+v", unit.application, application)
		}
		waitNamedContainer(t, ctx, unit.endpoint)
		waitNamedContainer(t, ctx, unit.service)
		waitNamedContainer(t, ctx, unit.endpoint+"-observer")
		waitNamedContainer(t, ctx, unit.endpoint+"-policy")
	}
	for _, unit := range units {
		waitNamedContainer(t, ctx, unit.publisherApplication)
	}
	if result := waitForServiceResult(t, ctx, compose, "publisher-service"); result.RouteAttachmentsAccepted != 4 || result.ApplicationIPCAccepts != 4 {
		t.Fatalf("pressure publisher Service = %+v", result)
	}
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		if result := waitForKind(t, ctx, compose, role, "complete"); result.Terminal != "success" || !result.Cleanup {
			t.Fatalf("pressure %s Route = %+v", role, result)
		}
	}
	publishFinalWorkerTerminal()
	writeLiveFile(t, filepath.Join(fixture.root, "sync", "bridge", "bridge-stop"), []byte("stop\n"))
	waitForBlockedHostFile(t, ctx,
		filepath.Join(fixture.root, "sync", "bridge", "resource-cleanup-captured"))
	writeLiveFile(t, filepath.Join(fixture.root, "sync", "bridge", "resource-stop"), []byte("stop\n"))
	waitBlockedContainer(t, ctx, compose, "bridge-resource-collector")
	samples, oom := assertPressureResourceStream(t,
		filepath.Join(fixture.root, "sync", "bridge", "resource.jsonl"))
	reconciliation := reconcilePressureBatch(t, samples, beforeAdmission, afterAdmission, allocationDelta)
	writeLiveFile(t, filepath.Join(fixture.root, "sync", "bridge", "resource-release"), []byte("release\n"))
	for _, service := range []string{"bridge", "bridge-observer", "initiator-observer", "introduction-observer",
		"rendezvous-observer", "responder-observer", "publisher-observer", "publisher-service"} {
		waitBlockedContainer(t, ctx, compose, service)
	}
	removeCapacityProjectObjects(t, project)
	cleanup()
	reconciliation.Residuals = blockedProjectResiduals(t, ctx, project)
	return finalRefusalBatch{admission: admission, progress: allProgressAdvanced(before, after), oom: oom,
		reconcile: reconciliation, root: fixture.root}
}

func adapterReady(line []byte) bool {
	var value struct{ Kind, State string }
	return json.Unmarshal(line, &value) == nil && value.Kind == "adapter" && value.State == "READY"
}

func waitCapacityProgress(t *testing.T, ctx context.Context, units []blockedCapacityUnit,
	minimum map[string]uint32,
) map[string]uint32 {
	t.Helper()
	for {
		result := make(map[string]uint32, len(units))
		complete := true
		for _, unit := range units {
			logs, _ := dockerOutput(ctx, "logs", unit.application)
			result[unit.application] = latestLiveProgress(logs)
			if result[unit.application] == 0 || minimum != nil && result[unit.application] <= minimum[unit.application] {
				complete = false
			}
		}
		if complete {
			return result
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for four-unit progress: %v", ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func waitNamedApplication(t *testing.T, ctx context.Context, name string) streamworkload.Observation {
	t.Helper()
	for {
		logs, _ := dockerOutput(ctx, "logs", name)
		for _, line := range bytes.Split(logs, []byte{'\n'}) {
			var value streamworkload.Observation
			if json.Unmarshal(bytes.TrimSpace(line), &value) == nil && value.Schema == "ardents-h3-stream-application-v1" {
				return value
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s Application: %v", name, ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func waitNamedApplicationReady(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	for {
		logs, _ := dockerOutput(ctx, "logs", name)
		for _, line := range bytes.Split(logs, []byte{'\n'}) {
			var value struct {
				Schema string `json:"schema"`
			}
			if json.Unmarshal(bytes.TrimSpace(line), &value) == nil && value.Schema == "ardents-stream-ready-v1" {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s Application readiness: %v", name, ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func allProgressAdvanced(before, after map[string]uint32) bool {
	for name, value := range before {
		if after[name] <= value {
			return false
		}
	}
	return len(before) == 4
}

func assertPressureResourceStream(t *testing.T, path string) ([]blockedProcessSample, uint16) {
	t.Helper()
	samples, err := readBlockedProcessSamples(path)
	if err != nil || len(samples) < 30 || !hasPostCleanupSample(samples) {
		t.Fatalf("pressure resource stream: %v (%d samples)", err, len(samples))
	}
	var oom uint64
	for _, sample := range samples {
		if sample.RSSBytes > 128<<20 || sample.FDs > 64 || sample.Sockets > 32 || sample.SwapBytes != 0 {
			t.Fatalf("pressure helper resource gate = %+v", sample)
		}
		oom = max(oom, sample.EmergencyEvents)
	}
	return samples, uint16(oom)
}

func reconcilePressureBatch(t *testing.T, samples []blockedProcessSample,
	beforeAdmission, afterAdmission resource.Sample, allocationDelta int32,
) finalReconciliationEvidence {
	t.Helper()
	first := pressureBoundarySample(t, samples, "baseline")
	last := pressureBoundarySample(t, samples, "after-churn")
	cleanup := pressureBoundarySample(t, samples, "post-cleanup")
	return finalReconciliationEvidence{AllocationDelta: allocationDelta,
		FDDelta:              int32(last.FDs) - int32(first.FDs),
		SocketDelta:          int32(last.Sockets) - int32(first.Sockets),
		GoroutineDelta:       int32(afterAdmission.Goroutines) - int32(beforeAdmission.Goroutines),
		TimerDelta:           int32(afterAdmission.Timers) - int32(beforeAdmission.Timers),
		StateBytesDelta:      int64(last.StateBytes) - int64(first.StateBytes),
		EvidenceRecordsDelta: int32(last.EvidenceRecords) - int32(first.EvidenceRecords),
		CleanupSockets:       cleanup.Sockets,
		CleanupDescendants:   cleanupDescendants(cleanup.Processes),
		CleanupStateBytes:    cleanup.StateBytes}
}

func cleanupDescendants(processes uint16) uint16 {
	if processes > 1 {
		return processes - 1
	}
	return 0
}

func capturePressureResourceBoundary(t *testing.T, ctx context.Context, root, boundary string) {
	t.Helper()
	request := filepath.Join(root, "sync", "bridge", "resource-"+boundary)
	writeLiveFile(t, request, []byte("capture\n"))
	waitForBlockedHostFile(t, ctx, request+"-captured")
}

func pressureBoundarySample(t *testing.T, samples []blockedProcessSample, boundary string) blockedProcessSample {
	t.Helper()
	for _, sample := range samples {
		if sample.Boundary == boundary {
			return sample
		}
	}
	t.Fatalf("P4 resource boundary %q is missing", boundary)
	return blockedProcessSample{}
}

func waitBridgeAdmission(t *testing.T, ctx context.Context, compose composeCall,
	ready func(resource.Sample) bool,
) resource.Sample {
	t.Helper()
	for {
		samples := bridgeRuntimeSamples(t, ctx, compose)
		if len(samples) > 0 && ready(samples[len(samples)-1]) {
			return samples[len(samples)-1]
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for exact Bridge admission counters: %v", ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func blockedProjectResiduals(t *testing.T, ctx context.Context, project string) uint16 {
	t.Helper()
	var residuals uint16
	for _, arguments := range [][]string{
		{"ps", "-aq", "--filter", "label=com.docker.compose.project=" + project},
		{"network", "ls", "-q", "--filter", "label=com.docker.compose.project=" + project},
		{"volume", "ls", "-q", "--filter", "label=com.docker.compose.project=" + project},
	} {
		output, err := dockerOutput(ctx, arguments...)
		if err != nil {
			t.Fatalf("measure P4 residuals: %v", err)
		}
		residuals += uint16(len(strings.Fields(string(output))))
	}
	return residuals
}

func bridgeRuntimeSamples(t *testing.T, ctx context.Context, compose composeCall) []resource.Sample {
	t.Helper()
	output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", "bridge")
	if err != nil {
		t.Fatalf("read Bridge resource samples: %v\n%s", err, output)
	}
	var result []resource.Sample
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		var event struct {
			Kind     string           `json:"kind"`
			Resource *resource.Sample `json:"resource"`
		}
		if json.Unmarshal(bytes.TrimSpace(line), &event) == nil && event.Kind == "resource-sample" &&
			event.Resource != nil {
			result = append(result, *event.Resource)
		}
	}
	return result
}

func exactLiveReconciliations(values []finalReconciliationEvidence) bool {
	for index, value := range values {
		if value.Batch != uint16(index) || value.AllocationDelta != 0 || value.FDDelta != 0 ||
			value.SocketDelta != 0 || value.GoroutineDelta != 0 || value.TimerDelta != 0 ||
			value.StateBytesDelta != 0 || value.EvidenceRecordsDelta != 0 || value.CleanupSockets != 0 ||
			value.CleanupDescendants != 0 || value.CleanupStateBytes != 0 || value.Residuals != 0 {
			return false
		}
	}
	return true
}

func emitFinalPressure(t *testing.T, value finalPressureEvidence) {
	t.Helper()
	if value.Refused != value.Offers && value.ID != "P0" || !value.Progress || !value.Cleanup ||
		value.MaximumRefusalMillis > 1_000 || value.OOMEvents != 0 || value.Residuals != 0 || value.UpwardTrend {
		t.Fatalf("final pressure evidence failed: %+v", value)
	}
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		t.Fatal(err)
	}
}

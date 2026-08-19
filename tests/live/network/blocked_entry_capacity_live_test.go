//go:build live

package network_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type blockedAdmissionResult struct {
	Schema        string                  `json:"schema"`
	Offers        uint16                  `json:"offers"`
	Refused       uint16                  `json:"refused"`
	MaximumMillis uint32                  `json:"maximum_millis"`
	Outcomes      []blockedAdmissionOffer `json:"outcomes"`
}

type blockedAdmissionOffer struct {
	Ordinal               uint16 `json:"ordinal"`
	CanarySHA256          string `json:"canary_sha256"`
	ScheduledOffsetMillis uint32 `json:"scheduled_offset_millis"`
	StartedOffsetMillis   uint32 `json:"started_offset_millis"`
	RefusalMillis         uint32 `json:"refusal_millis"`
	Refused               bool   `json:"refused"`
}

func TestBlockedEntryFinalReferenceAndStrongCapacity(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository := repositoryRoot(t)
	toolImage := liveToolImage(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s55-capacity-%d:test", time.Now().UnixNano()))
	buildFixture := newBlockedEntryFixture(t, client, server)
	buildProject := finalProjectName(fmt.Sprintf("ardents-s55-capacity-build-%d", time.Now().UnixNano()))
	build := blockedCompose(repository, buildProject, image, buildFixture, "final-capacity")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	if ownedImage {
		if output, err := build(ctx, "build", "endpoint"); err != nil {
			cancel()
			t.Fatalf("build final capacity image: %v\n%s", err, output)
		}
	}
	cancel()
	if ownedImage {
		t.Cleanup(func() { removeBlockedPressureImage(t, image, buildProject) })
	}
	for _, profile := range []struct {
		name     string
		capacity int
	}{{"reference", 4}, {"strong", 16}} {
		for batch := range 5 {
			t.Run(fmt.Sprintf("%s-%d", profile.name, batch), func(t *testing.T) {
				profileID := "h3-s5-b1-v1"
				if profile.name == "strong" {
					profileID = "h3-s5-b1-v1-strong"
				}
				cell := fmt.Sprintf("capacity/%s/%d", profileID, batch)
				if !selectedFinalCell(cell) {
					return
				}
				started := time.Now()
				armFinalWorkerTerminal("complete")
				root, measurement := runBlockedCapacityBatch(t, repository, image, toolImage, client, server,
					profile.name, profile.capacity, batch)
				emitFinalWorkerCapacity(t, cell, started, measurement, root)
			})
		}
	}
}

func runBlockedCapacityBatch(t *testing.T, repository, image, toolImage, client, server, profile string,
	capacity, batch int,
) (string, finalWorkerCapacity) {
	t.Helper()
	fixture := newBlockedEntryFixture(t, client, server)
	profileID := "h3-s5-b1-v1"
	if profile == "strong" {
		profileID = "h3-s5-b1-v1-strong"
	}
	cell := fmt.Sprintf("capacity/%s/%d", profileID, batch)
	bindFinalFixtureSeed(t, fixture, cell, "useful-work")
	bindFinalOfferSeed(t, fixture, cell, "bounded-extra-offer")
	rewriteBlockedCapacity(t, fixture, uint16(capacity))
	addresses := make([]string, capacity)
	for index := range capacity {
		addresses[index] = fmt.Sprintf("203.0.113.%d", 40+index)
	}
	t.Setenv("ARDENTS_BLOCKED_ENDPOINT_ADDRESSES", strings.Join(addresses, ","))
	t.Setenv("ARDENTS_STREAM_CHUNK_DELAY", "1s")
	t.Setenv("ARDENTS_CAPACITY_OFFERS", "1")
	t.Setenv("ARDENTS_CAPACITY_CADENCE", "0s")
	if profile == "strong" {
		for name, value := range map[string]string{"ARDENTS_BRIDGE_CPUS": "6.4", "ARDENTS_BRIDGE_MEMORY_LIMIT": "5120m",
			"ARDENTS_BRIDGE_MEMORY_RESERVATION": "4608m", "ARDENTS_BRIDGE_PIDS": "2048",
			"ARDENTS_BRIDGE_GOMAXPROCS": "6", "ARDENTS_BRIDGE_GOMEMLIMIT": "3072MiB"} {
			t.Setenv(name, value)
		}
	}
	project := finalProjectName(fmt.Sprintf("ardents-s55-cap-%s-%d-%d", profile, batch, time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "final-capacity")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	t.Cleanup(func() { removeCapacityProjectObjects(t, project) })
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	startBlockedNetwork(t, ctx, compose, "C1")
	waitForBlockedJSON(t, ctx, compose, "bridge", func(line []byte) bool {
		var value struct{ Kind, State string }
		return json.Unmarshal(line, &value) == nil && value.Kind == "adapter" && value.State == "READY"
	})
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		waitForKind(t, ctx, compose, role, "ready")
	}
	startLiveService(t, ctx, compose, "publisher-service", "publisher")
	runLiveOneShot(t, ctx, compose, "publication-operator")
	if output, err := compose(ctx, "up", "-d", "--no-build", "--no-deps", "--scale",
		"publisher-app="+fmt.Sprint(capacity), "publisher-app"); err != nil {
		t.Fatalf("start capacity publisher Applications: %v\n%s", err, output)
	}
	bridgeRate := "100mbit"
	if profile == "strong" {
		bridgeRate = "400mbit"
	}
	applyFinalBridgeInfrastructure(t, ctx, compose, toolImage, bridgeRate)
	for _, service := range []string{"bridge-resource-collector", "bridge-carrier-collector",
		"publisher-resource-collector", "publisher-carrier-collector"} {
		startLiveContainer(t, ctx, compose, service)
	}
	for _, role := range []string{"bridge", "publisher"} {
		waitForBlockedHostFile(t, ctx, filepath.Join(fixture.root, "sync", role, "resource-ready"))
		waitForBlockedHostFile(t, ctx, filepath.Join(fixture.root, "sync", role, "carrier-ready"))
		writeLiveFile(t, filepath.Join(fixture.root, "sync", role, "carrier-start"), []byte("start\n"))
		waitForBlockedHostFile(t, ctx, filepath.Join(fixture.root, "sync", role, "carrier-started"))
	}
	units := startBlockedCapacityUnits(t, ctx, project, image, toolImage, fixture, capacity, "", "")
	waitForBridgeSocketSamples(t, ctx, compose, uint64(2+4*capacity), 1)
	timeline, timelineErr := finalWorkerTimelineOrigin()
	if timelineErr != nil {
		t.Fatal(timelineErr)
	}
	endpointTrees := make([]finalTreeSample, 0, len(units))
	for _, unit := range units {
		endpointTrees = append(endpointTrees, writeFinalCapacityTreeSnapshot(t, ctx,
			filepath.Join(unit.sync, "tree.jsonl"), timeline, unit.endpoint, unit.service, unit.application))
	}
	bridgeTree := writeFinalCapacityTreeSnapshot(t, ctx,
		filepath.Join(fixture.root, "sync", "bridge", "tree.jsonl"), timeline,
		finalComposeContainerIDs(t, ctx, compose, "bridge")...)
	publisherTree := writeFinalCapacityTreeSnapshot(t, ctx,
		filepath.Join(fixture.root, "sync", "publisher", "tree.jsonl"), timeline,
		finalComposeContainerIDs(t, ctx, compose, "publisher", "publisher-service", "publisher-app")...)
	if output, err := compose(ctx, "up", "-d", "--no-build", "--no-deps", "capacity-probe"); err != nil {
		t.Fatalf("start capacity refusal probe: %v\n%s", err, output)
	}
	waitBlockedContainer(t, ctx, compose, "capacity-probe")
	resultPath := filepath.Join(fixture.root, "sync", "capacity-probe", "admission-result.json")
	waitForBlockedHostFile(t, ctx, resultPath)
	var admission blockedAdmissionResult
	readHostJSON(t, resultPath, &admission)
	if admission.Offers != 1 || admission.Refused != 1 || admission.MaximumMillis > 1_000 {
		t.Fatalf("bounded %d+1 refusal = %+v", capacity, admission)
	}
	waitScaledComposeService(t, ctx, compose, "publisher-app", capacity)
	if result := waitForServiceResult(t, ctx, compose, "publisher-service"); result.Class != "clean service connection close" ||
		result.RouteAttachmentsAccepted != uint32(capacity) || result.ApplicationIPCAccepts != uint32(capacity) {
		t.Fatalf("publisher capacity result = %+v", result)
	}
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		if result := waitForKind(t, ctx, compose, role, "complete"); result.Terminal != "success" ||
			result.AttachmentsCompleted != uint16(capacity) || !result.Cleanup {
			t.Fatalf("%s capacity result = %+v", role, result)
		}
	}
	publishFinalWorkerTerminal()
	writeLiveFile(t, filepath.Join(fixture.root, "sync", "bridge", "bridge-stop"), []byte("stop\n"))
	resourceRoles := []string{"bridge", "publisher"}
	responseDurations := make([]time.Duration, 0, len(units))
	for _, unit := range units {
		resourceRoles = append(resourceRoles, filepath.Base(unit.sync))
	}
	for _, role := range resourceRoles {
		waitForBlockedHostFile(t, ctx, filepath.Join(fixture.root, "sync", role, "resource-cleanup-captured"))
		writeLiveFile(t, filepath.Join(fixture.root, "sync", role, "resource-stop"), []byte("stop\n"))
	}
	for _, service := range []string{"bridge-resource-collector", "publisher-resource-collector"} {
		waitBlockedContainer(t, ctx, compose, service)
	}
	for _, unit := range units {
		waitNamedContainer(t, ctx, unit.endpoint+"-resource-collector")
	}
	for _, role := range resourceRoles {
		writeLiveFile(t, filepath.Join(fixture.root, "sync", role, "resource-release"), []byte("release\n"))
	}
	for _, unit := range units {
		for _, name := range []string{unit.endpoint, unit.service, unit.application,
			unit.endpoint + "-observer", unit.endpoint + "-policy"} {
			waitNamedContainer(t, ctx, name)
		}
		assertContainerDuration(t, ctx, unit.endpoint, unit.released, 8*time.Second)
		responseDurations = append(responseDurations, finalContainerDuration(t, ctx, unit.endpoint, unit.released))
	}
	for _, service := range []string{"bridge", "bridge-observer", "initiator-observer", "introduction-observer",
		"rendezvous-observer", "responder-observer", "publisher-observer", "publisher-service"} {
		waitBlockedContainer(t, ctx, compose, service)
	}
	for _, role := range resourceRoles {
		writeLiveFile(t, filepath.Join(fixture.root, "sync", role, "carrier-stop"), []byte("stop\n"))
	}
	for _, service := range []string{"bridge-carrier-collector", "publisher-carrier-collector"} {
		waitBlockedContainer(t, ctx, compose, service)
	}
	for _, unit := range units {
		waitNamedContainer(t, ctx, unit.endpoint+"-carrier-collector")
	}
	measurement := finalCapacityMeasurement(t, fixture.root, profileID, batch, admission,
		endpointTrees, bridgeTree, publisherTree, responseDurations, bridgeRuntimeSamples(t, ctx, compose))
	removeCapacityProjectObjects(t, project)
	cleanup()
	return fixture.root, measurement
}

func readHostJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, value) != nil {
		t.Fatalf("read %s: %v", filepath.Base(path), err)
	}
}

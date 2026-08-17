//go:build live

package network_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestBlockedEntryDrainsAtExactEmergencySocketPressure(t *testing.T) {
	started := time.Now()
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository := repositoryRoot(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s55-p3-%d:test", time.Now().UnixNano()))
	fixture := newBlockedEntryFixture(t, client, server)
	bindFinalFixtureSeed(t, fixture, "pressure/P3", "established-work")
	bindFinalPressureSeed(t, fixture, "pressure/P3", "partial-handshakes")
	const transferBytes = uint32(32 << 20)
	rewriteBlockedWorkload(t, fixture, "endpoint-to-publisher", transferBytes)
	for name, value := range map[string]string{
		"ARDENTS_BLOCKED_WORKLOAD":                "sustained",
		"ARDENTS_BLOCKED_CLIENT_SEND_BYTES":       fmt.Sprint(transferBytes),
		"ARDENTS_BLOCKED_CLIENT_RECEIVE_BYTES":    "0",
		"ARDENTS_BLOCKED_PUBLISHER_SEND_BYTES":    "0",
		"ARDENTS_BLOCKED_PUBLISHER_RECEIVE_BYTES": fmt.Sprint(transferBytes),
		"ARDENTS_STREAM_PROGRESS":                 "1",
		"ARDENTS_STREAM_CHUNK_DELAY":              "100ms",
		"ARDENTS_STREAM_LIFETIME":                 "15m",
		"ARDENTS_PRESSURE_CONNECTIONS":            "23",
		"ARDENTS_BLOCKED_EXPECT_DRAIN":            "1",
	} {
		t.Setenv(name, value)
	}
	project := finalProjectName(fmt.Sprintf("ardents-s55-p3-%d", time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "final-pressure")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if ownedImage {
		if output, err := compose(ctx, "build", "endpoint"); err != nil {
			t.Fatalf("build final emergency-pressure image: %v\n%s", err, output)
		}
	}
	startBlockedPressureWork(t, ctx, compose)
	waitForBridgeSocketSamples(t, ctx, compose, 6, 1)
	progress := waitForLiveProgressAbove(t, ctx, compose, "publisher-app", 0)
	if output, err := compose(ctx, "up", "-d", "--no-build", "--no-deps", "pressure"); err != nil {
		t.Fatalf("start emergency partial-handshake collector: %v\n%s", err, output)
	}
	waitForBlockedHostFile(t, ctx, filepath.Join(fixture.root, "sync", "pressure", "pressure-ready"))
	waitForBridgeSocketSamples(t, ctx, compose, 29, 1)
	drainObserved := time.Now()
	armFinalWorkerTerminal("drain")
	waitForBridgeResourceState(t, ctx, compose, "DRAIN")
	publishFinalWorkerTerminal()
	waitForBridgeResourceState(t, ctx, compose, "EXIT")
	waitBlockedContainer(t, ctx, compose, "bridge")
	if elapsed := time.Since(drainObserved); elapsed > 60*time.Second {
		t.Fatalf("emergency Bridge exit took %s", elapsed)
	}
	if bridgeHasResourceState(t, ctx, compose, "NORMAL") || bridgeHasOOMEvent(t, ctx, compose) {
		t.Fatal("emergency socket pressure recovered to NORMAL or recorded OOM")
	}
	if latestLiveProgressForService(t, ctx, compose, "publisher-app") <= progress {
		t.Fatal("established work did not progress before emergency drain")
	}
	writeLiveFile(t, filepath.Join(fixture.root, "sync", "pressure", "pressure-release"), []byte("release\n"))
	waitBlockedContainer(t, ctx, compose, "pressure")
	for _, service := range blockedContainers("C1") {
		waitBlockedContainer(t, ctx, compose, service)
	}
	cleanup()
	if ownedImage {
		removeBlockedPressureImage(t, image, project)
	}
	emitFinalWorkerCell(t, "pressure/P3", "drain", started, fixture.root)
}

func latestLiveProgressForService(t *testing.T, ctx context.Context, compose composeCall, service string) uint32 {
	t.Helper()
	output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", service)
	if err != nil {
		t.Fatalf("read %s progress: %v\n%s", service, err, output)
	}
	return latestLiveProgress(output)
}

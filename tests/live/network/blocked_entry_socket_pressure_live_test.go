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
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestBlockedEntryReturnsFromExactRecoverableSocketPressure(t *testing.T) {
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
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s55-p2-%d:test", time.Now().UnixNano()))
	fixture := newBlockedEntryFixture(t, client, server)
	bindFinalFixtureSeed(t, fixture, "pressure/P2", "established-work")
	bindFinalPressureSeed(t, fixture, "pressure/P2", "partial-handshakes")
	const transferBytes = uint32(32 << 20)
	rewriteBlockedWorkload(t, fixture, "endpoint-to-publisher", transferBytes)
	t.Setenv("ARDENTS_BLOCKED_WORKLOAD", "sustained")
	t.Setenv("ARDENTS_BLOCKED_CLIENT_SEND_BYTES", fmt.Sprint(transferBytes))
	t.Setenv("ARDENTS_BLOCKED_CLIENT_RECEIVE_BYTES", "0")
	t.Setenv("ARDENTS_BLOCKED_PUBLISHER_SEND_BYTES", "0")
	t.Setenv("ARDENTS_BLOCKED_PUBLISHER_RECEIVE_BYTES", fmt.Sprint(transferBytes))
	t.Setenv("ARDENTS_STREAM_PROGRESS", "1")
	t.Setenv("ARDENTS_STREAM_CHUNK_DELAY", "100ms")
	t.Setenv("ARDENTS_STREAM_LIFETIME", "15m")
	t.Setenv("ARDENTS_PRESSURE_CONNECTIONS", "20")
	project := finalProjectName(fmt.Sprintf("ardents-s55-p2-%d", time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "final-pressure")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	if ownedImage {
		if output, err := compose(ctx, "build", "endpoint"); err != nil {
			t.Fatalf("build final pressure image: %v\n%s", err, output)
		}
	}
	startBlockedPressureWork(t, ctx, compose)
	waitForBridgeSocketSamples(t, ctx, compose, 6, 1)
	progress := waitForLiveProgressAbove(t, ctx, compose, "publisher-app", 0)
	if output, err := compose(ctx, "up", "-d", "--no-build", "--no-deps", "pressure"); err != nil {
		t.Fatalf("start partial-handshake collector: %v\n%s", err, output)
	}
	waitForBlockedHostFile(t, ctx, filepath.Join(fixture.root, "sync", "pressure", "pressure-ready"))
	waitForBridgeSocketSamples(t, ctx, compose, 26, 3)
	waitForBridgeResourceState(t, ctx, compose, "PROTECT")
	waitForLiveProgressAbove(t, ctx, compose, "publisher-app", progress)
	writeLiveFile(t, filepath.Join(fixture.root, "sync", "pressure", "pressure-release"), []byte("release\n"))
	waitBlockedContainer(t, ctx, compose, "pressure")
	waitForBridgeResourceState(t, ctx, compose, "NORMAL")
	if bridgeHasResourceState(t, ctx, compose, "DRAIN") {
		t.Fatal("recoverable socket pressure entered DRAIN")
	}
	armFinalWorkerTerminal("normal")
	finishBlockedPressureWork(t, ctx, compose, fixture, transferBytes)
	cleanup()
	if ownedImage {
		removeBlockedPressureImage(t, image, project)
	}
	emitFinalWorkerCell(t, "pressure/P2", "normal", started, fixture.root)
}

func waitForBlockedHostFile(t *testing.T, ctx context.Context, path string) {
	t.Helper()
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s: %v", filepath.Base(path), ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func removeBlockedPressureImage(t *testing.T, image, project string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if output, err := dockerOutput(ctx, "image", "rm", "--force", image); err != nil {
		t.Fatalf("remove final pressure image: %v\n%s", err, output)
	}
	assertNoDockerObjects(t, ctx, project, image)
}

func waitForBridgeSocketSamples(t *testing.T, ctx context.Context, compose composeCall, sockets uint64, count int) {
	t.Helper()
	for {
		observed := 0
		output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", "bridge")
		if err == nil {
			for _, line := range bytes.Split(output, []byte{'\n'}) {
				var event struct {
					Kind     string           `json:"kind"`
					Resource *resource.Sample `json:"resource"`
				}
				if json.Unmarshal(bytes.TrimSpace(line), &event) == nil && event.Kind == "resource-sample" &&
					event.Resource != nil && event.Resource.Sockets == sockets {
					observed++
				}
			}
			if observed >= count {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %d Bridge samples at %d sockets: %v\n%s", count, sockets, ctx.Err(), output)
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func waitForBridgeResourceState(t *testing.T, ctx context.Context, compose composeCall, state string) {
	t.Helper()
	for !bridgeHasResourceState(t, ctx, compose, state) {
		select {
		case <-ctx.Done():
			t.Fatalf("wait for Bridge resource state %s: %v", state, ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func bridgeHasResourceState(t *testing.T, ctx context.Context, compose composeCall, state string) bool {
	t.Helper()
	output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", "bridge")
	if err != nil {
		return false
	}
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		var event struct{ Kind, State string }
		if json.Unmarshal(bytes.TrimSpace(line), &event) == nil && event.Kind == "resource" && event.State == state {
			return true
		}
	}
	return false
}

func bridgeHasOOMEvent(t *testing.T, ctx context.Context, compose composeCall) bool {
	t.Helper()
	output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", "bridge")
	if err != nil {
		return false
	}
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		var event struct {
			Kind     string           `json:"kind"`
			Resource *resource.Sample `json:"resource"`
		}
		if json.Unmarshal(bytes.TrimSpace(line), &event) == nil && event.Kind == "resource-sample" &&
			event.Resource != nil && event.Resource.EmergencyEvents != 0 {
			return true
		}
	}
	return false
}

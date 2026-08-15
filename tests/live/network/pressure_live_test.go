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

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestContainerRoutePreservesEstablishedWorkThenDrainsUnderMeasuredCgroupPressure(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	repository := repositoryRoot(t)
	base := newLiveFixture(t)
	const transferBytes = uint32(32 << 20)
	newLiveServiceFixture(t, base, "client-to-publisher", transferBytes)
	project := fmt.Sprintf("ardents-live-pressure-%d", time.Now().UnixNano())
	image := project + ":test"
	environment := append(os.Environ(), "ARDENTS_LIVE_IMAGE="+image,
		"ARDENTS_LIVE_ROOT="+filepath.ToSlash(base.root),
		"ARDENTS_LIVE_CLIENT_SEND_BYTES="+fmt.Sprint(transferBytes), "ARDENTS_LIVE_CLIENT_RECEIVE_BYTES=0",
		"ARDENTS_LIVE_PUBLISHER_SEND_BYTES=0", "ARDENTS_LIVE_PUBLISHER_RECEIVE_BYTES="+fmt.Sprint(transferBytes),
		"ARDENTS_LIVE_CHUNK_DELAY=25ms")
	compose := func(ctx context.Context, arguments ...string) ([]byte, error) {
		base := []string{"compose", "-p", project, "-f", filepath.Join(repository, "tests", "live", "network.compose.yaml")}
		command := exec.CommandContext(ctx, "docker", append(base, arguments...)...)
		command.Dir, command.Env = repository, environment
		return command.CombinedOutput()
	}
	cleanup := strictLiveCleanup(t, compose, project, image)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if output, err := compose(ctx, "build", "publisher"); err != nil {
		t.Fatalf("build Route pressure image: %v\n%s", err, output)
	}
	passive := []string{"publisher", "responder", "rendezvous", "introduction", "initiator"}
	if output, err := compose(ctx, append([]string{"up", "-d", "--no-build"}, passive...)...); err != nil {
		t.Fatalf("start pressure Route: %v\n%s", err, output)
	}
	for _, service := range passive {
		waitForKind(t, ctx, compose, service, "ready")
	}
	startLiveService(t, ctx, compose, "publisher-service", "publisher")
	runLiveOneShot(t, ctx, compose, "publication-operator")
	startLiveService(t, ctx, compose, "client-service", "client")
	startLiveContainer(t, ctx, compose, "publisher-app")
	startLiveContainer(t, ctx, compose, "client-app")
	startLiveContainer(t, ctx, compose, "client")
	progress := waitForLiveProgressAbove(t, ctx, compose, "publisher-app", 0)

	runPressureMemory(t, ctx, compose, 392<<20, "60s", "172.31.20.11:4601")
	waitForResourceState(t, ctx, compose, "initiator", "PROTECT")
	runPressureMemory(t, ctx, compose, 1<<20, "10s", "172.31.20.11:4601")
	waitForLiveProgressAbove(t, ctx, compose, "publisher-app", progress)
	runPressureMemory(t, ctx, compose, 72<<20, "30s", "")
	drain := waitForResourceState(t, ctx, compose, "initiator", "DRAIN")
	if drain.Resource == nil || drain.Resource.MemoryBytes < 460<<20 && drain.Resource.EmergencyEvents == 0 {
		t.Fatalf("Route drained without measured emergency pressure: %+v", drain.Resource)
	}
	waitForResourceState(t, ctx, compose, "initiator", "EXIT")
	terminal := waitForKind(t, ctx, compose, "initiator", "complete")
	if terminal.Terminal != "error" || terminal.AttachmentsRefused == 0 || terminal.AttachmentsAbandoned == 0 {
		t.Fatalf("pressure terminal evidence omitted hostile work or drain: %+v", terminal)
	}
	waitForContainerStopped(t, ctx, compose, "initiator")
	cleanup()
}

func runPressureMemory(t *testing.T, ctx context.Context, compose composeCall, bytes int, duration, connect string) {
	t.Helper()
	arguments := []string{"exec", "-d", "-T", "initiator", "/usr/local/bin/carrier-lab", "pressure-memory",
		"-bytes", fmt.Sprint(bytes), "-duration", duration}
	if connect != "" {
		arguments = append(arguments, "-connect", connect)
	}
	if output, err := compose(ctx, arguments...); err != nil {
		t.Fatalf("apply measured cgroup pressure: %v\n%s", err, output)
	}
}

func waitForLiveProgressAbove(t *testing.T, ctx context.Context, compose composeCall, service string, previous uint32) uint32 {
	t.Helper()
	for {
		output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", "--tail", "64", service)
		if err == nil {
			if current := latestLiveProgress(output); current > previous {
				return current
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for established progress after %d bytes: %v\n%s", previous, ctx.Err(), output)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func waitForResourceState(t *testing.T, ctx context.Context, compose composeCall, service, state string) route.Evidence {
	t.Helper()
	for {
		output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", service)
		if err == nil {
			for _, line := range bytes.Split(output, []byte{'\n'}) {
				var evidence route.Evidence
				if json.Unmarshal(bytes.TrimSpace(line), &evidence) == nil &&
					evidence.Kind == "resource" && evidence.State == state {
					return evidence
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s resource state %s: %v\n%s", service, state, ctx.Err(), output)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func waitForContainerStopped(t *testing.T, ctx context.Context, compose composeCall, service string) {
	t.Helper()
	for {
		output, err := compose(ctx, "ps", "-q", "--status", "running", service)
		if err == nil && strings.TrimSpace(string(output)) == "" {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s runtime cleanup: %v\n%s", service, ctx.Err(), output)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

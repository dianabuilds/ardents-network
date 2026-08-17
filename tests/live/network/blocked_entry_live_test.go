//go:build live

package network_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
	"github.com/dianabuilds/ardents-network/internal/streamworkload"
)

const blockedClientHash = "de581c8dd36193bb4168aee840406294af406bf8187817c10ac2bcd9464fd120"
const blockedServerHash = "5fe32f8ab736ed54fc66027775761084e68f0e1ec9b5fea7c3417c6617255336"

func TestBlockedEntryCommandsAcrossNamespaces(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository := repositoryRoot(t)
	image := fmt.Sprintf("ardents-s53-%d:test", time.Now().UnixNano())
	fixture := newBlockedEntryFixture(t, client, server)
	buildProject := fmt.Sprintf("ardents-s53-build-%d", time.Now().UnixNano())
	build := blockedCompose(repository, buildProject, image, fixture)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	if output, err := build(ctx, "build", "endpoint"); err != nil {
		t.Fatalf("build blocked-entry image: %v\n%s", err, output)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		if output, err := dockerOutput(cleanupCtx, "image", "rm", "--force", image); err != nil {
			t.Errorf("remove blocked-entry image: %v\n%s", err, output)
		}
		assertNoDockerObjects(t, cleanupCtx, buildProject, image)
	})
	for _, profile := range []string{"C0", "C1", "C2", "C5", "C6"} {
		t.Run(profile, func(t *testing.T) {
			for episode := range 20 {
				t.Run(fmt.Sprint(episode), func(t *testing.T) {
					fixture = newBlockedEntryFixture(t, client, server)
					cell := fmt.Sprintf("profile/%s/%02d", profile, episode)
					if !selectedFinalCell(cell) {
						return
					}
					started := time.Now()
					bindFinalFixtureSeed(t, fixture, cell, "short-workload")
					if profile == "C5" || profile == "C6" {
						bindFinalProbeSeed(t, fixture, cell, "probe-corpus")
					}
					runBlockedEntryEpisode(t, repository, image, fixture, profile, episode)
					terminal := "success"
					if profile == "C5" {
						terminal = "probe-contained"
					} else if profile == "C6" {
						terminal = "limitation-recorded"
					}
					emitFinalWorkerCell(t, cell, terminal, started)
				})
			}
		})
	}
}

func runBlockedEntryEpisode(t *testing.T, repository, image string, fixture blockedEntryFixture,
	profile string, episode int,
) {
	t.Helper()
	project := fmt.Sprintf("ardents-s53-%s-%d-%d", strings.ToLower(profile), episode, time.Now().UnixNano())
	compose := blockedCompose(repository, project, image, fixture, profile)
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	startBlockedNetwork(t, ctx, compose, profile)
	if profile != "C0" {
		waitForBlockedJSON(t, ctx, compose, "bridge", func(line []byte) bool {
			var value struct{ Kind, State string }
			return json.Unmarshal(line, &value) == nil && value.Kind == "adapter" && value.State == "READY"
		})
	}
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		waitForKind(t, ctx, compose, role, "ready")
	}
	startLiveService(t, ctx, compose, "publisher-service", "publisher")
	runLiveOneShot(t, ctx, compose, "publication-operator")
	startLiveService(t, ctx, compose, "client-service", "client")
	startLiveContainer(t, ctx, compose, "publisher-app")
	startLiveContainer(t, ctx, compose, "client-app")
	endpointServices := []string{"up", "-d", "--no-build", "--no-deps", "endpoint", "endpoint-observer"}
	if profile != "C0" {
		endpointServices = append(endpointServices, "policy")
	}
	if output, err := compose(ctx, endpointServices...); err != nil {
		t.Fatalf("start blocked Endpoint: %v\n%s", err, output)
	}
	if profile != "C0" {
		waitForBlockedJSON(t, ctx, compose, "policy", func(line []byte) bool {
			var value struct{ Kind, State string }
			return json.Unmarshal(line, &value) == nil && value.Kind == "policy" && value.State == "READY"
		})
	}
	clientRoute := waitForKind(t, ctx, compose, "endpoint", "complete")
	if clientRoute.Terminal != "success" || !clientRoute.PeerAuthenticated || !clientRoute.Cleanup {
		t.Fatalf("blocked Endpoint Route = %+v", clientRoute)
	}
	clientApp := waitForApplication(t, ctx, compose, "client-app")
	publisherApp := waitForApplication(t, ctx, compose, "publisher-app")
	clientService := waitForServiceResult(t, ctx, compose, "client-service")
	publisherService := waitForServiceResult(t, ctx, compose, "publisher-service")
	assertBlockedApplication(t, fixture, clientApp, publisherApp, clientService, publisherService)
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		result := waitForKind(t, ctx, compose, role, "complete")
		if result.Terminal != "success" || !result.Cleanup {
			t.Fatalf("blocked %s Route = %+v", role, result)
		}
	}
	if profile == "C5" || profile == "C6" {
		if output, err := compose(ctx, "up", "-d", "--no-build", "--no-deps", "probe", "probe-observer"); err != nil {
			t.Fatalf("start external probes: %v\n%s", err, output)
		}
		waitBlockedContainer(t, ctx, compose, "probe")
		waitBlockedContainer(t, ctx, compose, "probe-observer")
	}
	if profile != "C0" {
		writeLiveFile(t, filepath.Join(fixture.root, "sync", "bridge", "bridge-stop"), []byte("stop\n"))
	}
	for _, service := range blockedContainers(profile) {
		waitBlockedContainer(t, ctx, compose, service)
	}
	cleanup()
}

func blockedCompose(repository, project, image string, fixture blockedEntryFixture, profile ...string) composeCall {
	selected := "C2"
	if len(profile) > 0 {
		selected = profile[0]
	}
	productProfile := selected
	if selected == "C5" || selected == "C6" {
		productProfile = "C2"
	}
	environment := append(os.Environ(), "ARDENTS_BLOCKED_IMAGE="+image,
		"ARDENTS_BLOCKED_ROOT="+filepath.ToSlash(fixture.root),
		"ARDENTS_WEBTUNNEL_CLIENT="+filepath.ToSlash(fixture.clientBinary),
		"ARDENTS_WEBTUNNEL_SERVER="+filepath.ToSlash(fixture.serverBinary),
		"ARDENTS_BLOCKED_PROFILE="+productProfile, "ARDENTS_BLOCKED_PROBE_PROFILE="+selected,
		"ARDENTS_NEGATIVE_PROFILE="+selected,
		"ARDENTS_FAULT_MODE="+selected)
	if selected == "recovery" {
		environment = append(environment, "ARDENTS_STREAM_PROGRESS=1", "ARDENTS_STREAM_CHUNK_DELAY=2s")
	}
	return func(ctx context.Context, arguments ...string) ([]byte, error) {
		base := []string{"compose", "-p", project, "-f", filepath.Join(repository, "tests", "live", "blocked-entry.compose.yaml")}
		command := exec.CommandContext(ctx, "docker", append(base, arguments...)...)
		command.Dir, command.Env = repository, environment
		return command.CombinedOutput()
	}
}

func blockedProjectCleanup(t *testing.T, compose composeCall, project string) func() {
	t.Helper()
	var once sync.Once
	return func() {
		once.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if output, err := compose(ctx, "down", "--volumes", "--remove-orphans", "--timeout", "0"); err != nil {
				t.Errorf("remove blocked-entry project: %v\n%s", err, output)
			}
			for kind, arguments := range map[string][]string{
				"containers": {"ps", "-aq", "--filter", "label=com.docker.compose.project=" + project},
				"networks":   {"network", "ls", "-q", "--filter", "label=com.docker.compose.project=" + project},
				"volumes":    {"volume", "ls", "-q", "--filter", "label=com.docker.compose.project=" + project}} {
				if output, err := dockerOutput(ctx, arguments...); err != nil || strings.TrimSpace(string(output)) != "" {
					t.Errorf("blocked-entry %s remain: %v %s", kind, err, output)
				}
			}
		})
	}
}

func startBlockedNetwork(t *testing.T, ctx context.Context, compose composeCall, profile string) {
	t.Helper()
	services := []string{"initiator", "initiator-observer",
		"introduction", "introduction-observer", "rendezvous", "rendezvous-observer",
		"responder", "responder-observer", "publisher", "publisher-observer"}
	if profile != "C0" {
		services = append([]string{"bridge", "bridge-observer"}, services...)
	}
	arguments := append([]string{"up", "-d", "--no-build"}, services...)
	if output, err := compose(ctx, arguments...); err != nil {
		t.Fatalf("start blocked Route: %v\n%s", err, output)
	}
}

func blockedContainers(profile string) []string {
	containers := []string{"endpoint", "endpoint-observer",
		"initiator", "initiator-observer", "introduction", "introduction-observer",
		"rendezvous", "rendezvous-observer", "responder", "responder-observer",
		"publisher", "publisher-observer", "publisher-service", "client-service", "publisher-app", "client-app"}
	if profile != "C0" {
		containers = append(containers, "policy", "bridge", "bridge-observer")
	}
	if profile == "C5" || profile == "C6" {
		containers = append(containers, "probe", "probe-observer")
	}
	return containers
}

func waitBlockedContainer(t *testing.T, ctx context.Context, compose composeCall, service string) {
	t.Helper()
	identity, err := compose(ctx, "ps", "--all", "-q", service)
	if err != nil || strings.TrimSpace(string(identity)) == "" {
		t.Fatalf("resolve %s: %v\n%s", service, err, identity)
	}
	output, waitErr := dockerOutput(ctx, "wait", strings.TrimSpace(string(identity)))
	if waitErr != nil || strings.TrimSpace(string(output)) != "0" {
		logs, _ := compose(ctx, "logs", "--no-color", "--no-log-prefix", service)
		t.Fatalf("%s exit = %s, %v\n%s", service, output, waitErr, logs)
	}
}

func waitForBlockedJSON(t *testing.T, ctx context.Context, compose composeCall, service string,
	accept func([]byte) bool) {
	t.Helper()
	for {
		output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", service)
		if err == nil {
			for _, line := range bytes.Split(output, []byte{'\n'}) {
				if accept(bytes.TrimSpace(line)) {
					return
				}
			}
		}
		select {
		case <-ctx.Done():
			logCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			logs, _ := compose(logCtx, "logs", "--no-color", "--no-log-prefix", service)
			cancel()
			t.Fatalf("wait for %s: %v\n%s", service, ctx.Err(), logs)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func requireBlockedCandidate(t *testing.T, name, expected string) string {
	t.Helper()
	path := os.Getenv(name)
	if path == "" {
		t.Fatalf("%s must name the pinned external R-036 binary", name)
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(resolved)
	if err != nil {
		t.Fatal(err)
	}
	actual := sha256.Sum256(raw)
	if hex.EncodeToString(actual[:]) != expected {
		t.Fatalf("%s hash = %x, want %s", name, actual, expected)
	}
	return resolved
}

func assertBlockedApplication(t *testing.T, fixture blockedEntryFixture, clientApp, publisherApp streamworkload.Observation,
	clientService, publisherService serviceconn.Result) {
	t.Helper()
	if clientApp.Terminal != "success" || publisherApp.Terminal != "success" ||
		clientApp.Corpus != "h3-s5-http-512-65536-v1" || publisherApp.Corpus != clientApp.Corpus ||
		clientApp.RequestNonce == [32]byte{} || publisherApp.RequestNonce != clientApp.RequestNonce ||
		clientApp.SentBytes != 512 || clientApp.ReceivedBytes != 64<<10 ||
		publisherApp.SentBytes != 64<<10 || publisherApp.ReceivedBytes != 512 ||
		clientApp.SentDigest != publisherApp.ReceivedDigest || publisherApp.SentDigest != clientApp.ReceivedDigest {
		t.Fatalf("blocked Application bytes changed: client=%+v publisher=%+v", clientApp, publisherApp)
	}
	for role, result := range map[string]serviceconn.Result{"client": clientService, "publisher": publisherService} {
		if result.Class != "clean service connection close" || result.AuthenticatedTarget != fixture.target ||
			result.ConnectionCanary == [32]byte{} || result.RouteAttachmentsAccepted != 1 ||
			result.ApplicationIPCAccepts != 1 || result.AcceptedBytes != result.AcknowledgedBytes {
			t.Fatalf("blocked %s Service Connection = %+v", role, result)
		}
	}
}

func findBlockedRoute(output []byte) (route.Evidence, bool) {
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		var value route.Evidence
		if json.Unmarshal(bytes.TrimSpace(line), &value) == nil && value.Kind == "complete" {
			return value, true
		}
	}
	return route.Evidence{}, false
}

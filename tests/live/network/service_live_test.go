//go:build live

package network_test

import (
	"bytes"
	"context"
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

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
	"github.com/dianabuilds/ardents-network/internal/streamworkload"
)

func TestContainersKeepSustainedServiceConnectionUnderImpairment(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	toolImage := liveToolImage(t)
	for _, direction := range []string{"client-to-publisher", "publisher-to-client"} {
		t.Run(direction, func(t *testing.T) { runLiveImpairedDirection(t, toolImage, direction) })
	}
}

func runLiveImpairedDirection(t *testing.T, toolImage, direction string) {
	t.Helper()
	repository := repositoryRoot(t)
	baseFixture := newLiveFixture(t)
	writeLiveFile(t, filepath.Join(baseFixture.root, "client-seed.hex"),
		[]byte(hex.EncodeToString(bytes.Repeat([]byte{17}, 32))))
	writeLiveFile(t, filepath.Join(baseFixture.root, "publisher-seed.hex"),
		[]byte(hex.EncodeToString(bytes.Repeat([]byte{91}, 32))))
	project := fmt.Sprintf("ardents-live-service-%d", time.Now().UnixNano())
	image := project + ":test"
	environment := append(os.Environ(), "ARDENTS_LIVE_IMAGE="+image,
		"ARDENTS_LIVE_ROOT="+filepath.ToSlash(baseFixture.root),
		"ARDENTS_LIVE_DIRECT_BYTES="+fmt.Sprint(liveDirectBytes),
		"ARDENTS_LIVE_TOOL_IMAGE="+toolImage)
	compose := func(ctx context.Context, arguments ...string) ([]byte, error) {
		base := []string{"compose", "-p", project, "-f", filepath.Join(repository, "tests", "live", "network.compose.yaml")}
		command := exec.CommandContext(ctx, "docker", append(base, arguments...)...)
		command.Dir, command.Env = repository, environment
		return command.CombinedOutput()
	}
	cleanup := strictLiveCleanup(t, compose, project, image)
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()
	if output, err := compose(ctx, "build", "publisher"); err != nil {
		t.Fatalf("build sustained live image: %v\n%s", err, output)
	}
	directStart := filepath.Join(baseFixture.root, "direct.start")
	directBefore := runLiveDirectBaseline(t, ctx, compose, toolImage, directStart)
	serviceBytes, chunkDelay := liveSustainedWorkload(directBefore)
	fixture := newLiveServiceFixture(t, baseFixture, direction, serviceBytes)
	clientSend, clientReceive, publisherSend, publisherReceive := uint32(0), serviceBytes, serviceBytes, uint32(0)
	receiver := "client-app"
	if direction == "client-to-publisher" {
		clientSend, clientReceive, publisherSend, publisherReceive = serviceBytes, 0, 0, serviceBytes
		receiver = "publisher-app"
	}
	environment = append(environment,
		"ARDENTS_LIVE_CLIENT_SEND_BYTES="+fmt.Sprint(clientSend),
		"ARDENTS_LIVE_CLIENT_RECEIVE_BYTES="+fmt.Sprint(clientReceive),
		"ARDENTS_LIVE_PUBLISHER_SEND_BYTES="+fmt.Sprint(publisherSend),
		"ARDENTS_LIVE_PUBLISHER_RECEIVE_BYTES="+fmt.Sprint(publisherReceive),
		"ARDENTS_LIVE_CHUNK_DELAY="+chunkDelay)
	passive := []string{"publisher", "responder", "rendezvous", "introduction", "initiator"}
	if output, err := compose(ctx, append([]string{"up", "-d", "--no-build"}, passive...)...); err != nil {
		t.Fatalf("start sustained live Route: %v\n%s", err, output)
	}
	for _, service := range passive {
		waitForKind(t, ctx, compose, service, "ready")
	}
	startLiveService(t, ctx, compose, "publisher-service", "publisher")
	runLiveOneShot(t, ctx, compose, "publication-operator")
	startLiveService(t, ctx, compose, "client-service", "client")
	startLiveContainer(t, ctx, compose, "publisher-app")
	startLiveContainer(t, ctx, compose, "client-app")
	applyLiveImpairment(t, ctx, compose, toolImage, "publisher")
	routeStarted := time.Now()
	startLiveContainer(t, ctx, compose, "client")
	applyLiveImpairment(t, ctx, compose, toolImage, "client")
	monitor := monitorLiveTransfer(t, ctx, compose, serviceBytes, routeStarted, receiver)
	clientApplication := waitForApplication(t, ctx, compose, "client-app")
	publisherApplication := waitForApplication(t, ctx, compose, "publisher-app")
	clientResult := waitForServiceResult(t, ctx, compose, "client-service")
	publisherResult := waitForServiceResult(t, ctx, compose, "publisher-service")
	assertLiveServiceResults(t, fixture, direction, serviceBytes, clientResult, publisherResult,
		clientApplication, publisherApplication)

	clientRoute := waitForKind(t, ctx, compose, "client", "complete")
	if clientRoute.Terminal != "success" || clientRoute.PeerAuthenticated == false {
		t.Fatalf("impaired live Route did not finish authenticated: %+v", clientRoute)
	}
	for _, service := range passive {
		evidence := waitForKind(t, ctx, compose, service, "complete")
		if evidence.Terminal != "success" {
			t.Fatalf("%s failed during impaired live transfer: %+v", service, evidence)
		}
		assertLiveRouteResources(t, service, evidence)
	}
	directAfter := runLiveDirectBaseline(t, ctx, compose, toolImage, directStart)
	if math.Max(directBefore, directAfter)/math.Min(directBefore, directAfter) > 1.10 {
		t.Fatalf("paired impaired direct baseline drift exceeds 10%%: before %.3f after %.3f", directBefore, directAfter)
	}
	monitor.assert(t, serviceBytes, (directBefore+directAfter)/2)
	cleanup()
}

func runLiveDirectBaseline(t *testing.T, ctx context.Context, compose composeCall, toolImage, startFile string) float64 {
	t.Helper()
	if err := os.Remove(startFile); err != nil && !os.IsNotExist(err) {
		t.Fatalf("reset direct baseline start file: %v", err)
	}
	startLiveContainer(t, ctx, compose, "direct-receiver")
	waitForDirectReady(t, ctx, compose)
	applyLiveImpairment(t, ctx, compose, toolImage, "direct-receiver")
	startLiveContainer(t, ctx, compose, "direct-sender")
	applyLiveImpairment(t, ctx, compose, toolImage, "direct-sender")
	writeLiveFile(t, startFile, []byte("start\n"))
	client := waitForApplication(t, ctx, compose, "direct-sender")
	server := waitForApplication(t, ctx, compose, "direct-receiver")
	delivered := assertLiveDirectResults(t, client, server)
	if output, err := compose(ctx, "rm", "-s", "-f", "direct-sender", "direct-receiver"); err != nil {
		t.Fatalf("remove direct baseline containers: %v\n%s", err, output)
	}
	if err := os.Remove(startFile); err != nil {
		t.Fatalf("remove direct baseline start file: %v", err)
	}
	return float64(delivered) * 8 / liveDirectDuration.Seconds() / 1e6
}

func liveSustainedWorkload(directGoodput float64) (uint32, string) {
	target := math.Max(.05, math.Min(2.5, directGoodput*.5))
	count := uint64(target * 1e6 * (10 * time.Minute).Seconds() / 8)
	count = max(count, 4<<20)
	count = min(count, uint64(serviceconn.MaximumStreamBytes))
	chunks := (count + 16_380) / 16_381
	delay := (10*time.Minute + 2*time.Second) / time.Duration(max(chunks-1, 1))
	return uint32(count), delay.String()
}

func assertLiveDirectResults(t *testing.T, client, server streamworkload.Observation) uint32 {
	t.Helper()
	if client.Terminal != "success" || server.Terminal != "success" || client.SentBytes == 0 ||
		client.SentBytes > liveDirectBytes || client.DurationMillis < uint32(liveDirectDuration/time.Millisecond) ||
		client.DurationMillis > uint32((liveDirectDuration+time.Second)/time.Millisecond) ||
		server.DurationMillis < uint32((liveDirectDuration-100*time.Millisecond)/time.Millisecond) ||
		server.ReceivedBytes != client.SentBytes || client.SentDigest != server.ReceivedDigest {
		t.Fatalf("paired impaired direct baseline failed: client=%+v server=%+v", client, server)
	}
	return client.SentBytes
}

func waitForDirectReady(t *testing.T, ctx context.Context, compose composeCall) {
	t.Helper()
	waitForJSON(t, ctx, compose, "direct-receiver", func(line []byte) bool {
		var ready struct{ Kind string }
		return json.Unmarshal(line, &ready) == nil && ready.Kind == "ready"
	})
}

func startLiveService(t *testing.T, ctx context.Context, compose composeCall, service, role string) {
	t.Helper()
	startLiveContainer(t, ctx, compose, service)
	for {
		output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", service)
		if err == nil {
			for _, line := range bytes.Split(output, []byte{'\n'}) {
				var ready struct{ Kind, Role string }
				if json.Unmarshal(bytes.TrimSpace(line), &ready) == nil && ready.Kind == "ready" && ready.Role == role {
					return
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s readiness: %v\n%s", service, ctx.Err(), output)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func startLiveContainer(t *testing.T, ctx context.Context, compose composeCall, service string) {
	t.Helper()
	if output, err := compose(ctx, "up", "-d", "--no-build", "--no-deps", service); err != nil {
		t.Fatalf("start %s: %v\n%s", service, err, output)
	}
}

func runLiveOneShot(t *testing.T, ctx context.Context, compose composeCall, service string) {
	t.Helper()
	if output, err := compose(ctx, "run", "--rm", "--no-deps", service); err != nil {
		t.Fatalf("run %s: %v\n%s", service, err, output)
	}
}

func applyLiveImpairment(t *testing.T, ctx context.Context, compose composeCall, toolImage, service string) {
	t.Helper()
	seed := "424242"
	if service == "publisher" || service == "direct-receiver" {
		seed = "434343"
	} else if service != "client" && service != "direct-sender" {
		t.Fatalf("unsupported impaired service %q", service)
	}
	identity, err := compose(ctx, "ps", "-q", service)
	if err != nil || strings.TrimSpace(string(identity)) == "" {
		t.Fatalf("resolve %s for impairment: %v\n%s", service, err, identity)
	}
	arguments := []string{"run", "--rm", "--network", "container:" + strings.TrimSpace(string(identity)),
		"--user", "0:0", "--cap-add", "NET_ADMIN", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
		"--entrypoint", "/usr/sbin/tc", toolImage, "qdisc", "replace", "dev", "eth0", "root", "netem", "limit", "1000",
		"delay", "150ms", "60ms", "distribution", "normal", "loss", "5%", "rate", "20mbit", "seed", seed}
	if output, runErr := dockerOutput(ctx, arguments...); runErr != nil {
		t.Fatalf("apply %s impairment: %v\n%s", service, runErr, output)
	}
}

func liveToolImage(t *testing.T) string {
	t.Helper()
	image := os.Getenv("ARDENTS_LIVE_TOOL_IMAGE")
	if image == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		output, err := dockerOutput(ctx, "image", "ls", "--filter", "label=io.ardents.carrier-lab.target=tooling",
			"--format", "{{.Repository}}:{{.Tag}}")
		if err != nil {
			t.Fatalf("find live tooling image: %v\n%s", err, output)
		}
		for _, candidate := range strings.Fields(string(output)) {
			if !strings.Contains(candidate, "<none>") {
				image = candidate
				break
			}
		}
	}
	if image == "" {
		t.Fatal("live impairment requires a locally built Carrier tooling image or ARDENTS_LIVE_TOOL_IMAGE")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	label, err := dockerOutput(ctx, "image", "inspect", "--format", "{{index .Config.Labels \"io.ardents.carrier-lab.target\"}}", image)
	if err != nil || strings.TrimSpace(string(label)) != "tooling" {
		t.Fatalf("live tooling image has the wrong identity: %s: %v\n%s", image, err, label)
	}
	return image
}

func waitForApplication(t *testing.T, ctx context.Context, compose composeCall, service string) streamworkload.Observation {
	t.Helper()
	var value streamworkload.Observation
	waitForJSON(t, ctx, compose, service, func(line []byte) bool {
		var candidate streamworkload.Observation
		if json.Unmarshal(line, &candidate) == nil && candidate.Schema == "ardents-h3-stream-application-v1" {
			value = candidate
			return true
		}
		return false
	})
	return value
}

func waitForServiceResult(t *testing.T, ctx context.Context, compose composeCall, service string) serviceconn.Result {
	t.Helper()
	var value serviceconn.Result
	waitForJSON(t, ctx, compose, service, func(line []byte) bool {
		var candidate serviceconn.Result
		if json.Unmarshal(line, &candidate) == nil && candidate.Class != "" {
			value = candidate
			return true
		}
		return false
	})
	return value
}

func waitForJSON(t *testing.T, ctx context.Context, compose composeCall, service string, accept func([]byte) bool) {
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
			t.Fatalf("wait for %s terminal result: %v\n%s", service, ctx.Err(), output)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func assertLiveServiceResults(t *testing.T, fixture liveServiceFixture, direction string, transferBytes uint32,
	client, publisher serviceconn.Result,
	clientApp, publisherApp streamworkload.Observation) {
	t.Helper()
	for role, result := range map[string]serviceconn.Result{"client": client, "publisher": publisher} {
		if result.Class != "clean service connection close" || result.AuthenticatedTarget != fixture.target ||
			result.RouteGeneration != 1 || result.RecoveryCount != 0 || result.RouteAttachmentsAccepted != 1 ||
			result.ApplicationIPCAccepts != 1 || result.AcceptedBytes != result.AcknowledgedBytes ||
			result.QueueHighWater > 256<<10 {
			t.Fatalf("%s changed connection identity or semantics under impairment: %+v", role, result)
		}
	}
	if clientApp.Terminal != "success" || publisherApp.Terminal != "success" ||
		clientApp.ResultClass != "clean service connection close" ||
		publisherApp.ResultClass != "clean service connection close" || clientApp.AuthenticatedTarget != fixture.target ||
		publisherApp.AuthenticatedTarget != fixture.target {
		t.Fatalf("impaired Application stream changed: client=%+v publisher=%+v", clientApp, publisherApp)
	}
	if direction == "client-to-publisher" && (clientApp.SentBytes != transferBytes ||
		publisherApp.ReceivedBytes != transferBytes || clientApp.SentDigest != publisherApp.ReceivedDigest) {
		t.Fatalf("client-to-publisher impaired stream changed: client=%+v publisher=%+v", clientApp, publisherApp)
	}
	if direction == "publisher-to-client" && (publisherApp.SentBytes != transferBytes ||
		clientApp.ReceivedBytes != transferBytes || publisherApp.SentDigest != clientApp.ReceivedDigest) {
		t.Fatalf("publisher-to-client impaired stream changed: client=%+v publisher=%+v", clientApp, publisherApp)
	}
}

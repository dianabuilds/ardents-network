//go:build live

package network_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
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

func TestContainersCarryDeclaredConcurrentRoleCapacity(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	repository := repositoryRoot(t)
	toolImage := liveToolImage(t)
	fixture := newLiveFixture(t)
	const capacity = uint16(4)
	authority := fixture.authority.Public().(ed25519.PublicKey)
	fixture.writePlans(t, filepath.Join(fixture.root, "plans"), authority, capacity)
	stateRoots := []string{filepath.Join(fixture.root, "state")}
	for index := uint16(1); index < capacity+1; index++ {
		destination := filepath.Join(fixture.root, fmt.Sprintf("state-%d", index))
		if err := os.CopyFS(destination, os.DirFS(stateRoots[0])); err != nil {
			t.Fatal(err)
		}
		stateRoots = append(stateRoots, destination)
	}
	project := fmt.Sprintf("ardents-live-capacity-%d", time.Now().UnixNano())
	image := project + ":test"
	environment := append(os.Environ(), "ARDENTS_LIVE_IMAGE="+image,
		"ARDENTS_LIVE_ROOT="+filepath.ToSlash(fixture.root))
	compose := func(ctx context.Context, arguments ...string) ([]byte, error) {
		base := []string{"compose", "-p", project, "-f", filepath.Join(repository, "tests", "live", "network.compose.yaml")}
		command := exec.CommandContext(ctx, "docker", append(base, arguments...)...)
		command.Dir, command.Env = repository, environment
		return command.CombinedOutput()
	}
	cleanup := strictLiveCleanup(t, compose, project, image)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if output, err := compose(ctx, "build", "publisher"); err != nil {
		t.Fatalf("build capacity image: %v\n%s", err, output)
	}
	roles := []string{"publisher", "responder", "rendezvous", "introduction", "initiator"}
	if output, err := compose(ctx, append([]string{"up", "-d", "--no-build"}, roles...)...); err != nil {
		t.Fatalf("start capacity roles: %v\n%s", err, output)
	}
	for _, role := range roles {
		waitForKind(t, ctx, compose, role, "ready")
	}
	applyLiveCapacityDelay(t, ctx, compose, toolImage)
	time.Sleep(600 * time.Millisecond)
	type outcome struct {
		output []byte
		err    error
	}
	clients := make(chan outcome, capacity+1)
	plansRoot := filepath.Join(fixture.root, "plans")
	for index, stateRoot := range stateRoots {
		planName := fmt.Sprintf("capacity-client-%d", index)
		writeLivePlan(t, plansRoot, planName, map[string]any{
			"Role": "client", "ManifestDigest": liveHex(fixture.manifest), "StateRoot": "/run/ardents/client-state",
			"NetworkID": liveHex(fixture.network), "Authorities": []string{hex.EncodeToString(authority)}, "Threshold": 1,
			"At": fixture.now.Format(time.RFC3339), "Seed": liveHex(fixture.plan.Seed),
			"Certificate": fixture.identities[5].cert, "Key": fixture.identities[5].key,
			"PublisherPin": liveHex(fixture.identities[4].public), "Deadline": "10s",
		})
		go func(state, plan string) {
			output, err := compose(ctx, "run", "--rm", "--no-deps",
				"-v", filepath.ToSlash(state)+":/run/ardents/client-state", "--entrypoint", "/usr/local/bin/ardents-route",
				"capacity-client", "run", "/run/ardents/plans/"+plan+".json")
			clients <- outcome{output: output, err: err}
		}(stateRoot, planName)
	}
	succeeded, refused := 0, 0
	var failures []string
	for range capacity + 1 {
		client := <-clients
		if client.err == nil && containsSuccessfulClient(client.output) {
			succeeded++
		} else {
			refused++
			failures = append(failures, fmt.Sprintf("err=%v output=%s", client.err, client.output))
		}
	}
	if succeeded != int(capacity) || refused != 1 {
		t.Fatalf("authenticated live overload result: succeeded=%d refused=%d failures=%v", succeeded, refused, failures)
	}
	overloadObserved := false
	for _, role := range roles {
		evidence := waitForKind(t, ctx, compose, role, "complete")
		if evidence.Terminal != "success" || evidence.AttachmentsCompleted != capacity {
			t.Fatalf("%s capacity evidence failed: %+v", role, evidence)
		}
		if role == "publisher" && evidence.CanaryLength != uint32(capacity)*32 {
			t.Fatalf("publisher capacity omitted useful-work evidence: %+v", evidence)
		}
		overloadObserved = overloadObserved || evidence.AttachmentsRefused > 0
		assertLiveRouteResources(t, role, evidence)
	}
	if !overloadObserved {
		t.Fatal("capacity roles omitted authenticated overload evidence")
	}
	cleanup()
}

func applyLiveCapacityDelay(t *testing.T, ctx context.Context, compose composeCall, toolImage string) {
	t.Helper()
	identity, err := compose(ctx, "ps", "-q", "initiator")
	if err != nil || strings.TrimSpace(string(identity)) == "" {
		t.Fatalf("resolve Initiator for capacity delay: %v\n%s", err, identity)
	}
	arguments := []string{"run", "--rm", "--network", "container:" + strings.TrimSpace(string(identity)),
		"--user", "0:0", "--cap-add", "NET_ADMIN", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
		"--entrypoint", "/usr/sbin/tc", toolImage, "qdisc", "replace", "dev", "eth0", "root", "netem",
		"limit", "1000", "delay", "25ms", "seed", "434343"}
	if output, runErr := dockerOutput(ctx, arguments...); runErr != nil {
		t.Fatalf("apply deterministic capacity setup delay: %v\n%s", runErr, output)
	}
}

func assertLiveRouteResources(t *testing.T, role string, evidence route.Evidence) {
	t.Helper()
	if evidence.Resource == nil || evidence.Resource.MemoryBytes == 0 || evidence.Resource.FDs == 0 ||
		evidence.Resource.Goroutines == 0 {
		t.Fatalf("%s omitted bounded runtime resource evidence: %+v", role, evidence.Resource)
	}
	maximum := evidence.ResourceMaximum
	if evidence.State != "NORMAL" || evidence.ResourceSamples < 5 || maximum == nil ||
		maximum.CPUUsageUsec >= 800_000 || maximum.MemoryBytes >= 384<<20 || maximum.GoMemoryBytes >= 288<<20 ||
		maximum.SocketMemoryBytes >= 128<<20 || maximum.FDs >= 410 || maximum.Goroutines >= 410 ||
		maximum.Threads >= 64 || maximum.Timers >= 410 || maximum.QueueItems >= 410 ||
		maximum.QueueBytes >= (8<<20)*8/10 || maximum.CPUPressure >= 20 || maximum.MemoryPressure >= 5 ||
		maximum.IOPressure >= 1 || maximum.HighEvents != 0 || maximum.EmergencyEvents != 0 {
		t.Fatalf("%s crossed the reference resource profile: samples=%d maximum=%+v", role,
			evidence.ResourceSamples, maximum)
	}
}

func containsSuccessfulClient(output []byte) bool {
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		var evidence route.Evidence
		if json.Unmarshal(bytes.TrimSpace(line), &evidence) == nil && evidence.Role == "client" &&
			evidence.Terminal == "success" && evidence.CanaryLength == 32 {
			return true
		}
	}
	return false
}

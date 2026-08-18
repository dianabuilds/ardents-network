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

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
	"github.com/dianabuilds/ardents-network/internal/streamworkload"
)

type blockedRecoveryCutoff struct {
	LastByte time.Time `json:"last_byte"`
}

func TestBlockedEntryRecoveryParentCommandsAcrossNamespaces(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository := repositoryRoot(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s53-recovery-%d:test", time.Now().UnixNano()))
	buildFixture := newBlockedNegativeFixture(t, client, server)
	buildProject := finalProjectName(fmt.Sprintf("ardents-s53-recovery-build-%d", time.Now().UnixNano()))
	build := blockedCompose(repository, buildProject, image, buildFixture, "recovery")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	if ownedImage {
		if output, err := build(ctx, "build", "endpoint"); err != nil {
			t.Fatalf("build recovery image: %v\n%s", err, output)
		}
	}
	t.Cleanup(func() {
		if !ownedImage {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		if output, err := dockerOutput(cleanupCtx, "image", "rm", "--force", image); err != nil {
			t.Errorf("remove recovery image: %v\n%s", err, output)
		}
		assertNoDockerObjects(t, cleanupCtx, buildProject, image)
	})
	for episode := range 5 {
		t.Run(fmt.Sprint(episode), func(t *testing.T) {
			fixture := newBlockedNegativeFixture(t, client, server)
			cell, terminal, selected := selectedRecoveryFinalCell(episode)
			if !selected {
				return
			}
			started := time.Now()
			bindFinalFixtureSeed(t, fixture, cell, "recovery-stream")
			armFinalWorkerTerminal(terminal)
			runBlockedRecoveryEpisode(t, repository, image, fixture, episode)
			emitFinalWorkerCell(t, cell, terminal, started, fixture.root)
		})
	}
}

// selectedRecoveryFinalCell exposes the recovery scenario under its ordinary
// result cell and under G8 cancellation. Both execute the same real parent
// cancellation during a published Bridge contact; the hostile cell retains the
// Bridge terminal while recovery retains the Application connection result.
func selectedRecoveryFinalCell(episode int) (string, string, bool) {
	for _, candidate := range []struct{ cell, terminal string }{
		{fmt.Sprintf("recovery/%d", episode), "abrupt connection loss"},
		{fmt.Sprintf("hostile/G8-lifecycle/cancellation/%d", episode), "bridge-deadline-exceeded"},
	} {
		if selectedFinalCell(candidate.cell) {
			return candidate.cell, candidate.terminal, true
		}
	}
	return "", "", false
}

func runBlockedRecoveryEpisode(t *testing.T, repository, image string, fixture blockedEntryFixture, episode int) {
	t.Helper()
	project := finalProjectName(fmt.Sprintf("ardents-s53-recovery-%d-%d", episode, time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "recovery")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if output, err := compose(ctx, "up", "-d", "--no-build", "fault-one"); err != nil {
		t.Fatalf("start recovery fault: %v\n%s", err, output)
	}
	waitForBlockedJSON(t, ctx, compose, "fault-one", func(line []byte) bool {
		var value struct{ Kind, State string }
		return json.Unmarshal(line, &value) == nil && value.Kind == "fault" && value.State == "READY"
	})
	routes := []string{"initiator", "introduction", "rendezvous", "responder", "publisher"}
	routeServices := make([]string, 0, len(routes)*2)
	for _, role := range routes {
		routeServices = append(routeServices, role, role+"-observer")
	}
	if output, err := compose(ctx, append([]string{"up", "-d", "--no-build"}, routeServices...)...); err != nil {
		t.Fatalf("start recovery Route: %v\n%s", err, output)
	}
	for _, role := range routes {
		waitForKind(t, ctx, compose, role, "ready")
	}
	startLiveService(t, ctx, compose, "publisher-service", "publisher")
	runLiveOneShot(t, ctx, compose, "publication-operator")
	startLiveService(t, ctx, compose, "client-service", "client")
	startLiveContainer(t, ctx, compose, "publisher-app")
	startLiveContainer(t, ctx, compose, "client-app")
	if output, err := compose(ctx, "up", "-d", "--no-build", "--no-deps", "recovery-endpoint", "recovery-observer"); err != nil {
		t.Fatalf("start recovery endpoint: %v\n%s", err, output)
	}
	var progress struct {
		Schema        string `json:"schema"`
		ReceivedBytes uint32 `json:"received_bytes"`
		AtUnixNano    int64  `json:"at_unix_nano"`
	}
	waitForBlockedJSON(t, ctx, compose, "client-app", func(line []byte) bool {
		var value struct {
			Schema        string `json:"schema"`
			ReceivedBytes uint32 `json:"received_bytes"`
			AtUnixNano    int64  `json:"at_unix_nano"`
		}
		if json.Unmarshal(line, &value) == nil && value.Schema == "ardents-stream-progress-v1" &&
			value.ReceivedBytes > 0 && value.AtUnixNano > 0 {
			progress = value
			return true
		}
		return false
	})
	if output, err := compose(ctx, "kill", "-s", "SIGKILL", "initiator"); err != nil {
		t.Fatalf("inject initial Route loss: %v\n%s", err, output)
	}
	time.Sleep(500 * time.Millisecond)
	logs, _ := compose(ctx, "logs", "--no-color", "--no-log-prefix", "client-app")
	for _, line := range bytes.Split(logs, []byte{'\n'}) {
		var candidate struct {
			Schema        string `json:"schema"`
			ReceivedBytes uint32 `json:"received_bytes"`
			AtUnixNano    int64  `json:"at_unix_nano"`
		}
		if json.Unmarshal(bytes.TrimSpace(line), &candidate) == nil && candidate.Schema == "ardents-stream-progress-v1" &&
			candidate.ReceivedBytes >= progress.ReceivedBytes && candidate.AtUnixNano > progress.AtUnixNano {
			progress = candidate
		}
	}
	lastByte := time.Unix(0, progress.AtUnixNano)
	writeLivePlan(t, filepath.Join(fixture.root, "sync", "recovery-endpoint"), "recovery-cutoff",
		blockedRecoveryCutoff{LastByte: lastByte})
	waitForBlockedJSON(t, ctx, compose, "recovery-endpoint", func(line []byte) bool {
		var value struct{ Kind, Class string }
		return json.Unmarshal(line, &value) == nil && value.Kind == "recovery-result" &&
			value.Class == "bridge-deadline-exceeded"
	})
	if time.Since(lastByte) > 14*time.Second {
		t.Fatal("recovery command cleanup missed +14s")
	}
	var result serviceconn.Result
	waitForBlockedJSON(t, ctx, compose, "client-service", func(line []byte) bool {
		return json.Unmarshal(line, &result) == nil && result.Class != ""
	})
	var publication struct {
		Kind       string `json:"kind"`
		AtUnixNano int64  `json:"at_unix_nano"`
	}
	waitForBlockedJSON(t, ctx, compose, "client-service", func(line []byte) bool {
		return json.Unmarshal(line, &publication) == nil && publication.Kind == "connection-result-published"
	})
	publishedAt := time.Unix(0, publication.AtUnixNano)
	if result.Class != "abrupt connection loss" || result.RecoveryCount != 0 ||
		publishedAt.Sub(lastByte) > 15*time.Second {
		t.Fatalf("recovery Connection Result = %+v published after %s", result, publishedAt.Sub(lastByte))
	}
	var application streamworkload.Observation
	waitForBlockedJSON(t, ctx, compose, "client-app", func(line []byte) bool {
		return json.Unmarshal(line, &application) == nil && application.Schema == "ardents-h3-stream-application-v1"
	})
	applicationAt := time.Unix(0, application.CompletedAtUnixNano)
	if application.ResultClass != "abrupt connection loss" || application.CompletedAtUnixNano == 0 ||
		applicationAt.Sub(lastByte) > 15*time.Second {
		t.Fatalf("recovery Application result = %+v received after %s", application, applicationAt.Sub(lastByte))
	}
	publishFinalWorkerTerminal()
	for _, service := range []string{"recovery-endpoint", "recovery-observer", "fault-one"} {
		waitBlockedContainer(t, ctx, compose, service)
	}
	cleanup()
}

//go:build live

package network_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestBlockedEntryNegativeCommandsAcrossNamespaces(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository := repositoryRoot(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s53-negative-%d:test", time.Now().UnixNano()))
	buildFixture := newBlockedNegativeFixture(t, client, server)
	buildProject := finalProjectName(fmt.Sprintf("ardents-s53-negative-build-%d", time.Now().UnixNano()))
	build := blockedCompose(repository, buildProject, image, buildFixture, "C3")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	if ownedImage {
		if output, err := build(ctx, "build", "endpoint"); err != nil {
			t.Fatalf("build negative image: %v\n%s", err, output)
		}
	}
	t.Cleanup(func() {
		if !ownedImage {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		if output, err := dockerOutput(cleanupCtx, "image", "rm", "--force", image); err != nil {
			t.Errorf("remove negative image: %v\n%s", err, output)
		}
		assertNoDockerObjects(t, cleanupCtx, buildProject, image)
	})
	for _, profile := range []string{"C3", "C4", "G5"} {
		t.Run(profile, func(t *testing.T) {
			for episode := range 5 {
				t.Run(fmt.Sprint(episode), func(t *testing.T) {
					fixture := newBlockedNegativeFixture(t, client, server)
					cell := fmt.Sprintf("profile/%s/%02d", profile, episode)
					if profile == "G5" {
						cell = fmt.Sprintf("hostile/G5-adapter-fault/accept-then-stall/%d", episode)
					}
					if !selectedFinalCell(cell) {
						return
					}
					started := time.Now()
					bindFinalFixtureSeed(t, fixture, cell,
						"short-workload")
					armFinalWorkerTerminal("bridge-attempt-exhausted")
					runBlockedNegativeEpisode(t, repository, image, fixture, profile, episode)
					emitFinalWorkerCell(t, cell, "bridge-attempt-exhausted", started, fixture.root)
				})
			}
		})
	}
}

func runBlockedNegativeEpisode(t *testing.T, repository, image string, fixture blockedEntryFixture,
	profile string, episode int,
) {
	t.Helper()
	project := finalProjectName(fmt.Sprintf("ardents-s53-%s-%d-%d", strings.ToLower(profile), episode, time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, profile)
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if profile == "C4" || profile == "G5" {
		faults := []string{"fault-zero"}
		if profile == "C4" {
			faults = append(faults, "fault-one")
		}
		if output, err := compose(ctx, append([]string{"up", "-d", "--no-build"}, faults...)...); err != nil {
			t.Fatalf("start C4 faults: %v\n%s", err, output)
		}
		for _, service := range faults {
			waitForBlockedJSON(t, ctx, compose, service, func(line []byte) bool {
				var value struct{ Kind, State string }
				return json.Unmarshal(line, &value) == nil && value.Kind == "fault" && value.State == "READY"
			})
		}
	}
	services := []string{"negative-endpoint", "negative-observer"}
	if profile == "C3" {
		services = append(services, "negative-policy")
	}
	arguments := append([]string{"up", "-d", "--no-build", "--no-deps"}, services...)
	if output, err := compose(ctx, arguments...); err != nil {
		t.Fatalf("start %s endpoint: %v\n%s", profile, err, output)
	}
	if profile == "C3" {
		waitForBlockedJSON(t, ctx, compose, "negative-policy", func(line []byte) bool {
			var value struct{ Kind, State string }
			return json.Unmarshal(line, &value) == nil && value.Kind == "policy" && value.State == "READY"
		})
	}
	waitForBlockedJSON(t, ctx, compose, "negative-endpoint", func(line []byte) bool {
		var value struct{ Kind, Profile, Terminal string }
		return json.Unmarshal(line, &value) == nil && value.Kind == "negative-result" &&
			value.Profile == profile && value.Terminal == "bridge-attempt-exhausted"
	})
	publishFinalWorkerTerminal()
	waitBlockedContainer(t, ctx, compose, "negative-endpoint")
	waitBlockedContainer(t, ctx, compose, "negative-observer")
	if profile == "C3" {
		waitBlockedContainer(t, ctx, compose, "negative-policy")
	} else if profile == "C4" {
		waitBlockedContainer(t, ctx, compose, "fault-zero")
		waitBlockedContainer(t, ctx, compose, "fault-one")
	} else {
		waitBlockedContainer(t, ctx, compose, "fault-zero")
	}
	cleanup()
}

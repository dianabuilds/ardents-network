//go:build live

package network_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
					terminal := "bridge-attempt-exhausted"
					if profile == "G5" {
						var selected bool
						cell, terminal, selected = selectedG5FinalCell(episode)
						if !selected {
							return
						}
						prepareG5FinalFixture(t, fixture, strings.Split(cell, "/")[2])
					}
					if !selectedFinalCell(cell) {
						return
					}
					started := time.Now()
					bindFinalFixtureSeed(t, fixture, cell,
						"short-workload")
					armFinalWorkerTerminal(terminal)
					receipt := runBlockedNegativeEpisode(t, repository, image, fixture, profile, episode, terminal)
					if profile == "G5" {
						variant := strings.Split(cell, "/")[2]
						recordFinalFault(cell, []byte("adapter-ready"), []byte(variant), receipt)
					}
					emitFinalWorkerCell(t, cell, terminal, started, fixture.root)
				})
			}
		})
	}
}

func runBlockedNegativeEpisode(t *testing.T, repository, image string, fixture blockedEntryFixture,
	profile string, episode int, terminal string,
) []byte {
	t.Helper()
	project := finalProjectName(fmt.Sprintf("ardents-s53-%s-%d-%d", strings.ToLower(profile), episode, time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, profile)
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	variant := os.Getenv("ARDENTS_HOSTILE_VARIANT")
	childControlFault := profile == "G5" &&
		(variant == "malformed-pt-control" || variant == "wrong-socks-listener-method")
	if profile == "C4" || profile == "G5" {
		faults := []string{"fault-zero"}
		if profile == "C4" {
			faults = append(faults, "fault-one")
		}
		if childControlFault {
			faults = nil
		}
		if len(faults) > 0 {
			if output, err := compose(ctx, append([]string{"up", "-d", "--no-build"}, faults...)...); err != nil {
				t.Fatalf("start C4 faults: %v\n%s", err, output)
			}
		}
		for _, service := range faults {
			waitForBlockedJSON(t, ctx, compose, service, func(line []byte) bool {
				var value struct{ Kind, State string }
				return json.Unmarshal(line, &value) == nil && value.Kind == "fault" && value.State == "READY"
			})
		}
		if profile == "G5" && (os.Getenv("ARDENTS_HOSTILE_VARIANT") == "sigterm" ||
			os.Getenv("ARDENTS_HOSTILE_VARIANT") == "sigkill") {
			signal := "SIGTERM"
			if os.Getenv("ARDENTS_HOSTILE_VARIANT") == "sigkill" {
				signal = "SIGKILL"
			}
			if output, err := compose(ctx, "kill", "-s", signal, "fault-zero"); err != nil {
				t.Fatalf("inject G5 %s: %v\n%s", signal, err, output)
			}
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
			value.Profile == profile && value.Terminal == terminal
	})
	receipt, logErr := compose(ctx, "logs", "--no-color", "--no-log-prefix", "negative-endpoint")
	if logErr != nil || len(receipt) == 0 {
		t.Fatalf("read causal negative endpoint receipt: %v\n%s", logErr, receipt)
	}
	publishFinalWorkerTerminal()
	waitBlockedContainer(t, ctx, compose, "negative-endpoint")
	waitBlockedContainer(t, ctx, compose, "negative-observer")
	if profile == "C3" {
		waitBlockedContainer(t, ctx, compose, "negative-policy")
	} else if profile == "C4" {
		waitBlockedContainer(t, ctx, compose, "fault-zero")
		waitBlockedContainer(t, ctx, compose, "fault-one")
	} else if !childControlFault {
		waitBlockedContainer(t, ctx, compose, "fault-zero")
	}
	cleanup()
	return receipt
}

func selectedG5FinalCell(episode int) (string, string, bool) {
	cell := os.Getenv("ARDENTS_FINAL_CELL")
	parts := strings.Split(cell, "/")
	if len(parts) != 4 || parts[0] != "hostile" || parts[1] != "G5-adapter-fault" ||
		parts[3] != strconv.Itoa(episode) {
		return "", "", false
	}
	terminal := "bridge-attempt-exhausted"
	if parts[2] == "evidence-write-exhaustion" {
		terminal = "bridge-local-denial"
	}
	return cell, terminal, true
}

func prepareG5FinalFixture(t *testing.T, fixture blockedEntryFixture, variant string) {
	t.Helper()
	path := filepath.Join(fixture.root, "input", "negative-endpoint", "entry.json")
	if variant == "malformed-pt-control" || variant == "wrong-socks-listener-method" {
		transcript := "VERSION 1\\nCMETHOD webtunnel socks5 127.0.0.1:not-a-port\\nCMETHODS DONE\\n"
		if variant == "wrong-socks-listener-method" {
			transcript = "VERSION 1\\nCMETHOD webtunnel socks4 127.0.0.1:4123\\nCMETHODS DONE\\n"
		}
		script := "#!/bin/sh\nprintf '" + transcript + "'\n"
		faultPath := filepath.Join(fixture.root, "input", "negative-endpoint", "pt-control-fault")
		if err := os.WriteFile(faultPath, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		rewriteBlockedPlan(t, path, func(plan map[string]any) {
			plan["binary"] = "/run/input/pt-control-fault"
		})
		return
	}
	if variant != "evidence-write-exhaustion" {
		return
	}
	rewriteBlockedPlan(t, path, func(plan map[string]any) {
		plan["candidate_state_root"] = "/proc/1/ardents-evidence-denied"
	})
}

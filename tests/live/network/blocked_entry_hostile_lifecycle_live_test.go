//go:build live

package network_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

type blockedHostileLifecycleResult struct {
	Kind, Cell, Terminal, Before, After, Receipt string
}

func TestBlockedEntryFinalHostileLifecycle(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	cell, terminal, ok := selectedHostileLifecycleCell(os.Getenv("ARDENTS_FINAL_CELL"))
	if !ok {
		t.Skip("selected G8 lifecycle cell only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository := repositoryRoot(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s55-g8-%d:test", time.Now().UnixNano()))
	fixture := newBlockedEntryFixture(t, client, server)
	project := finalProjectName(fmt.Sprintf("ardents-s55-g8-%d", time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "final-hostile")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if ownedImage {
		if output, err := compose(ctx, "build", "endpoint"); err != nil {
			t.Fatalf("build G8 product image: %v\n%s", err, output)
		}
	}
	startHostileInviteObservers(t, ctx, compose, fixture.root)
	started := time.Now()
	armFinalWorkerTerminal(terminal)
	output, err := compose(ctx, "exec", "-T", "-e", "ARDENTS_BLOCKED_ROLE=hostile-lifecycle",
		"-e", "ARDENTS_HOSTILE_CELL="+cell, "endpoint", "/usr/local/bin/network-live.test",
		"-test.count=1", "-test.v", "-test.run", "^TestBlockedEntryRole$")
	if err != nil {
		t.Fatalf("execute G8 lifecycle exercise: %v\n%s", err, output)
	}
	result := decodeBlockedHostileLifecycleResult(t, output)
	if result.Cell != cell || result.Terminal != terminal || result.Before == "" || result.After == "" ||
		result.Receipt == "" {
		t.Fatalf("G8 lifecycle result is incomplete: %+v", result)
	}
	recordFinalFault(cell, []byte(result.Before), []byte(result.After), []byte(result.Receipt))
	publishFinalWorkerTerminal()
	stopHostileInviteObservers(t, ctx, compose, fixture.root)
	cleanup()
	emitFinalWorkerCell(t, cell, terminal, started, fixture.root)
}

func selectedHostileLifecycleCell(cell string) (string, string, bool) {
	parts := strings.Split(cell, "/")
	if len(parts) != 4 || parts[0] != "hostile" || parts[1] != "G8-lifecycle" ||
		parts[2] != "expiry-revocation" && parts[2] != "bridge-restart" &&
			parts[2] != "clock-discontinuity" && parts[2] != "residual-injection" {
		return "", "", false
	}
	episode, err := strconv.Atoi(parts[3])
	if err != nil || episode < 0 || episode >= 5 {
		return "", "", false
	}
	terminal := "bridge-local-denial"
	if parts[2] == "bridge-restart" {
		terminal = "bridge-interrupted"
	} else if parts[2] != "residual-injection" {
		terminal = "bridge-ineligible"
	}
	return cell, terminal, true
}

func decodeBlockedHostileLifecycleResult(t *testing.T, output []byte) blockedHostileLifecycleResult {
	t.Helper()
	for _, line := range strings.Split(string(output), "\n") {
		var value blockedHostileLifecycleResult
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &value) == nil && value.Kind == "hostile-lifecycle" {
			return value
		}
	}
	t.Fatalf("missing hostile lifecycle result:\n%s", output)
	return blockedHostileLifecycleResult{}
}

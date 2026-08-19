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

type blockedHostileLedgerResult struct {
	Kind, Cell, Terminal, Before, After, Receipt string
}

func TestBlockedEntryFinalHostileLedger(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	cell, ok := selectedHostileLedgerCell(os.Getenv("ARDENTS_FINAL_CELL"))
	if !ok {
		t.Skip("selected G9 product-ledger cell only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository := repositoryRoot(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s55-g9-%d:test", time.Now().UnixNano()))
	fixture := newBlockedEntryFixture(t, client, server)
	project := finalProjectName(fmt.Sprintf("ardents-s55-g9-%d", time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "final-hostile")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if ownedImage {
		if output, err := compose(ctx, "build", "endpoint"); err != nil {
			t.Fatalf("build G9 product image: %v\n%s", err, output)
		}
	}
	startHostileInviteObservers(t, ctx, compose, fixture.root)
	started := time.Now()
	armFinalWorkerTerminal("bridge-local-denial")
	output, err := compose(ctx, "exec", "-T", "-e", "ARDENTS_BLOCKED_ROLE=hostile-ledger",
		"-e", "ARDENTS_HOSTILE_CELL="+cell, "endpoint", "/usr/local/bin/network-live.test",
		"-test.count=1", "-test.v", "-test.run", "^TestBlockedEntryRole$")
	if err != nil {
		t.Fatalf("execute G9 ledger exercise: %v\n%s", err, output)
	}
	result := decodeBlockedHostileLedgerResult(t, output)
	if result.Cell != cell || result.Terminal != "bridge-local-denial" || result.Before == "" ||
		result.After == "" || result.Receipt == "" {
		t.Fatalf("G9 ledger result is incomplete: %+v", result)
	}
	recordFinalFault(cell, []byte(result.Before), []byte(result.After), []byte(result.Receipt))
	publishFinalWorkerTerminal()
	stopHostileInviteObservers(t, ctx, compose, fixture.root)
	cleanup()
	emitFinalWorkerCell(t, cell, "bridge-local-denial", started, fixture.root)
}

func selectedHostileLedgerCell(cell string) (string, bool) {
	parts := strings.Split(cell, "/")
	if len(parts) != 4 || parts[0] != "hostile" || parts[1] != "G9-ledger-leakage" ||
		parts[2] == "unknown-invite-field" || strings.HasPrefix(parts[2], "pipeline-contamination-") {
		return "", false
	}
	episode, err := strconv.Atoi(parts[3])
	return cell, err == nil && episode >= 0 && episode < 5
}

func decodeBlockedHostileLedgerResult(t *testing.T, output []byte) blockedHostileLedgerResult {
	t.Helper()
	for _, line := range strings.Split(string(output), "\n") {
		var value blockedHostileLedgerResult
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &value) == nil && value.Kind == "hostile-ledger" {
			return value
		}
	}
	t.Fatalf("missing hostile ledger result:\n%s", output)
	return blockedHostileLedgerResult{}
}

func hostileLedgerResultPath(root string) string {
	return filepath.Join(root, "hostile-ledger-result.json")
}

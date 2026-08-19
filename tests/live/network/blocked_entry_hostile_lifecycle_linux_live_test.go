//go:build linux && live

package network_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/localroles"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

func runBlockedHostileLifecycle(t *testing.T) {
	cell := os.Getenv("ARDENTS_HOSTILE_CELL")
	parts := strings.Split(cell, "/")
	if len(parts) != 4 || parts[1] != "G8-lifecycle" {
		t.Fatal("hostile lifecycle cell is invalid")
	}
	prepareBlockedState(t, "bridge-network", "bridge-network")
	prepareBlockedState(t, "local-roles", "local-roles")
	runBlockedCommand(t, "/usr/local/bin/ardents-bridge", "import", "/run/secure/import.json")
	before := blockedCurrentStateHash(t, "/run/state/bridge")
	terminal, receipt := "bridge-local-denial", ""
	switch parts[2] {
	case "expiry-revocation":
		terminal, receipt = "bridge-ineligible", exerciseBlockedRevocation(t)
	case "bridge-restart":
		terminal, receipt = "bridge-interrupted", exerciseBlockedBridgeRestart(t)
	case "clock-discontinuity":
		terminal, receipt = "bridge-ineligible", exerciseBlockedClockDiscontinuity(t)
	case "residual-injection":
		receipt = exerciseBlockedResidualInjection(t)
	default:
		t.Fatalf("unsupported G8 variant %q", parts[2])
	}
	after := blockedCurrentStateHash(t, "/run/state/bridge")
	result := blockedHostileLifecycleResult{Kind: "hostile-lifecycle", Cell: cell, Terminal: terminal,
		Before: before, After: after, Receipt: receipt}
	raw, _ := json.Marshal(result)
	fmt.Println(string(raw))
}

func exerciseBlockedBridgeRestart(t *testing.T) string {
	first := runBlockedBridgeRestartChild(t, "begin")
	second := runBlockedBridgeRestartChild(t, "resume")
	if first.Terminal != "contact-published" || second.Terminal != "bridge-interrupted" ||
		first.PID == second.PID {
		t.Fatalf("Bridge process restart first=%+v second=%+v", first, second)
	}
	return fmt.Sprintf("process-restart:%d->%d:%s", first.PID, second.PID, second.Terminal)
}

type blockedBridgeRestartProcessResult struct {
	Kind, Terminal string
	PID            int
}

func runBlockedBridgeRestartChild(t *testing.T, mode string) blockedBridgeRestartProcessResult {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run", "^TestBlockedEntryRole$", "-test.count=1", "-test.v")
	command.Env = append(os.Environ(), "ARDENTS_BLOCKED_ROLE=hostile-bridge-restart",
		"ARDENTS_BRIDGE_RESTART_MODE="+mode)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run Bridge restart %s process: %v\n%s", mode, err, output)
	}
	for _, line := range strings.Split(string(output), "\n") {
		var result blockedBridgeRestartProcessResult
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &result) == nil && result.Kind == "bridge-restart-process" {
			return result
		}
	}
	t.Fatalf("missing Bridge restart %s process result:\n%s", mode, output)
	return blockedBridgeRestartProcessResult{}
}

func runBlockedBridgeRestartProcess(t *testing.T) {
	transition, manifest := blockedTransitionInputs(t)
	owner, closeOwner := openBlockedBridgeOwner(t, "/run/secure/import.json", time.Now)
	mode := os.Getenv("ARDENTS_BRIDGE_RESTART_MODE")
	terminal := ""
	if mode == "begin" {
		_, _, _, _, _, err := owner.BeginContact(transition, manifest, time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		terminal = "contact-published"
	} else if mode == "resume" {
		defer func() { _ = closeOwner() }()
		_, _, _, _, _, err := owner.BeginContact(transition, manifest, time.Now().Add(time.Minute))
		if err == nil || !strings.Contains(err.Error(), "bridge-interrupted") {
			t.Fatalf("Bridge restarted process returned %v", err)
		}
		terminal = "bridge-interrupted"
	} else {
		t.Fatal("Bridge restart process mode is invalid")
	}
	result := blockedBridgeRestartProcessResult{Kind: "bridge-restart-process", Terminal: terminal, PID: os.Getpid()}
	raw, _ := json.Marshal(result)
	fmt.Println(string(raw))
}

func blockedTransitionInputs(t *testing.T) ([]byte, [32]byte) {
	t.Helper()
	timeline := startBlockedTimeline(t)
	transition, err := os.ReadFile("/run/secure/transition.bin")
	if err != nil {
		t.Fatal(err)
	}
	transition = stampBlockedTransition(t, transition, timeline)
	var entry struct {
		RouteManifestDigest string `json:"route_manifest_digest"`
	}
	if err := planfile.Decode("/run/secure/entry.json", 32<<10, &entry); err != nil {
		t.Fatal(err)
	}
	var manifest [32]byte
	if err := planfile.FixedHex(entry.RouteManifestDigest, manifest[:]); err != nil {
		t.Fatal(err)
	}
	return transition, manifest
}

func exerciseBlockedRevocation(t *testing.T) string {
	transition, manifest := blockedTransitionInputs(t)
	owner, closeOwner := openBlockedBridgeOwner(t, "/run/secure/import.json", time.Now)
	defer func() { _ = closeOwner() }()
	_, _, ordinal, _, _, err := owner.BeginContact(transition, manifest, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.FinishContact(ordinal, uint64(time.Second), false, true); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("/run/secure/invite.bin")
	if err != nil {
		t.Fatal(err)
	}
	identity, family := hostileInviteIdentityFamily(t, raw)
	roles, err := localroles.Open(localroles.Config{Root: "/run/state/local-roles", Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	err = roles.Replace([32]byte{9}, []localroles.Duty{{Identity: identity, Family: family,
		Class: "route-rendezvous", State: "quarantined", NotAfter: time.Now().Add(time.Minute)}})
	if closeErr := roles.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = owner.NextContact(context.Background())
	if err == nil || !strings.Contains(err.Error(), "bridge-ineligible") {
		t.Fatalf("revoked Bridge contact returned %v", err)
	}
	return err.Error()
}

func exerciseBlockedClockDiscontinuity(t *testing.T) string {
	transition, manifest := blockedTransitionInputs(t)
	confident := true
	owner, closeOwner := openBlockedBridgeOwnerWithConfidence(t, "/run/secure/import.json", time.Now,
		func() bool { return confident })
	defer func() { _ = closeOwner() }()
	_, _, ordinal, _, _, err := owner.BeginContact(transition, manifest, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.FinishContact(ordinal, uint64(time.Second), false, true); err != nil {
		t.Fatal(err)
	}
	confident = false
	_, _, _, err = owner.NextContact(context.Background())
	if err == nil || !strings.Contains(err.Error(), "bridge-ineligible") {
		t.Fatalf("lost Time Confidence returned %v", err)
	}
	return "time-confidence:true->false:" + err.Error()
}

func exerciseBlockedResidualInjection(t *testing.T) string {
	path := "/run/state/bridge/residual-injected"
	if err := os.WriteFile(path, []byte("injected residual\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/usr/local/bin/ardents-bridge", "import", "/run/secure/import.json")
	output, err := command.CombinedOutput()
	if removeErr := os.Remove(path); removeErr != nil {
		t.Fatal(removeErr)
	}
	if err == nil || !strings.Contains(string(output), "unknown") {
		t.Fatalf("injected residual was accepted: %v\n%s", err, output)
	}
	return hex.EncodeToString(output)
}

//go:build live

package network_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestBlockedEntryFinalHostileReplay verifies the first replay matrix case with
// two independent command invocations over the same durable Bridge root.
func TestBlockedEntryFinalHostileReplay(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	cell, variant, ok := selectedHostileReplayCell(os.Getenv("ARDENTS_FINAL_CELL"))
	if !ok {
		t.Skip("selected supported G3 final cell only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository := repositoryRoot(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s55-g3-%d:test", time.Now().UnixNano()))
	fixture := newBlockedEntryFixture(t, client, server)
	project := finalProjectName(fmt.Sprintf("ardents-s55-g3-%d", time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "final-hostile")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if ownedImage {
		if output, err := compose(ctx, "build", "endpoint"); err != nil {
			t.Fatalf("build G3 product image: %v\n%s", err, output)
		}
	}
	startHostileInviteObservers(t, ctx, compose, fixture.root)
	started := time.Now()
	if observed := runHostileReplayImport(t, ctx, compose, "/run/input/import.json"); observed != "accepted" {
		t.Fatalf("G3 initial import terminal=%s want accepted", observed)
	}
	terminal, replayPlan := "already-present", "/run/input/import.json"
	if variant == "retired-replay" {
		replayPlan = prepareHostileReplacement(t, fixture)
		if observed := runHostileReplayImport(t, ctx, compose, replayPlan); observed != "accepted" {
			t.Fatalf("G3 replacement terminal=%s want accepted", observed)
		}
		terminal = "replay"
	} else if variant == "same-generation-different-bytes" {
		replayPlan, terminal = prepareHostileSameGenerationInvite(t, fixture), "replay"
	} else if variant == "full-set" {
		plans := prepareHostileFullSet(t, fixture)
		for _, setup := range plans[:len(plans)-1] {
			if observed := runHostileReplayImport(t, ctx, compose, setup); observed != "accepted" {
				t.Fatalf("G3 full-set setup terminal=%s want accepted", observed)
			}
		}
		replayPlan, terminal = plans[len(plans)-1], "set-full"
	} else if variant == "wrong-replacement-id" || variant == "skipped-generation" ||
		variant == "third-generation" || variant == "cross-slot-replacement" {
		replayPlan, terminal = prepareHostileRejectedReplacement(t, fixture, variant), "replacement-rejected"
	}
	armFinalWorkerTerminal(terminal)
	if observed := runHostileReplayImport(t, ctx, compose, replayPlan); observed != terminal {
		t.Fatalf("G3 replay terminal=%s want=%s", observed, terminal)
	}
	publishFinalWorkerTerminal()
	stopHostileInviteObservers(t, ctx, compose, fixture.root)
	cleanup()
	emitFinalWorkerCell(t, cell, terminal, started, fixture.root)
}

func selectedHostileReplayCell(cell string) (string, string, bool) {
	parts := strings.Split(cell, "/")
	if len(parts) != 4 || parts[0] != "hostile" || parts[1] != "G3-replay-replacement" ||
		parts[2] != "active-reimport" && parts[2] != "retired-replay" &&
			parts[2] != "same-generation-different-bytes" && parts[2] != "wrong-replacement-id" &&
			parts[2] != "skipped-generation" && parts[2] != "third-generation" &&
			parts[2] != "full-set" && parts[2] != "cross-slot-replacement" {
		return "", "", false
	}
	episode, err := strconv.Atoi(parts[3])
	return cell, parts[2], err == nil && episode >= 0 && episode < 5
}

func prepareHostileRejectedReplacement(t *testing.T, fixture blockedEntryFixture, variant string) string {
	t.Helper()
	input := filepath.Join(fixture.root, "input", "endpoint")
	raw, err := os.ReadFile(filepath.Join(input, "invite.bin"))
	if err != nil {
		t.Fatal(err)
	}
	body := hostileInviteBody(t, raw)
	_, notAfter := hostileInviteTimeOffsets(t, body)
	position := notAfter + 8
	if position+3 > len(body) {
		t.Fatal("fixture Invite replacement fields are truncated")
	}
	switch variant {
	case "wrong-replacement-id":
		body[position], body[position+2] = 2, 1
		body = append(body[:position+3], append(make([]byte, 32), body[position+3:]...)...)
	case "skipped-generation":
		body[position], body[position+1], body[position+2] = 2, 1, 0
	case "third-generation":
		body[position] = 3
	case "cross-slot-replacement":
		id := hostileInviteID(body)
		body[position], body[position+1], body[position+2] = 2, 1, 1
		body = append(body[:position+3], append(id[:], body[position+3:]...)...)
	}
	writeLiveFile(t, filepath.Join(input, "rejected-replacement.bin"), signHostileInvite(body))
	plan := hostileImportPlan(t, input, "/run/input/rejected-replacement.bin")
	writeLivePlan(t, input, "hostile-rejected-replacement", plan)
	return "/run/input/hostile-rejected-replacement.json"
}

func prepareHostileFullSet(t *testing.T, fixture blockedEntryFixture) []string {
	t.Helper()
	input := filepath.Join(fixture.root, "input", "endpoint")
	raw, err := os.ReadFile(filepath.Join(input, "invite.bin"))
	if err != nil {
		t.Fatal(err)
	}
	original := hostileInviteBody(t, raw)
	_, notAfter := hostileInviteTimeOffsets(t, original)
	position := notAfter + 8
	if position+5 > len(original) {
		t.Fatal("fixture Invite replacement fields are truncated")
	}

	slot1 := append([]byte(nil), original...)
	slot1[position+1] = 1
	writeLiveFile(t, filepath.Join(input, "slot-1.bin"), signHostileInvite(slot1))
	writeLivePlan(t, input, "hostile-slot-1", hostileImportPlan(t, input, "/run/input/slot-1.bin"))

	slot1Replacement := append([]byte(nil), original...)
	slot1Replacement[position], slot1Replacement[position+1], slot1Replacement[position+2] = 2, 1, 1
	id := hostileInviteID(slot1)
	slot1Replacement = append(slot1Replacement[:position+3], append(id[:], slot1Replacement[position+3:]...)...)
	writeLiveFile(t, filepath.Join(input, "slot-1-replacement.bin"), signHostileInvite(slot1Replacement))
	writeLivePlan(t, input, "hostile-slot-1-replacement",
		hostileImportPlan(t, input, "/run/input/slot-1-replacement.bin"))

	full := append([]byte(nil), original...)
	candidate := position + 3
	length := int(binary.BigEndian.Uint16(full[candidate : candidate+2]))
	if length == 0 || candidate+2+length > len(full) {
		t.Fatal("fixture Invite candidate is invalid")
	}
	full[candidate+1+length] ^= 1
	writeLiveFile(t, filepath.Join(input, "full-set.bin"), signHostileInvite(full))
	writeLivePlan(t, input, "hostile-full-set", hostileImportPlan(t, input, "/run/input/full-set.bin"))
	return []string{prepareHostileReplacement(t, fixture), "/run/input/hostile-slot-1.json",
		"/run/input/hostile-slot-1-replacement.json", "/run/input/hostile-full-set.json"}
}

func prepareHostileSameGenerationInvite(t *testing.T, fixture blockedEntryFixture) string {
	t.Helper()
	input := filepath.Join(fixture.root, "input", "endpoint")
	raw, err := os.ReadFile(filepath.Join(input, "invite.bin"))
	if err != nil {
		t.Fatal(err)
	}
	body := hostileInviteBody(t, raw)
	_, notAfter := hostileInviteTimeOffsets(t, body)
	position := notAfter + 8 + 3
	if position+2 > len(body) {
		t.Fatal("fixture Invite candidate is truncated")
	}
	length := int(binary.BigEndian.Uint16(body[position : position+2]))
	if length == 0 || position+2+length > len(body) {
		t.Fatal("fixture Invite candidate is invalid")
	}
	body[position+1+length] ^= 1
	writeLiveFile(t, filepath.Join(input, "same-generation.bin"), signHostileInvite(body))
	plan := hostileImportPlan(t, input, "/run/input/same-generation.bin")
	writeLivePlan(t, input, "hostile-same-generation", plan)
	return "/run/input/hostile-same-generation.json"
}

func prepareHostileReplacement(t *testing.T, fixture blockedEntryFixture) string {
	t.Helper()
	input := filepath.Join(fixture.root, "input", "endpoint")
	raw, err := os.ReadFile(filepath.Join(input, "invite.bin"))
	if err != nil {
		t.Fatal(err)
	}
	body := hostileInviteBody(t, raw)
	id := hostileInviteID(body)
	_, notAfter := hostileInviteTimeOffsets(t, body)
	position := notAfter + 8
	if position+3 > len(body) {
		t.Fatal("fixture Invite replacement fields are truncated")
	}
	body[position], body[position+2] = 2, 1
	body = append(body[:position+3], append(id[:], body[position+3:]...)...)
	writeLiveFile(t, filepath.Join(input, "replacement.bin"), signHostileInvite(body))
	plan := hostileImportPlan(t, input, "/run/input/replacement.bin")
	writeLivePlan(t, input, "hostile-replacement", plan)
	return "/run/input/hostile-replacement.json"
}

func hostileInviteID(body []byte) [32]byte {
	return sha256.Sum256(append([]byte("ardents-h3-bridge-invite-id-v1\x00"), body...))
}

func hostileImportPlan(t *testing.T, input, invite string) map[string]any {
	t.Helper()
	return map[string]any{"state_root": "/run/state/bridge", "network_state_root": "/run/state/bridge-network",
		"invite_file": invite, "network_id": readPlanString(t, filepath.Join(input, "import.json"), "network_id"),
		"network_authorities": readPlanValue(t, filepath.Join(input, "import.json"), "network_authorities"),
		"network_threshold":   1, "network_profile": "h3-role-probe-v1", "route_profile": "h3-route-tracer-v1",
		"local_role_state_root": "/run/state/local-roles", "time_confidence_file": "/run/input/time-confidence"}
}

func runHostileReplayImport(t *testing.T, ctx context.Context, compose composeCall, plan string) string {
	t.Helper()
	output, err := compose(ctx, "exec", "-T", "endpoint", "/usr/local/bin/ardents-bridge", "import", plan)
	if err != nil {
		t.Fatalf("run G3 import: %v\n%s", err, output)
	}
	return decodeHostileImportClass(t, output)
}

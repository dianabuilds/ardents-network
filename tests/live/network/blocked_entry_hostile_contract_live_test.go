//go:build live

package network_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

type blockedHostileContractResult struct {
	Kind, Cell, Terminal, Before, After, Receipt string
}

func TestBlockedEntryFinalHostileBindingAndPath(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	cell, terminal, ok := selectedHostileContractCell(os.Getenv("ARDENTS_FINAL_CELL"))
	if !ok {
		t.Skip("selected G6/G7 contract cell only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository := repositoryRoot(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s55-contract-%d:test", time.Now().UnixNano()))
	fixture := newBlockedEntryFixture(t, client, server)
	project := finalProjectName(fmt.Sprintf("ardents-s55-contract-%d", time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "final-hostile")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if ownedImage {
		if output, err := compose(ctx, "build", "endpoint"); err != nil {
			t.Fatalf("build hostile contract image: %v\n%s", err, output)
		}
	}
	started := time.Now()
	armFinalWorkerTerminal(terminal)
	var bypassReceipt []byte
	if strings.Contains(cell, "/G7-forbidden-path/") {
		if output, err := compose(ctx, "up", "-d", "--no-build", "--no-deps", "endpoint"); err != nil {
			t.Fatalf("start Endpoint for exact G7 component contract: %v\n%s", err, output)
		}
		bypassReceipt = runG7CandidateBypassContract(t, ctx, compose, strings.Split(cell, "/")[2])
	}
	startHostileInviteObservers(t, ctx, compose, fixture.root)
	output, err := compose(ctx, "exec", "-T", "-e", "ARDENTS_BLOCKED_ROLE=hostile-contract",
		"-e", "ARDENTS_HOSTILE_CELL="+cell, "endpoint", "/usr/local/bin/network-live.test",
		"-test.count=1", "-test.v", "-test.run", "^TestBlockedEntryRole$")
	if err != nil {
		t.Fatalf("execute hostile contract exercise: %v\n%s", err, output)
	}
	result := decodeBlockedHostileContractResult(t, output)
	if result.Cell != cell || result.Terminal != terminal || result.Before == "" || result.After == "" ||
		result.Receipt == "" {
		t.Fatalf("hostile contract result is incomplete: %+v", result)
	}
	receipt := []byte(result.Receipt)
	if strings.Contains(cell, "/G7-forbidden-path/") {
		var value finalForbiddenPathReceipt
		if json.Unmarshal(receipt, &value) != nil || len(value.CandidateContract) != 0 ||
			value.CandidateContractSHA256 != "" || value.InputSHA256 != "" {
			t.Fatalf("G7 worker receipt is invalid: %s", receipt)
		}
		digest := sha256.Sum256(bypassReceipt)
		var contract g7ComponentContract
		if json.Unmarshal(bypassReceipt, &contract) != nil || contract.Variant != value.Variant ||
			contract.Component != value.Component {
			t.Fatalf("G7 component contract is invalid: %s", bypassReceipt)
		}
		inputDigest := sha256.Sum256(contract.Input)
		value.CandidateContract = bypassReceipt
		value.CandidateContractSHA256 = hex.EncodeToString(digest[:])
		value.InputSHA256 = hex.EncodeToString(inputDigest[:])
		receipt, _ = json.Marshal(value)
	}
	recordFinalFault(cell, []byte(result.Before), []byte(result.After),
		receipt)
	publishFinalWorkerTerminal()
	stopHostileInviteObservers(t, ctx, compose, fixture.root)
	cleanup()
	emitFinalWorkerCell(t, cell, terminal, started, fixture.root)
}

func runG7CandidateBypassContract(t *testing.T, ctx context.Context, compose composeCall, variant string) []byte {
	t.Helper()
	arguments := []string{"exec", "-T", "-e", "ARDENTS_G7_VARIANT=" + variant, "endpoint"}
	if variant == "ordinary-entry" || variant == "direct-target" || variant == "shorter-route" {
		arguments = append(arguments, "/usr/local/bin/network-live.test", "-test.count=1", "-test.v",
			"-test.run", "^TestG7RouteEntryFailureHasNoAlternateDial$")
	} else {
		arguments = append(arguments[:len(arguments)-1], "-e", "ARDENTS_WEBTUNNEL_CLIENT=/candidate/webtunnel-client",
			"-e", "HTTP_PROXY=http://198.51.100.10:9", "-e", "HTTPS_PROXY=http://198.51.100.10:9",
			"-e", "ALL_PROXY=socks5://198.51.100.11:9", "-e", "NO_PROXY=", "endpoint",
			"/usr/local/bin/camouflage.test", "-test.count=1", "-test.v", "-test.run",
			"^TestPinnedClientUsesSanitizedPTAndOneNumericDialBeforeRefusal$")
	}
	output, err := compose(ctx, arguments...)
	if err != nil {
		t.Fatalf("execute exact G7 candidate bypass contract: %v\n%s", err, output)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if raw, ok := strings.CutPrefix(strings.TrimSpace(line), "g7-component-contract="); ok {
			return []byte(raw)
		}
	}
	t.Fatalf("G7 component contract omitted its canonical result: %s", output)
	return nil
}

func selectedHostileContractCell(cell string) (string, string, bool) {
	parts := strings.Split(cell, "/")
	if len(parts) != 4 || parts[0] != "hostile" {
		return "", "", false
	}
	episode, err := strconv.Atoi(parts[3])
	if err != nil || episode < 0 || episode >= 5 {
		return "", "", false
	}
	if parts[1] == "G6-substitution" && parts[2] != "network" && parts[2] != "route-profile" {
		return cell, "bridge-local-denial", true
	}
	if parts[1] == "G7-forbidden-path" {
		terminal := "bridge-attempt-exhausted"
		if parts[2] == "deadline-exposure-reset" {
			terminal = "bridge-deadline-exceeded"
		}
		return cell, terminal, true
	}
	return "", "", false
}

func decodeBlockedHostileContractResult(t *testing.T, output []byte) blockedHostileContractResult {
	t.Helper()
	for _, line := range strings.Split(string(output), "\n") {
		var value blockedHostileContractResult
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &value) == nil && value.Kind == "hostile-contract" {
			return value
		}
	}
	t.Fatalf("missing hostile contract result:\n%s", output)
	return blockedHostileContractResult{}
}

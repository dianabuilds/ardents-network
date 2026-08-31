//go:build referencec2 && (h4_3b_multihost || h4_8_a11)

package service_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

type h48A11ExpectedRemoteRoleExit struct {
	status int
	phase  string
}

type h48A11RemoteRoleExitReceipt struct {
	Schema         string `json:"schema"`
	Role           string `json:"role"`
	PID            int    `json:"pid"`
	ExitStatus     int    `json:"exit_status"`
	ExpectedStatus int    `json:"expected_status"`
	Phase          string `json:"phase"`
}

func h48A11ValidateRemoteRoleExitStatuses(raw []byte, expected map[string]h48A11ExpectedRemoteRoleExit) error {
	if len(raw) == 0 || len(raw) > 64<<10 || len(expected) == 0 {
		return fmt.Errorf("remote role exit-status evidence is empty or outside its bound")
	}
	observed := make(map[string]struct{}, len(expected))
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			return fmt.Errorf("remote role exit-status evidence contains an empty line")
		}
		decoder := json.NewDecoder(strings.NewReader(line))
		decoder.DisallowUnknownFields()
		var receipt h48A11RemoteRoleExitReceipt
		if err := decoder.Decode(&receipt); err != nil {
			return fmt.Errorf("decode remote role exit-status receipt: %w", err)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return fmt.Errorf("remote role exit-status receipt has trailing JSON")
		}
		want, exists := expected[receipt.Role]
		if !exists || receipt.Schema != "ardents-h4-8-a11-remote-role-exit-v1" || receipt.PID <= 1 ||
			receipt.ExitStatus != want.status || receipt.ExpectedStatus != want.status || receipt.Phase != want.phase {
			return fmt.Errorf("remote role exit-status receipt is not exact: %+v", receipt)
		}
		if _, duplicate := observed[receipt.Role]; duplicate {
			return fmt.Errorf("remote role exit-status receipt is duplicated for %q", receipt.Role)
		}
		observed[receipt.Role] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan remote role exit-status evidence: %w", err)
	}
	if len(observed) != len(expected) {
		return fmt.Errorf("remote role exit-status inventory count = %d, want %d", len(observed), len(expected))
	}
	return nil
}

func h48A11ExpectedRemoteRoleExits(scenario referenceC2Scenario) map[string]h48A11ExpectedRemoteRoleExit {
	completion := h48A11ExpectedRemoteRoleExit{status: 0, phase: "completion"}
	expected := map[string]h48A11ExpectedRemoteRoleExit{
		"source-a":      {status: 0, phase: "cleanup"},
		"source-b":      {status: 0, phase: "cleanup"},
		"initiator":     completion,
		"introduction":  completion,
		"responder":     completion,
		"gateway":       completion,
		"publisher":     completion,
		"publisher-app": completion,
		"alpha-gateway": completion,
		"alpha-relay":   completion,
	}
	if scenario.productRendezvousRelay {
		expected["rendezvous-node"] = completion
		expected["carrier-relay"] = completion
	} else {
		expected["rendezvous"] = completion
	}
	if scenario.publisherTerminal == referenceC2PublisherEndpointStop {
		expected["publisher"] = h48A11ExpectedRemoteRoleExit{status: 137, phase: "fault"}
	}
	if scenario.publisherTerminal == referenceC2PublisherApplicationReset ||
		scenario.publisherTerminal == referenceC2PublisherEndpointStop || scenario.transitFault != "" {
		expected["publisher-app"] = h48A11ExpectedRemoteRoleExit{status: 2, phase: "terminal"}
	}
	if scenario.transitFault == referenceC2TransitFaultProductNodeLoss {
		expected["rendezvous-node"] = h48A11ExpectedRemoteRoleExit{status: 137, phase: "fault"}
	}
	return expected
}

func h48A11AssertRemoteRoleExitStatuses(t *testing.T, remote h43RemoteC2, scenario referenceC2Scenario) {
	t.Helper()
	raw, err := remote.readFile(t, remote.environment.remoteDirectory+"/remote-role-exit-statuses.jsonl")
	if err != nil {
		t.Fatalf("read A11 remote role exit-status evidence: %v", err)
	}
	if err := h48A11ValidateRemoteRoleExitStatuses(raw, h48A11ExpectedRemoteRoleExits(scenario)); err != nil {
		t.Fatalf("A11 remote role exit-status evidence: %v\n%s", err, raw)
	}
}

func TestH48A11RemoteRoleExitStatusesRejectWrongStatus(t *testing.T) {
	expected := map[string]h48A11ExpectedRemoteRoleExit{
		"source-a": {status: 0, phase: "cleanup"},
	}
	wrong := []byte(`{"schema":"ardents-h4-8-a11-remote-role-exit-v1","role":"source-a","pid":41,"exit_status":2,"expected_status":0,"phase":"cleanup"}` + "\n")
	if err := h48A11ValidateRemoteRoleExitStatuses(wrong, expected); err == nil {
		t.Fatal("A11 remote exit-status oracle accepted a wrong source role status")
	}
}

func TestH48A11RemoteRoleExitStatusesAcceptExactInventory(t *testing.T) {
	expected := map[string]h48A11ExpectedRemoteRoleExit{
		"source-a":      {status: 0, phase: "cleanup"},
		"publisher-app": {status: 2, phase: "terminal"},
	}
	exact := []byte(
		`{"schema":"ardents-h4-8-a11-remote-role-exit-v1","role":"publisher-app","pid":52,"exit_status":2,"expected_status":2,"phase":"terminal"}` + "\n" +
			`{"schema":"ardents-h4-8-a11-remote-role-exit-v1","role":"source-a","pid":41,"exit_status":0,"expected_status":0,"phase":"cleanup"}` + "\n",
	)
	if err := h48A11ValidateRemoteRoleExitStatuses(exact, expected); err != nil {
		t.Fatalf("A11 exact remote exit-status inventory: %v", err)
	}
}

func TestH48A11ExpectedRemoteRoleExitStatusesBindFaultAndCleanup(t *testing.T) {
	expected := h48A11ExpectedRemoteRoleExits(referenceC2Scenario{productRendezvousRelay: true,
		transitFault: referenceC2TransitFaultCarrierLoss})
	if len(expected) != 12 || expected["source-a"] != (h48A11ExpectedRemoteRoleExit{status: 0, phase: "cleanup"}) ||
		expected["source-b"] != (h48A11ExpectedRemoteRoleExit{status: 0, phase: "cleanup"}) ||
		expected["publisher-app"] != (h48A11ExpectedRemoteRoleExit{status: 2, phase: "terminal"}) ||
		expected["carrier-relay"] != (h48A11ExpectedRemoteRoleExit{status: 0, phase: "completion"}) {
		t.Fatalf("A11 Carrier-loss remote exit inventory = %+v", expected)
	}
}

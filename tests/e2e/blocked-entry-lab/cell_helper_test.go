package blocked_entry_lab_test

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

type helperPlan struct {
	Schema           string `json:"schema"`
	EventID          string `json:"event_id"`
	Variant          string `json:"variant"`
	ExpectedTerminal string `json:"expected_terminal"`
}

type cellHelperObservation struct {
	Schema               string           `json:"schema"`
	EventID              string           `json:"event_id"`
	ObservedTerminal     string           `json:"observed_terminal"`
	ProductStarted       bool             `json:"product_started"`
	FaultInjected        bool             `json:"fault_injected"`
	FaultOwner           string           `json:"fault_owner"`
	Attribution          string           `json:"attribution"`
	AttributionEvidence  string           `json:"attribution_evidence"`
	Diagnostic           string           `json:"diagnostic"`
	StartedOffsetMillis  uint64           `json:"started_offset_millis"`
	TerminalOffsetMillis uint64           `json:"terminal_offset_millis"`
	CleanupOffsetMillis  uint64           `json:"cleanup_offset_millis"`
	AdapterCleanupMillis uint64           `json:"adapter_cleanup_millis"`
	Observers            []helperObserver `json:"observers"`
	Residuals            []helperResidual `json:"residuals"`
}

type helperObserver struct {
	Boundary             string `json:"boundary"`
	IPv4UDPControl       bool   `json:"ipv4_udp_control"`
	IPv6UDPControl       bool   `json:"ipv6_udp_control"`
	IPv4TCPControl       bool   `json:"ipv4_tcp_control"`
	Attribution          string `json:"attribution"`
	ForbiddenPackets     uint64 `json:"forbidden_packets"`
	ForbiddenOwner       string `json:"forbidden_owner"`
	UnclassifiedPackets  uint64 `json:"unclassified_packets"`
	ObservationCompleted bool   `json:"observation_completed"`
}

type helperResidual struct {
	Kind                string `json:"kind"`
	Count               uint64 `json:"count"`
	Owner               string `json:"owner"`
	AttributionEvidence string `json:"attribution_evidence"`
}

func runCellHelper() int {
	decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	mode, ordinal := os.Getenv("ARDENTS_BLOCKED_MODE"), 0
	for {
		var input helperPlan
		if err := decoder.Decode(&input); err == io.EOF {
			if encoder.Encode(cellHelperObservation{Schema: "ardents-h3-blocked-campaign-closed-v1"}) != nil {
				return 1
			}
			return 0
		} else if err != nil || input.Schema != "ardents-h3-blocked-cell-plan-v1" {
			return 1
		}
		cell := cellHelperObservation{Schema: "ardents-h3-blocked-cell-observation-v1",
			EventID: input.EventID, ObservedTerminal: input.ExpectedTerminal, ProductStarted: true,
			FaultInjected: true, FaultOwner: "none", Attribution: "exact",
			Diagnostic:          "fixture process exercised scheduled fault",
			StartedOffsetMillis: uint64(ordinal * 3), TerminalOffsetMillis: uint64(ordinal*3 + 1),
			CleanupOffsetMillis: uint64(ordinal*3 + 2), AdapterCleanupMillis: 1,
			Observers: cleanHelperObservers(), Residuals: cleanHelperResiduals()}
		first := ordinal == 0
		if first && (mode == "candidate-fail" || mode == "candidate-fail-harness-invalid") {
			cell.ObservedTerminal, cell.FaultOwner = "unexpected-success", "candidate"
			cell.Diagnostic = "trustworthy candidate gate failure"
		}
		if first && (mode == "harness-invalid" || mode == "candidate-fail-harness-invalid") {
			cell.Observers[0].Attribution, cell.Observers[0].ObservationCompleted = "ambiguous", false
		}
		if first && mode == "candidate-residual" {
			cell.Residuals[0].Count, cell.Residuals[0].Owner = 1, "candidate"
		}
		if first && mode == "candidate-forbidden" {
			cell.Observers[0].ForbiddenPackets, cell.Observers[0].ForbiddenOwner = 1, "candidate"
		}
		if first && mode == "forbidden-owner-mismatch" {
			cell.Observers[0].ForbiddenPackets, cell.Observers[0].ForbiddenOwner = 1, "candidate"
		}
		if first && mode == "cell-inventory-missing" {
			cell.Residuals = cell.Residuals[1:]
		}
		if mode == "collector-loss" && strings.Contains(input.EventID, "/collector-loss/0") {
			cell.Observers[0].ObservationCompleted = false
		}
		if mode == "blocker-loss" && strings.Contains(input.EventID, "/blocker-loss/0") {
			cell.Observers[0].ObservationCompleted = false
		}
		if variant := candidateCanaryVariant(mode); variant != "" &&
			strings.Contains(input.EventID, "/"+variant+"/0") {
			cell.FaultOwner = "candidate"
			cell.Diagnostic = "candidate leak: " + canariesForVariant(variant)
		}
		if err := encoder.Encode(cell); err != nil {
			return 1
		}
		ordinal++
	}
}

func cleanHelperObservers() []helperObserver {
	boundaries := []string{"endpoint-adapter", "tls-front", "webtunnel-server", "bridge-next-leg",
		"publisher-endpoint", "ordinary-initiator", "ordinary-introduction", "ordinary-rendezvous",
		"ordinary-responder"}
	result := make([]helperObserver, 0, len(boundaries))
	for _, boundary := range boundaries {
		result = append(result, helperObserver{Boundary: boundary, IPv4UDPControl: true,
			IPv6UDPControl: true, IPv4TCPControl: true, Attribution: "exact", ForbiddenOwner: "none",
			ObservationCompleted: true})
	}
	return result
}

func cleanHelperResiduals() []helperResidual {
	kinds := []string{"process", "listener", "socket", "namespace", "mount", "file", "queue", "timer",
		"cgroup", "publishable-secret"}
	result := make([]helperResidual, 0, len(kinds))
	for _, kind := range kinds {
		result = append(result, helperResidual{Kind: kind, Owner: "none"})
	}
	return result
}

func candidateCanaryVariant(mode string) string {
	if mode == "candidate-canary" {
		return "candidate-leak-invite"
	}
	if strings.HasPrefix(mode, "candidate-canary-") {
		return "candidate-leak-" + strings.TrimPrefix(mode, "candidate-canary-")
	}
	return ""
}

func canariesForVariant(variant string) string {
	var corpus struct {
		Sets []struct {
			Variant     string `json:"variant"`
			Invite      string `json:"invite"`
			Address     string `json:"address"`
			Path        string `json:"path"`
			Certificate string `json:"certificate"`
		} `json:"sets"`
	}
	raw, err := os.ReadFile(os.Getenv("ARDENTS_BLOCKED_CANARY_FILE"))
	if err != nil || json.Unmarshal(raw, &corpus) != nil {
		return "missing-canary"
	}
	for _, set := range corpus.Sets {
		if set.Variant == variant {
			return strings.Join([]string{set.Invite, set.Address, set.Path, set.Certificate}, " ")
		}
	}
	return "missing-canary"
}

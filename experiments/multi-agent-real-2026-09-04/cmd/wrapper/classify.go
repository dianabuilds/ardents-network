//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

func classifyRefresh(persona personaConfig, output []byte, exitCode int, runErr error) eventRecord {
	if runErr != nil || exitCode != 0 {
		message := strings.TrimSpace(string(output))
		if message == "" && runErr != nil {
			message = runErr.Error()
		}
		if len(message) > 4096 {
			message = message[:4096]
		}
		kind, diagnostic := classifyFailure(message)
		return eventRecord{Kind: kind, Diagnostic: diagnostic, ExitCode: exitCode, Error: message}
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	var source sourceEvent
	if err := decoder.Decode(&source); err != nil {
		return eventRecord{Kind: "infra_error", Diagnostic: "event_parse", Error: "refresh output is not one source event"}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return eventRecord{Kind: "infra_error", Diagnostic: "event_parse", Error: "refresh output has trailing content"}
	}
	outcomes := source.SourceOutcomes
	result := eventRecord{Generation: source.Generation, ActualOutcomes: &outcomes}
	if source.Schema != "ardents-source-event-v1" || source.Kind != "source-wave-accepted" || source.Generation == "" || source.Epoch == 0 {
		result.Kind, result.Diagnostic, result.Error = "harness_abort", "unexpected_source_event", "refresh output has the wrong event identity"
		return result
	}
	if source.SourceOutcomes != persona.ExpectedOutcomes {
		result.Kind, result.Diagnostic, result.Error = "harness_abort", "unexpected_source_signature", "source outcomes do not match the persona contract"
		return result
	}
	result.Kind = persona.ExpectedKind
	return result
}

func classifyFailure(message string) (string, string) {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "local role record limit exceeded"):
		return "harness_abort", "record_limit"
	case strings.Contains(lower, "local role producer limit exceeded"):
		return "harness_abort", "producer_limit"
	case strings.Contains(lower, "local role duty conflicts with retained state"):
		return "harness_abort", "conflict"
	case strings.Contains(lower, "local role state exceeds its bound"):
		return "harness_abort", "legacy_local_role_validation"
	case strings.Contains(lower, "direct-source exposure set is full"):
		return "harness_abort", "installation_source_exhausted"
	case strings.Contains(lower, "resource temporarily unavailable"):
		return "harness_abort", "lease_contention"
	case strings.Contains(lower, "retry is in durable backoff"):
		return "infra_error", "durable_backoff"
	case strings.Contains(lower, "durable source cycle reached its recorded deadline"):
		return "infra_error", "cycle_deadline"
	case strings.Contains(lower, "finite sources are temporarily unavailable"):
		return "infra_error", "source_unavailable"
	case strings.Contains(lower, "no such container"):
		return "infra_error", "container_missing"
	default:
		return "harness_abort", "unrecognized"
	}
}

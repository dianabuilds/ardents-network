//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const (
	manifestSchema = "ardents-multi-agent-run-v1"
	actionSchema   = "ardents-agent-action-v1"
	eventSchema    = "ardents-multi-agent-event-v1"

	defaultContainer         = "ardents-multi-agent-real-agent-executor-1"
	defaultContainerEvidence = "/workspace/evidence"
)

type runManifest struct {
	Schema                string                   `json:"schema"`
	RunID                 string                   `json:"run_id"`
	CreatedAt             string                   `json:"created_at"`
	HostRunRoot           string                   `json:"host_run_root"`
	Container             string                   `json:"container"`
	ContainerEvidenceRoot string                   `json:"container_evidence_root"`
	Personas              map[string]personaConfig `json:"personas"`
}

type personaConfig struct {
	Name               string    `json:"name"`
	StateRoot          string    `json:"state_root"`
	LocalRoleStateRoot string    `json:"local_role_state_root"`
	SourcePlan         string    `json:"source_plan"`
	ExpectedKind       string    `json:"expected_kind"`
	ExpectedOutcomes   [4]string `json:"expected_outcomes"`
	MinimumEvents      int       `json:"minimum_events"`
	MinimumSpanSeconds int       `json:"minimum_span_seconds"`
	AllowNoop          bool      `json:"allow_noop"`
	SourcePlanHash     string    `json:"source_plan_hash"`
	ConfigurationHash  string    `json:"configuration_hash"`
}

type actionRequest struct {
	Schema string `json:"schema"`
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
}

type eventRecord struct {
	Schema            string     `json:"schema"`
	RunID             string     `json:"run_id"`
	Persona           string     `json:"persona"`
	Sequence          uint64     `json:"sequence"`
	RecordedAt        string     `json:"recorded_at,omitempty"`
	Action            string     `json:"action,omitempty"`
	Reason            string     `json:"reason,omitempty"`
	Kind              string     `json:"kind"`
	Diagnostic        string     `json:"diagnostic,omitempty"`
	ConfigurationHash string     `json:"configuration_hash,omitempty"`
	Generation        string     `json:"generation,omitempty"`
	ActualOutcomes    *[4]string `json:"actual_outcomes,omitempty"`
	ExitCode          int        `json:"exit_code,omitempty"`
	Error             string     `json:"error,omitempty"`
}

type sourceEvent struct {
	Schema             string    `json:"schema"`
	Kind               string    `json:"kind"`
	Generation         string    `json:"generation"`
	Epoch              uint64    `json:"epoch"`
	SourceAttempts     uint16    `json:"source_attempts"`
	SourceOutcomes     [4]string `json:"source_outcomes"`
	LatestCompleteness string    `json:"latest_completeness"`
}

type personaDefinition struct {
	name             string
	fixture          string
	expectedKind     string
	expectedOutcomes [4]string
	minimumEvents    int
	minimumSpan      int
	allowNoop        bool
}

var personaDefinitions = []personaDefinition{
	{"honest_user", "client.json", "accept", [4]string{"valid", "valid", "not-attempted", "not-attempted"}, 10, 300, true},
	{"battery_saver", "client.json", "accept", [4]string{"valid", "valid", "not-attempted", "not-attempted"}, 3, 300, true},
	{"probe_consumer", "client-probe.json", "reject", [4]string{"valid", "invalid-state", "not-attempted", "not-attempted"}, 5, 180, true},
}

func decodeAction(raw []byte) (actionRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var request actionRequest
	if err := decoder.Decode(&request); err != nil {
		return actionRequest{}, errors.New("agent action is not canonical JSON")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return actionRequest{}, errors.New("agent action has trailing content")
	}
	if request.Schema != actionSchema || request.Action != "refresh" && request.Action != "noop" {
		return actionRequest{}, errors.New("agent action is not supported")
	}
	if len(request.Reason) > 240 || strings.ContainsAny(request.Reason, "\r\n\x00") {
		return actionRequest{}, errors.New("agent action reason is not bounded")
	}
	return request, nil
}

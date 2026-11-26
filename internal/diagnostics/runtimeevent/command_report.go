package runtimeevent

import (
	"bytes"
	"encoding/json"
	"errors"
)

// CommandReportSchema identifies the collector's normalized projection of
// current command reports. Producers do not declare this schema. Their original
// operation/phase/outcome report bytes remain a separate compatibility input.
const CommandReportSchema = "ardents-command-report-v1"

type commandTransition struct{ phase, outcome string }
type commandReport struct {
	operation, owner string
	transitions      []commandTransition
}

var nodeTransitions = []commandTransition{
	{"refused", "unsupported-platform"},
	{"refused", "canceled"},
	{"refused", "state-unavailable"},
	{"refused", "startup-unavailable"},
	{"listening", "original-duty"},
	{"retiring", "completion-unavailable"},
	{"joined", "failed"},
	{"joined", "canceled"},
}
var sourceTransitions = []commandTransition{
	{"refused", "failed"},
	{"listening", "original-source"},
	{"retiring", "completion-unavailable"},
	{"joined", "failed"},
	{"joined", "canceled"},
	{"completion", "failed"},
	{"repair", "failed"},
}
var commandReports = []commandReport{
	{"node.forwarding", "node", nodeTransitions},
	{"node.issuer", "node", nodeTransitions},
	{"node.descriptor", "node", nodeTransitions},
	{"node.introduction", "node", nodeTransitions},
	{"node.join", "node", nodeTransitions},
	{"source.serve", "source", sourceTransitions},
	// Non-Linux dispatch retains this exact operation spelling. Linux serving
	// reports both fresh and retained invocations as source.serve.
	{"source", "source", []commandTransition{{"refused", "unsupported-platform"}}},
}

func commandSpec(operation string) (commandReport, bool) {
	for _, report := range commandReports {
		if report.operation == operation {
			return report, true
		}
	}
	return commandReport{}, false
}

// CommandObservation contains canonical categories for one current report.
// It grants neither runtime authority nor proof of readiness or joined cleanup.
type CommandObservation struct {
	Operation, Owner, Phase, Outcome, Failure string
}

// ClassifyCommand validates operation-specific phase/outcome pairs and returns
// only catalogue-owned strings. Unknown input returns no retained observation.
func ClassifyCommand(operation, phase, outcome string) (CommandObservation, bool) {
	spec, ok := commandSpec(operation)
	if !ok {
		return CommandObservation{}, false
	}
	for _, transition := range spec.transitions {
		if transition.phase != phase || transition.outcome != outcome {
			continue
		}
		observation := CommandObservation{Operation: spec.operation, Owner: spec.owner, Phase: transition.phase, Outcome: transition.outcome}
		// Cancellation and a listening announcement are distinct from a refusal,
		// failed invocation or unavailable physical completion.
		switch transition.outcome {
		case "unsupported-platform", "state-unavailable", "startup-unavailable", "completion-unavailable", "failed":
			observation.Failure = transition.outcome
		}
		return observation, true
	}
	return CommandObservation{}, false
}

func decodeCommandReport(raw []byte, fields map[string]json.RawMessage, record Record) (Record, bool, error) {
	spec, ok := commandSpec(stringValue(fields, "operation"))
	if !ok {
		return Record{}, false, nil
	}
	if !uniqueCommandFields(raw) {
		return Record{}, false, errors.New("command diagnostic has ambiguous fields")
	}
	var phase, outcome string
	for name, destination := range map[string]*string{"phase": &phase, "outcome": &outcome} {
		var text *string
		if json.Unmarshal(fields[name], &text) != nil || text == nil {
			return Record{}, false, errors.New("command diagnostic contains an invalid field type")
		}
		*destination = *text
	}
	if observation, valid := ClassifyCommand(spec.operation, phase, outcome); valid {
		record.Schema, record.Kind, record.Owner = CommandReportSchema, observation.Operation, observation.Owner
		record.Phase, record.Outcome = observation.Phase, observation.Outcome
		// Reports have no event time or measured resources. Never manufacture
		// them from collector receipt time or private command detail.
		return record, true, nil
	}
	return Record{}, false, errors.New("command diagnostic contains an invalid transition")
}

// Current command reports never repeat keys. Refuse ambiguity instead of
// selecting the last declaration and turning a failure into a listening fact.
func uniqueCommandFields(raw []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return false
	}
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] {
			return false
		}
		seen[name] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return false
		}
	}
	return true
}

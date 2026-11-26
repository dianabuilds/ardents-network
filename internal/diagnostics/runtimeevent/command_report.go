package runtimeevent

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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
var commandResults = []commandTransition{
	{"command", "completed"}, {"command", "failed"},
	{"command", "usage-or-output-failure"}, {"command", "canceled"},
}
var conversationResults = []commandTransition{
	{"closed", "completed"}, {"closed", "failed"},
	{"closed", "usage-or-output-failure"}, {"closed", "canceled"},
}
var endpointUnavailable = []commandTransition{
	{"installed-startup", "unavailable"}, {"installed-declarations", "unavailable"},
	{"reader-preparation", "unavailable"}, {"publisher-preparation", "unavailable"},
	{"source-configuration", "unavailable"}, {"network-open", "unavailable"},
	{"network-refresh", "unavailable"}, {"permission-bootstrap", "unavailable"},
	{"service-route", "unavailable"}, {"administration-listener", "unavailable"},
	{"reader-listener", "unavailable"}, {"running", "unavailable"},
	{"permission-pending", "pending"}, {"permission-accepted", "accepted"},
	{"joined", "completed"}, {"joined", "canceled"},
	{"refused", "unsupported-platform"},
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
	{"endpoint.start-installed", "endpoint", endpointUnavailable},
	{"endpoint.prepare-history", "endpoint", []commandTransition{
		{"history-preparation", "prepared"}, {"history-preparation", "unavailable"}, {"refused", "unsupported-platform"},
	}},
	{"endpoint.construct-plan", "endpoint", []commandTransition{{"plan-construction", "constructed"}, {"plan-construction", "unavailable"}, {"refused", "unsupported-platform"}}},
	{"endpoint", "endpoint", []commandTransition{{"refused", "unsupported-platform"}}},
	{"endpoint.withdraw", "endpoint", commandResults},
	{"application.read", "application", commandResults},
	{"application.publish", "application", commandResults},
	{"application.link", "application", commandResults},
	{"application.stream.connect", "application", commandResults},
	{"application.stream.publish", "application", commandResults},
	{"admission.allocate", "admission", commandResults},
	{"admission.issue-current", "admission", commandResults},
	{"network.accept-epoch", "network", commandResults},
	{"network.prepare-initial-keys", "network", commandResults},
	{"network.prepare-initial-node-record", "network", commandResults},
	{"network.prepare-initial-epoch", "network", commandResults},
	{"network.prepare-closed-profile", "network", commandResults},
	{"network.sign-initial-node-record", "network", commandResults},
	{"network.sign-initial-epoch", "network", commandResults},
	{"network.sign-closed-profile", "network", commandResults},
	{"network.prepare-node-certificate", "network", commandResults},
	{"network.accept-profile", "network", commandResults},
	{"network.inspect", "network", commandResults},
	{"network.inspect-closed-profile", "network", commandResults},
	{"enrollment.verify", "enrollment", commandResults},
	{"enrollment.artifact-name", "enrollment", commandResults},
	{"enrollment.check-candidate", "enrollment", commandResults},
	{"enrollment.check-protected-generation", "enrollment", commandResults},
	{"release.verify-initial", "release", commandResults},
	{"publication.prepare-root", "publication", commandResults},
	{"publication.instance-initialize", "publication", commandResults},
	{"publication.instance-request", "publication", commandResults},
	{"publication.instance-accept", "publication", commandResults},
	{"execution.prepare-permission", "execution", commandResults},
	{"execution.route-holder", "execution", commandResults},
	{"route.receive", "route", []commandTransition{
		{"listening", "original-duty"}, {"joined", "failed"}, {"joined", "canceled"},
	}},
	{"route", "route", []commandTransition{{"refused", "unsupported-platform"}}},
	{"admission.holder", "admission", conversationResults},
	{"admission.receiver", "admission", conversationResults},
	{"custody.create-service-authority", "custody", commandResults},
	{"custody.issue-service-credential", "custody", commandResults},
	{"custody.create-admission-authority", "custody", commandResults},
	{"custody.issue-admission-permission", "custody", commandResults},
	{"custody.verify-record", "custody", commandResults},
	{"custody.export-recovery-bundle", "custody", commandResults},
	{"custody.restore-recovery-bundle", "custody", commandResults},
	{"custody.purge-record", "custody", commandResults},
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
	if spec, ok := commandSpec(operation); ok {
		for _, transition := range spec.transitions {
			if transition.phase == phase && transition.outcome == outcome {
				return commandObservation(spec.operation, spec.owner, transition.phase, transition.outcome), true
			}
		}
		return CommandObservation{}, false
	}
	// OTLP and local command projections share this exact category authority.
	selected := LookupOperation(operation)
	completion := selected.Classify(phase, outcome)
	phase, outcome, valid := completion.Labels(selected)
	if !valid {
		return CommandObservation{}, false
	}
	spec, _ := selected.spec()
	owner := "issuance"
	switch selected {
	case AdmissionInitialize, AdmissionPrepareBinding:
		owner = "admission"
	case HostingInitialize, HostingObserve:
		owner = "hosting"
	case IdentityImport:
		owner = "node"
	}
	return commandObservation(spec.name, owner, phase, outcome), true
}

func commandObservation(operation, owner, phase, outcome string) CommandObservation {
	observation := CommandObservation{Operation: operation, Owner: owner, Phase: phase, Outcome: outcome}
	switch outcome {
	case "unsupported-platform", "state-unavailable", "startup-unavailable", "completion-unavailable", "failed", "unavailable",
		"usage-or-output-failure", "invalid-input", "storage-uncertain", "busy", "request-conflict", "outside-validity", "result-capacity",
		"storage-unavailable", "quota-exhausted", "malformed-permission", "invalid-signature", "binding-mismatch", "request-limit",
		"budget-exhausted", "budget-unavailable":
		observation.Failure = outcome
	}
	return observation
}

// WriteCommand emits only catalogue-owned fields on a caller-selected local
// writer. It opens no exporter and returns a finite error for an unknown tuple.
func WriteCommand(out io.Writer, operation, phase, outcome string) error {
	observation, valid := ClassifyCommand(operation, phase, outcome)
	if !valid || out == nil {
		return errors.New("command diagnostic unavailable")
	}
	return json.NewEncoder(out).Encode(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{observation.Operation, observation.Phase, observation.Outcome})
}

// CommandExit classifies a command boundary, never the health or durable
// effects of a domain. A failed receipt can follow an already committed effect.
func CommandExit(code int) string {
	switch code {
	case 0:
		return "completed"
	case 2:
		return "usage-or-output-failure"
	case 130:
		return "canceled"
	default:
		return "failed"
	}
}

func decodeCommandReport(raw []byte, fields map[string]json.RawMessage, record Record) (Record, bool, error) {
	operationKey, phaseKey, outcomeKey := "operation", "phase", "outcome"
	if _, lower := fields[operationKey]; !lower {
		operationKey, phaseKey, outcomeKey = "Operation", "Phase", "Outcome"
	}
	operation := stringValue(fields, operationKey)
	_, local := commandSpec(operation)
	if !local && LookupOperation(operation) == 0 {
		return Record{}, false, nil
	}
	// Only the existing Admission/Hosting OTLP adapters use uppercase fields.
	selected := LookupOperation(operation)
	if operationKey == "Operation" && selected != AdmissionInitialize && selected != HostingInitialize && selected != HostingObserve {
		return Record{}, false, nil
	}
	if !uniqueCommandFields(raw) {
		return Record{}, false, errors.New("command diagnostic has ambiguous fields")
	}
	for _, pair := range [][2]string{{"operation", "Operation"}, {"phase", "Phase"}, {"outcome", "Outcome"}} {
		_, lower := fields[pair[0]]
		_, upper := fields[pair[1]]
		if lower && upper {
			return Record{}, false, errors.New("command diagnostic has ambiguous fields")
		}
	}
	var phase, outcome string
	for name, destination := range map[string]*string{phaseKey: &phase, outcomeKey: &outcome} {
		if name == outcomeKey && (operation == "admission.holder" || operation == "admission.receiver") {
			var code *int
			if json.Unmarshal(fields["exit_code"], &code) != nil || code == nil || (*code != 0 && *code != 1 && *code != 2 && *code != 130) {
				return Record{}, false, errors.New("command diagnostic contains an invalid exit category")
			}
			outcome = CommandExit(*code)
			if declared, exists := fields[outcomeKey]; exists {
				var reported *string
				if json.Unmarshal(declared, &reported) != nil || reported == nil || *reported != outcome {
					return Record{}, false, errors.New("command diagnostic has conflicting results")
				}
			}
			continue
		}
		var text *string
		if json.Unmarshal(fields[name], &text) != nil || text == nil {
			return Record{}, false, errors.New("command diagnostic contains an invalid field type")
		}
		*destination = *text
	}
	if observation, valid := ClassifyCommand(operation, phase, outcome); valid {
		record.Schema, record.Kind, record.Owner = CommandReportSchema, observation.Operation, observation.Owner
		record.Phase, record.Outcome = observation.Phase, observation.Outcome
		if LookupOperation(operation) != 0 {
			key := "telemetry"
			other := "Telemetry"
			if operationKey == "Operation" {
				key, other = other, key
			}
			if _, exists := fields[other]; exists {
				return Record{}, false, errors.New("command diagnostic has ambiguous delivery spelling")
			}
			if rawDelivery, exists := fields[key]; exists {
				var delivery *string
				if json.Unmarshal(rawDelivery, &delivery) != nil || delivery == nil || !KnownExportDelivery(*delivery) {
					return Record{}, false, errors.New("command diagnostic has invalid export delivery")
				}
				record.Delivery = *delivery
			}
		}

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

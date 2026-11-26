package runtimeevent

// Operation identifies an existing instrumented command, not domain authority.
// Its zero value and unregistered values cannot be exported.
type Operation uint8

const (
	AdmissionInitialize Operation = iota + 1
	HostingInitialize
	HostingObserve
	IssuanceInitialize
	IssuanceInspect
	IssuanceInitializeResults
	IssuanceIssue
	IssuanceInitializeProfile
	IssuanceInspectProfile
	IdentityImport
	AdmissionPrepareBinding
)

type operationSpec struct {
	scope, name      string
	phases, outcomes []string
}

// These spellings preserve existing OTLP compatibility. Domain owners still
// classify their own results; this catalogue admits only reviewed export labels.
var operationSpecs = [...]operationSpec{
	{},
	AdmissionInitialize: {admissionScope, "admission.initialize", []string{"input", "execute"}, []string{
		"initialized-offline", "invalid-input", "canceled", "storage-uncertain",
		"unsupported-platform", "busy", "storage-unavailable",
	}},
	HostingInitialize:         {hostingScope, "hosting.initialize", hostingPhases, hostingOutcomes},
	HostingObserve:            {hostingScope, "hosting.observe", hostingPhases, hostingOutcomes},
	IssuanceInitialize:        {issuanceScope, "issuance.initialize", inventoryInitializePhases, inventoryOutcomes},
	IssuanceInspect:           {issuanceScope, "issuance.inspect", inventoryInspectPhases, inventoryOutcomes},
	IssuanceInitializeResults: {issuanceScope, "issuance.initialize-results", resultInitializePhases, resultInitializeOutcomes},
	IssuanceIssue:             {issuanceScope, "issuance.issue", resultIssuePhases, resultIssueOutcomes},
	IssuanceInitializeProfile: {issuanceScope, "issuance.initialize-profile", profilePhases, profileInitializeOutcomes},
	IssuanceInspectProfile:    {issuanceScope, "issuance.inspect-profile", profilePhases, profileInspectOutcomes},
	IdentityImport:            {issuanceScope, "node-identity.import", []string{"input", "import"}, identityOutcomes},
	AdmissionPrepareBinding:   {issuanceScope, "admission.prepare-binding", []string{"input", "prepare-binding", "export"}, bindingOutcomes},
}

const (
	admissionScope = "ardents.admission"
	hostingScope   = "ardents.hosting"
	issuanceScope  = "ardents.issuance"
)

var (
	hostingPhases   = []string{"input", "execute", "close"}
	hostingOutcomes = []string{
		"completed", "storage-uncertain", "unsupported-platform", "canceled",
		"invalid-input", "budget-exhausted", "budget-unavailable",
	}
	inventoryInitializePhases = []string{"input", "initialize", "open", "inventory", "export", "close"}
	inventoryInspectPhases    = []string{"input", "open", "inventory", "export", "close"}
	// ErrorOutcome and category are the current offline Issuance classifiers.
	issuanceRefusals = []string{
		"storage-uncertain", "unsupported-platform", "busy", "request-conflict",
		"outside-validity", "result-capacity", "canceled", "invalid-input", "storage-unavailable",
	}
	inventoryOutcomes      = withOutcomes(issuanceRefusals, "completed")
	resultInitializePhases = []string{
		"input", "open-admission", "open-keys", "open-results", "close-keys", "close-admission",
	}
	resultIssuePhases = []string{
		"input", "open-admission", "open-keys", "open-results", "debit", "issue",
		"export", "close-results", "close-keys", "close-admission",
	}
	resultInitializeOutcomes = withOutcomes(issuanceRefusals, "initialized-results")
	resultIssueOutcomes      = withOutcomes(issuanceRefusals,
		"issued-offline", "already-issued", "quota-exhausted", "malformed-permission",
		"invalid-signature", "binding-mismatch", "request-limit",
	)
	profilePhases = []string{
		"input", "open-identity", "open-keys", "profile", "export",
		"close-profile", "close-keys", "close-identity",
	}
	profileInitializeOutcomes = withOutcomes(issuanceRefusals, "profile-initialized")
	profileInspectOutcomes    = withOutcomes(issuanceRefusals, "profile-verified")
	identityOutcomes          = withOutcomes(issuanceRefusals, "identity-imported")
	bindingOutcomes           = withOutcomes(issuanceRefusals, "binding-prepared")
)

func withOutcomes(refusals []string, success ...string) []string {
	return append(append([]string(nil), refusals...), success...)
}

// LookupOperation resolves a command name through the finite export catalogue.
// Unknown names return the invalid zero value; the input is never retained.
func LookupOperation(name string) Operation {
	for i := 1; i < len(operationSpecs); i++ {
		if operationSpecs[i].name == name {
			return Operation(i)
		}
	}
	return 0
}

func (operation Operation) spec() (operationSpec, bool) {
	if operation == 0 || int(operation) >= len(operationSpecs) {
		return operationSpec{}, false
	}
	return operationSpecs[operation], true
}

// Completion contains only canonical catalogue labels, or the invalid zero value.
// It supplies no permission, durable result or proof of successful operation.
type Completion struct {
	operation      Operation
	phase, outcome string
}

// Classify admits the owner's completion categories for this operation. Unknown
// categories return an invalid completion without retaining the original strings.
func (operation Operation) Classify(phase, outcome string) Completion {
	spec, ok := operation.spec()
	if !ok {
		return Completion{}
	}
	phase = canonicalLabel(spec.phases, phase)
	outcome = canonicalLabel(spec.outcomes, outcome)
	if phase == "" || outcome == "" || !successfulCompletionPhase(phase, outcome) {
		return Completion{}
	}
	return Completion{operation, phase, outcome}
}

func canonicalLabel(labels []string, value string) string {
	for _, label := range labels {
		if label == value {
			return label
		}
	}
	return ""
}

// ExportIdentity returns the fixed scope/name selected for existing OTLP export.
// An invalid operation cannot create an SDK tracer or meter.
func (operation Operation) ExportIdentity() (scope, name string, valid bool) {
	spec, valid := operation.spec()
	return spec.scope, spec.name, valid
}

// Labels returns canonical labels only for this completion's original operation.
func (completion Completion) Labels(operation Operation) (phase, outcome string, valid bool) {
	if completion.operation != operation || operation == 0 || completion.phase == "" || completion.outcome == "" {
		return "", "", false
	}
	return completion.phase, completion.outcome, true
}

// Successful labels describe the actual command projection, not any phase in
// the owner. In particular, input refusal or cleanup cannot become readiness.
func successfulCompletionPhase(phase, outcome string) bool {
	switch outcome {
	case "initialized-offline":
		return phase == "execute"
	case "completed":
		return phase == "execute" || phase == "export"
	case "initialized-results":
		return phase == "open-results"
	case "issued-offline", "already-issued", "profile-initialized", "profile-verified", "binding-prepared":
		return phase == "export"
	case "identity-imported":
		return phase == "import"
	default:
		return true
	}
}

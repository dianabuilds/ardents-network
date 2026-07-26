package admission

import (
	"fmt"
	"strings"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
)

// ProcedureRule contains the product-owned parts of protected Application
// admission. Authentication, Delegation, access invocation, audit recording,
// and sealed-call injection remain admission-owned.
type ProcedureRule struct {
	Action        string
	ResourceKind  identityaccess.ResourceKind
	OwnerRequired bool
	Mutating      bool
	Resolve       func(any) (identityaccess.ResourceTarget, error)
	Finalize      identityaccess.ResourceFinalizer
	MapTargetErr  func(error) error
}

// ProcedureContract is the composition-owned declaration of one required
// protected procedure. It is kept separate from its implementation so missing,
// extra, or action/classification-mismatched rules fail closed.
type ProcedureContract struct {
	Procedure     string
	Action        string
	ResourceKind  identityaccess.ResourceKind
	OwnerRequired bool
	Mutating      bool
}

type ProcedureRegistration struct {
	Procedure string
	Rule      ProcedureRule
}

type Registry interface {
	Lookup(procedure string) (ProcedureRule, bool)
	closedProcedureRegistry()
}

type procedureRegistry struct {
	rules map[string]ProcedureRule
}

// NewRegistry closes an exact procedure set at composition time. The returned
// registry exposes lookup only; it has no runtime registration path.
func NewRegistry(contracts []ProcedureContract, registrations []ProcedureRegistration) (Registry, error) {
	if len(contracts) == 0 {
		return nil, fmt.Errorf("protected Application procedure contracts are required")
	}
	if len(contracts) > identitycontract.MaxActions || len(registrations) > identitycontract.MaxActions {
		return nil, fmt.Errorf("protected Application procedure registry exceeds its bound")
	}

	required := make(map[string]ProcedureContract, len(contracts))
	for _, contract := range contracts {
		resource, registeredResource := identitycontract.LookupResourceKind(string(contract.ResourceKind))
		if !validProcedure(contract.Procedure) ||
			!identitycontract.IsRegisteredAction(identitycontract.InterfaceApplication, contract.Action) ||
			!registeredResource || resource.OwnerRequired != contract.OwnerRequired {
			return nil, fmt.Errorf("protected Application procedure contract is invalid")
		}
		if _, duplicate := required[contract.Procedure]; duplicate {
			return nil, fmt.Errorf("protected Application procedure contract is duplicated")
		}
		required[contract.Procedure] = contract
	}

	rules := make(map[string]ProcedureRule, len(registrations))
	for _, registration := range registrations {
		contract, requiredProcedure := required[registration.Procedure]
		if !requiredProcedure {
			return nil, fmt.Errorf("protected Application procedure registration is not declared")
		}
		if _, duplicate := rules[registration.Procedure]; duplicate {
			return nil, fmt.Errorf("protected Application procedure registration is duplicated")
		}
		rule := registration.Rule
		resource, registeredResource := identitycontract.LookupResourceKind(string(rule.ResourceKind))
		if rule.Resolve == nil || rule.Finalize == nil || rule.MapTargetErr == nil ||
			!identitycontract.IsRegisteredAction(identitycontract.InterfaceApplication, rule.Action) ||
			!registeredResource || resource.OwnerRequired != rule.OwnerRequired ||
			rule.Action != contract.Action || rule.ResourceKind != contract.ResourceKind ||
			rule.OwnerRequired != contract.OwnerRequired || rule.Mutating != contract.Mutating {
			return nil, fmt.Errorf("protected Application procedure rule is invalid")
		}
		rules[registration.Procedure] = rule
	}
	if len(rules) != len(required) {
		return nil, fmt.Errorf("protected Application procedure registry is incomplete")
	}
	return &procedureRegistry{rules: rules}, nil
}

func (r *procedureRegistry) Lookup(procedure string) (ProcedureRule, bool) {
	if r == nil {
		return ProcedureRule{}, false
	}
	rule, ok := r.rules[procedure]
	return rule, ok
}

func (*procedureRegistry) closedProcedureRegistry() {}

func validProcedure(procedure string) bool {
	if len(procedure) < 4 || len(procedure) > 512 || procedure[0] != '/' ||
		strings.TrimSpace(procedure) != procedure {
		return false
	}
	for _, value := range procedure {
		if value < 0x21 || value > 0x7e {
			return false
		}
	}
	parts := strings.Split(procedure[1:], "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != "" &&
		!strings.ContainsAny(parts[0], " \t\r\n") &&
		!strings.ContainsAny(parts[1], " \t\r\n")
}

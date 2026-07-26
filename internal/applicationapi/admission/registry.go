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
	Procedure     string
	Action        string
	ResourceKind  identityaccess.ResourceKind
	OwnerRequired bool
	Mutating      bool
	Resolve       func(any) (identityaccess.ResourceTarget, error)
	Finalize      identityaccess.ResourceFinalizer
	MapTargetErr  func(error) error
}

// Registry is the immutable, composition-time view of every protected
// Application procedure. It exposes lookup only and has no runtime
// registration path.
type Registry struct {
	rules map[string]ProcedureRule
}

// NewRegistry closes an exact procedure set at composition time. The returned
// registry exposes lookup only; it has no runtime registration path.
func NewRegistry(requiredProcedures []string, ruleset []ProcedureRule) (*Registry, error) {
	if len(requiredProcedures) == 0 {
		return nil, fmt.Errorf("protected Application procedure contracts are required")
	}
	if len(requiredProcedures) > identitycontract.MaxActions || len(ruleset) > identitycontract.MaxActions {
		return nil, fmt.Errorf("protected Application procedure registry exceeds its bound")
	}

	required := make(map[string]struct{}, len(requiredProcedures))
	for _, procedure := range requiredProcedures {
		if !validProcedure(procedure) {
			return nil, fmt.Errorf("protected Application procedure contract is invalid")
		}
		if _, duplicate := required[procedure]; duplicate {
			return nil, fmt.Errorf("protected Application procedure contract is duplicated")
		}
		required[procedure] = struct{}{}
	}

	rules := make(map[string]ProcedureRule, len(ruleset))
	for _, rule := range ruleset {
		_, requiredProcedure := required[rule.Procedure]
		if !requiredProcedure {
			return nil, fmt.Errorf("protected Application procedure registration is not declared")
		}
		if _, duplicate := rules[rule.Procedure]; duplicate {
			return nil, fmt.Errorf("protected Application procedure registration is duplicated")
		}
		action, registeredAction := identitycontract.LookupApplicationAction(rule.Action)
		resource, registeredResource := identitycontract.LookupResourceKind(string(rule.ResourceKind))
		if rule.Resolve == nil || rule.Finalize == nil || rule.MapTargetErr == nil ||
			!registeredAction || action.Mutating != rule.Mutating ||
			!registeredResource || resource.OwnerRequired != rule.OwnerRequired ||
			!validProcedure(rule.Procedure) {
			return nil, fmt.Errorf("protected Application procedure rule is invalid")
		}
		rules[rule.Procedure] = rule
	}
	if len(rules) != len(required) {
		return nil, fmt.Errorf("protected Application procedure registry is incomplete")
	}
	return &Registry{rules: rules}, nil
}

func (r *Registry) Lookup(procedure string) (ProcedureRule, bool) {
	if r == nil {
		return ProcedureRule{}, false
	}
	rule, ok := r.rules[procedure]
	return rule, ok
}

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

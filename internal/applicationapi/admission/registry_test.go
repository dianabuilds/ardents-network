package admission

import (
	"errors"
	"testing"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"

	"github.com/stretchr/testify/require"
)

const testProcedure = "/ardents.application.v1.TestService/Read"

func testProcedureRule() ProcedureRule {
	return ProcedureRule{
		Procedure:     testProcedure,
		Action:        "application.content.get",
		ResourceKind:  "node",
		OwnerRequired: false,
		Mutating:      false,
		Resolve: func(any) (identityaccess.ResourceTarget, error) {
			return identityaccess.ResourceTarget{Kind: "node"}, nil
		},
		Finalize: func(target identityaccess.ResourceTarget, audience identityaccess.Audience, _, _ string) (identityaccess.ResourceRef, error) {
			return identityaccess.NewResourceRef(audience.Node, identityaccess.ResourceOwner{}, string(target.Kind), target.ID)
		},
		MapTargetErr: func(err error) error { return err },
	}
}

func TestProcedureRegistryClosesTheExactValidRuleSet(t *testing.T) {
	rule := testProcedureRule()
	required := []string{testProcedure}
	rules := []ProcedureRule{rule}

	registry, err := NewRegistry(required, rules)

	require.NoError(t, err)
	registered, ok := registry.Lookup(testProcedure)
	require.True(t, ok)
	_, unknown := registry.Lookup("/ardents.application.v1.TestService/Unknown")
	require.False(t, unknown)

	target, err := registered.Resolve(struct{}{})
	require.NoError(t, err)
	_, err = registered.Finalize(target, identityaccess.Audience{}, "", "")
	require.Error(t, err)
	sentinel := errors.New("target rejected")
	require.ErrorIs(t, registered.MapTargetErr(sentinel), sentinel)

	required[0] = "/mutated"
	rules[0].Action = "application.content.put"
	registered, ok = registry.Lookup(testProcedure)
	require.True(t, ok)
	require.Equal(t, "application.content.get", registered.Action)
}

func TestProcedureRegistryRejectsInvalidComposition(t *testing.T) {
	valid := testProcedureRule()
	mutate := func(change func(*ProcedureRule)) ProcedureRule {
		rule := valid
		change(&rule)
		return rule
	}

	tests := map[string]struct {
		required []string
		rules    []ProcedureRule
	}{
		"no required procedures": {
			rules: []ProcedureRule{valid},
		},
		"duplicate required procedure": {
			required: []string{testProcedure, testProcedure},
			rules:    []ProcedureRule{valid},
		},
		"duplicate rule": {
			required: []string{testProcedure},
			rules:    []ProcedureRule{valid, valid},
		},
		"undeclared rule": {
			required: []string{testProcedure},
			rules: []ProcedureRule{valid, mutate(func(rule *ProcedureRule) {
				rule.Procedure = "/ardents.application.v1.TestService/Extra"
			})},
		},
		"incomplete rule set": {
			required: []string{testProcedure},
		},
		"invalid required procedure": {
			required: []string{"not-canonical"},
			rules:    []ProcedureRule{valid},
		},
		"invalid action": {
			required: []string{testProcedure},
			rules: []ProcedureRule{mutate(func(rule *ProcedureRule) {
				rule.Action = "application.unknown.read"
			})},
		},
		"read action marked mutating": {
			required: []string{testProcedure},
			rules: []ProcedureRule{mutate(func(rule *ProcedureRule) {
				rule.Mutating = true
			})},
		},
		"write action marked non-mutating": {
			required: []string{testProcedure},
			rules: []ProcedureRule{mutate(func(rule *ProcedureRule) {
				rule.Action = "application.content.put"
			})},
		},
		"invalid resource kind": {
			required: []string{testProcedure},
			rules: []ProcedureRule{mutate(func(rule *ProcedureRule) {
				rule.ResourceKind = "unknown"
			})},
		},
		"invalid owner shape": {
			required: []string{testProcedure},
			rules: []ProcedureRule{mutate(func(rule *ProcedureRule) {
				rule.OwnerRequired = true
			})},
		},
		"nil resolver": {
			required: []string{testProcedure},
			rules: []ProcedureRule{mutate(func(rule *ProcedureRule) {
				rule.Resolve = nil
			})},
		},
		"nil finalizer": {
			required: []string{testProcedure},
			rules: []ProcedureRule{mutate(func(rule *ProcedureRule) {
				rule.Finalize = nil
			})},
		},
		"nil target error mapper": {
			required: []string{testProcedure},
			rules: []ProcedureRule{mutate(func(rule *ProcedureRule) {
				rule.MapTargetErr = nil
			})},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			registry, err := NewRegistry(test.required, test.rules)
			require.Error(t, err)
			require.Nil(t, registry)
		})
	}
}

func TestProcedureRegistryRejectsCompositionBeyondActionBound(t *testing.T) {
	required := make([]string, identitycontract.MaxActions+1)
	for index := range required {
		required[index] = testProcedure
	}

	registry, err := NewRegistry(required, []ProcedureRule{testProcedureRule()})

	require.Error(t, err)
	require.Nil(t, registry)
}

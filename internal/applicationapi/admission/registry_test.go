package admission

import (
	"errors"
	"testing"

	identityaccess "ardents/internal/identity/access"

	"github.com/stretchr/testify/require"
)

const testProcedure = "/ardents.application.v1.TestService/Read"

func TestProcedureRegistryClosesTheExactValidRuleSet(t *testing.T) {
	resolved := false
	finalized := false
	mapped := false
	sentinel := errors.New("target rejected")
	rule := ProcedureRule{
		Action:        "application.content.get",
		ResourceKind:  "node",
		OwnerRequired: false,
		Mutating:      false,
		Resolve: func(any) (identityaccess.ResourceTarget, error) {
			resolved = true
			return identityaccess.ResourceTarget{Kind: "node"}, nil
		},
		Finalize: func(target identityaccess.ResourceTarget, audience identityaccess.Audience, _, _ string) (identityaccess.ResourceRef, error) {
			finalized = true
			return identityaccess.NewResourceRef(audience.Node, identityaccess.ResourceOwner{}, string(target.Kind), target.ID)
		},
		MapTargetErr: func(err error) error {
			mapped = true
			return err
		},
	}
	contracts := []ProcedureContract{{
		Procedure: testProcedure, Action: rule.Action, ResourceKind: rule.ResourceKind,
		OwnerRequired: rule.OwnerRequired, Mutating: rule.Mutating,
	}}
	registrations := []ProcedureRegistration{{Procedure: testProcedure, Rule: rule}}

	registry, err := NewRegistry(contracts, registrations)

	require.NoError(t, err)
	registered, ok := registry.Lookup(testProcedure)
	require.True(t, ok)
	_, unknown := registry.Lookup("/ardents.application.v1.TestService/Unknown")
	require.False(t, unknown)

	target, err := registered.Resolve(struct{}{})
	require.NoError(t, err)
	_, err = registered.Finalize(target, identityaccess.Audience{}, "", "")
	require.Error(t, err)
	require.ErrorIs(t, registered.MapTargetErr(sentinel), sentinel)
	require.True(t, resolved)
	require.True(t, finalized)
	require.True(t, mapped)

	contracts[0].Action = "application.content.put"
	registrations[0].Procedure = "/mutated"
	registered, ok = registry.Lookup(testProcedure)
	require.True(t, ok)
	require.Equal(t, "application.content.get", registered.Action)
}

func TestProcedureRegistryRejectsDuplicateUnknownIncompleteAndInvalidRules(t *testing.T) {
	validRule := ProcedureRule{
		Action: "application.content.get", ResourceKind: "node", OwnerRequired: false,
		Resolve: func(any) (identityaccess.ResourceTarget, error) {
			return identityaccess.ResourceTarget{Kind: "node"}, nil
		},
		Finalize: func(identityaccess.ResourceTarget, identityaccess.Audience, string, string) (identityaccess.ResourceRef, error) {
			return identityaccess.ResourceRef{}, nil
		},
		MapTargetErr: func(err error) error { return err },
	}
	validContract := ProcedureContract{
		Procedure: testProcedure, Action: validRule.Action, ResourceKind: validRule.ResourceKind,
		OwnerRequired: validRule.OwnerRequired,
	}
	validRegistration := ProcedureRegistration{Procedure: testProcedure, Rule: validRule}

	tests := map[string]struct {
		contracts     []ProcedureContract
		registrations []ProcedureRegistration
	}{
		"no contracts": {
			registrations: []ProcedureRegistration{validRegistration},
		},
		"duplicate contract": {
			contracts:     []ProcedureContract{validContract, validContract},
			registrations: []ProcedureRegistration{validRegistration},
		},
		"duplicate registration": {
			contracts:     []ProcedureContract{validContract},
			registrations: []ProcedureRegistration{validRegistration, validRegistration},
		},
		"unknown registration": {
			contracts: []ProcedureContract{validContract},
			registrations: []ProcedureRegistration{validRegistration, {
				Procedure: "/ardents.application.v1.TestService/Extra",
				Rule:      validRule,
			}},
		},
		"incomplete registration": {
			contracts: []ProcedureContract{validContract},
		},
		"invalid contract procedure": {
			contracts: []ProcedureContract{{
				Procedure: "not-canonical", Action: validRule.Action,
				ResourceKind: validRule.ResourceKind, OwnerRequired: validRule.OwnerRequired,
			}},
			registrations: []ProcedureRegistration{validRegistration},
		},
		"invalid contract action": {
			contracts: []ProcedureContract{{
				Procedure: testProcedure, Action: "application.unknown.read",
				ResourceKind: validRule.ResourceKind, OwnerRequired: validRule.OwnerRequired,
			}},
			registrations: []ProcedureRegistration{validRegistration},
		},
		"invalid contract resource kind": {
			contracts: []ProcedureContract{{
				Procedure: testProcedure, Action: validRule.Action, ResourceKind: "unknown",
			}},
			registrations: []ProcedureRegistration{validRegistration},
		},
		"invalid contract owner shape": {
			contracts: []ProcedureContract{{
				Procedure: testProcedure, Action: validRule.Action,
				ResourceKind: validRule.ResourceKind, OwnerRequired: true,
			}},
			registrations: []ProcedureRegistration{validRegistration},
		},
		"nil resolver": {
			contracts: []ProcedureContract{validContract},
			registrations: []ProcedureRegistration{{
				Procedure: testProcedure,
				Rule: ProcedureRule{
					Action: validRule.Action, ResourceKind: validRule.ResourceKind,
					Finalize: validRule.Finalize, MapTargetErr: validRule.MapTargetErr,
				},
			}},
		},
		"nil finalizer": {
			contracts: []ProcedureContract{validContract},
			registrations: []ProcedureRegistration{{
				Procedure: testProcedure,
				Rule: ProcedureRule{
					Action: validRule.Action, ResourceKind: validRule.ResourceKind,
					Resolve: validRule.Resolve, MapTargetErr: validRule.MapTargetErr,
				},
			}},
		},
		"nil target error mapper": {
			contracts: []ProcedureContract{validContract},
			registrations: []ProcedureRegistration{{
				Procedure: testProcedure,
				Rule: ProcedureRule{
					Action: validRule.Action, ResourceKind: validRule.ResourceKind,
					Resolve: validRule.Resolve, Finalize: validRule.Finalize,
				},
			}},
		},
		"action mismatch": {
			contracts: []ProcedureContract{validContract},
			registrations: []ProcedureRegistration{{
				Procedure: testProcedure,
				Rule: ProcedureRule{
					Action: "application.content.put", ResourceKind: validRule.ResourceKind,
					Resolve:  validRule.Resolve,
					Finalize: validRule.Finalize, MapTargetErr: validRule.MapTargetErr,
				},
			}},
		},
		"mutation mismatch": {
			contracts: []ProcedureContract{validContract},
			registrations: []ProcedureRegistration{{
				Procedure: testProcedure,
				Rule: ProcedureRule{
					Action: validRule.Action, ResourceKind: validRule.ResourceKind,
					Mutating: true, Resolve: validRule.Resolve,
					Finalize: validRule.Finalize, MapTargetErr: validRule.MapTargetErr,
				},
			}},
		},
		"resource mismatch": {
			contracts: []ProcedureContract{validContract},
			registrations: []ProcedureRegistration{{
				Procedure: testProcedure,
				Rule: ProcedureRule{
					Action: validRule.Action, ResourceKind: "content-owner", OwnerRequired: true,
					Resolve: validRule.Resolve, Finalize: validRule.Finalize, MapTargetErr: validRule.MapTargetErr,
				},
			}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			registry, err := NewRegistry(test.contracts, test.registrations)
			require.Error(t, err)
			require.Nil(t, registry)
		})
	}
}

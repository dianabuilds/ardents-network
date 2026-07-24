package catalog_test

import (
	"testing"

	"ardents/internal/cli/catalog"
	localauth "ardents/internal/localapi/auth"
	localidentity "ardents/internal/localapi/identity"

	"github.com/stretchr/testify/require"
)

func TestProtectedCommandsMatchServerOwnedOperatorProcedureMetadata(t *testing.T) {
	resolver := func(procedure string) (catalog.ProcedureRule, bool) {
		if rule, ok := localauth.RuleForProcedure(procedure); ok {
			return catalog.ProcedureRule{
				Access:       catalog.AccessProtected,
				Action:       rule.Action,
				ResourceKind: rule.ResourceKind,
				Mutating:     rule.Mutating,
			}, true
		}
		rule, ok := localidentity.RuleForProcedure(procedure)
		if !ok {
			return catalog.ProcedureRule{}, false
		}
		var access catalog.AccessClass
		switch rule.Class {
		case localidentity.RuleClassPublicBounded:
			access = catalog.AccessPublicBounded
		case localidentity.RuleClassSessionLifecycle:
			access = catalog.AccessSessionLifecycle
		case localidentity.RuleClassProtected:
			access = catalog.AccessProtected
		default:
			return catalog.ProcedureRule{}, false
		}
		return catalog.ProcedureRule{
			Access:       access,
			Action:       rule.Action,
			ResourceKind: rule.ResourceKind,
			Mutating:     rule.Mutating,
		}, true
	}

	require.NoError(t, catalog.Validate(catalog.Commands(), resolver))
}

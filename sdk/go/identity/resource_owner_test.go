package identity

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourceOwnerIsClosedToPrincipals(t *testing.T) {
	principal := "p1_3tblsvfqcspd2hkaujldchz7vbzgb725kpkbu33lnvjnn7dtzjma"
	owner, err := PrincipalOwner(principal)
	require.NoError(t, err)
	require.Equal(t, principal, owner.String())
	require.False(t, owner.IsNone())

	for _, invalid := range []string{"node", "workload_1", "service_1", "p_3tblsvfqcspd2hkaujldchz7vbzgb725kpkbu33lnvjnn7dtzjma", ""} {
		_, err := PrincipalOwner(invalid)
		require.ErrorIs(t, err, ErrInvalidResourceOwner)
	}

	none, err := ParseResourceOwner("")
	require.NoError(t, err)
	require.True(t, none.IsNone())
}

func TestSecurityOwnerFieldsUseResourceOwner(t *testing.T) {
	ownerType := reflect.TypeOf(ResourceOwner{})
	require.Equal(t, ownerType, reflect.TypeOf(ResourceRef{}).Field(1).Type)
	require.Equal(t, ownerType, reflect.TypeOf(ResourceScope{}).Field(1).Type)
}

func mustSDKResourceOwner(t *testing.T, value string) ResourceOwner {
	t.Helper()
	owner, err := PrincipalOwner(value)
	require.NoError(t, err)
	return owner
}

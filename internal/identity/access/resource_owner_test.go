package access

import (
	"reflect"
	"testing"

	identityprincipal "ardents/internal/identity/principal"

	"github.com/stretchr/testify/require"
)

func TestResourceOwnerAcceptsOnlyCanonicalPrincipalOrNone(t *testing.T) {
	principal := fakePrincipal(91)
	owner, err := ParseResourceOwner(principal)
	require.NoError(t, err)
	require.Equal(t, principal, owner.String())
	require.False(t, owner.IsNone())
	parsed, ok := owner.Principal()
	require.True(t, ok)
	require.Equal(t, principal, parsed.String())

	none, err := ParseResourceOwner("")
	require.NoError(t, err)
	require.True(t, none.IsNone())
	_, ok = none.Principal()
	require.False(t, ok)

	for _, invalid := range []string{"node", "workload-1", "service-1", " p1_invalid", "p1_invalid"} {
		_, err := ParseResourceOwner(invalid)
		require.ErrorIs(t, err, ErrInvalidArgument, invalid)
	}
	_, err = PrincipalOwner(identityprincipal.ID{})
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestResourceOwnerFieldsAreClosedTypedValues(t *testing.T) {
	require.Equal(t, reflect.TypeFor[ResourceOwner](), reflect.TypeFor[ResourceRef]().Field(1).Type)
	require.Equal(t, reflect.TypeFor[ResourceOwner](), reflect.TypeFor[ResourceScope]().Field(1).Type)
}

func mustResourceOwner(t testing.TB, value string) ResourceOwner {
	t.Helper()
	owner, err := ParseResourceOwner(value)
	require.NoError(t, err)
	return owner
}

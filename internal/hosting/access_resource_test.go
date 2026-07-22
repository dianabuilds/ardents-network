package hosting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceAccessResourceIDRequiresCanonicalServiceID(t *testing.T) {
	id, err := ServiceAccessResourceID("svc.echo")
	require.NoError(t, err)
	require.Equal(t, "svc.echo", id)

	for _, value := range []string{"", " svc.echo", "svc.echo\n", strings.Repeat("s", 513)} {
		_, err := ServiceAccessResourceID(value)
		require.Error(t, err)
	}
}

package workload

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccessResourceIDRequiresCanonicalWorkloadID(t *testing.T) {
	id, err := AccessResourceID("work.echo")
	require.NoError(t, err)
	require.Equal(t, "work.echo", id)

	for _, value := range []string{"", " work.echo", "work.echo\n", strings.Repeat("w", 513)} {
		_, err := AccessResourceID(value)
		require.Error(t, err)
	}
}

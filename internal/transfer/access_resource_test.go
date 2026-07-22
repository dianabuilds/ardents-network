package transfer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccessResourceIDRequiresCanonicalTransferID(t *testing.T) {
	id, err := AccessResourceID("transfer-1")
	require.NoError(t, err)
	require.Equal(t, "transfer-1", id)
	for _, value := range []string{"", " transfer-1", "transfer-1\n", strings.Repeat("t", 513)} {
		_, err := AccessResourceID(value)
		require.Error(t, err)
	}
}

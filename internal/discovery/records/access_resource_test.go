package records

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccessResourceIDIsCanonicalTupleNotDelimiterJoin(t *testing.T) {
	first, err := AccessResourceID("node", "p1_subject")
	require.NoError(t, err)
	again, err := AccessResourceID("node", "p1_subject")
	require.NoError(t, err)
	otherKind, err := AccessResourceID("service", "p1_subject")
	require.NoError(t, err)
	otherSubject, err := AccessResourceID("node", "p1_other")
	require.NoError(t, err)
	require.Equal(t, first, again)
	require.NotEqual(t, first, otherKind)
	require.NotEqual(t, first, otherSubject)
	require.NotContains(t, first, "node")

	for _, values := range [][2]string{{"", "subject"}, {"node", ""}, {" node", "subject"}, {"node", "subject\n"}, {strings.Repeat("k", 513), "subject"}} {
		_, err := AccessResourceID(values[0], values[1])
		require.Error(t, err)
	}
}

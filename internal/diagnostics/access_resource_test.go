package diagnostics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubjectAccessResourceIDUsesClosedScopeAndExactTuple(t *testing.T) {
	service, err := SubjectAccessResourceID("service", "svc.echo")
	require.NoError(t, err)
	repeat, err := SubjectAccessResourceID("service", "svc.echo")
	require.NoError(t, err)
	transport, err := SubjectAccessResourceID("transport", "svc.echo")
	require.NoError(t, err)
	require.Equal(t, service, repeat)
	require.NotEqual(t, service, transport)
	_, err = SubjectAccessResourceID("unknown", "svc.echo")
	require.Error(t, err)
	_, err = SubjectAccessResourceID("service", "svc echo")
	require.Error(t, err)
}

func TestValidateRecentEventsPageRejectsNoncanonicalBounds(t *testing.T) {
	for _, valid := range []struct {
		limit  int32
		cursor string
	}{{0, ""}, {10, "1"}, {MaxRecentEventsPage, "9223372036854775807"}} {
		require.NoError(t, ValidateRecentEventsPage(valid.limit, valid.cursor))
	}
	for _, invalid := range []struct {
		limit  int32
		cursor string
	}{{-1, ""}, {MaxRecentEventsPage + 1, ""}, {10, "0"}, {10, "01"}, {10, "-1"}, {10, "x"}} {
		require.Error(t, ValidateRecentEventsPage(invalid.limit, invalid.cursor))
	}
}

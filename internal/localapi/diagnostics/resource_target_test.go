package diagnostics

import (
	"testing"

	identityaccess "ardents/internal/identity/access"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"github.com/stretchr/testify/require"
)

func TestCanonicalizeResourceOwnsDiagnosticsProtocolMapping(t *testing.T) {
	target, err := CanonicalizeResource(ardentsv1connect.DiagnosticsServiceExplainFailureProcedure, &protocol.ExplainFailureRequest{Scope: "service", ResourceId: "svc.echo"}, identityaccess.ResourceKind("diagnostic-subject"))
	require.NoError(t, err)
	require.NotEmpty(t, target.ID)
	target, err = CanonicalizeResource(ardentsv1connect.DiagnosticsServiceListRecentEventsProcedure, &protocol.ListRecentEventsRequest{Limit: 10, Cursor: "1"}, identityaccess.ResourceKind("event-collection"))
	require.NoError(t, err)
	require.Empty(t, target.ID)

	for _, request := range []*protocol.ListRecentEventsRequest{{Limit: -1}, {Cursor: "0"}, {Cursor: "01"}} {
		_, err = CanonicalizeResource(ardentsv1connect.DiagnosticsServiceListRecentEventsProcedure, request, "event-collection")
		require.ErrorIs(t, err, ErrInvalidResourceTarget)
	}
	_, err = CanonicalizeResource(ardentsv1connect.DiagnosticsServiceExplainFailureProcedure, &protocol.ExplainFailureRequest{Scope: "unknown"}, "diagnostic-subject")
	require.ErrorIs(t, err, ErrInvalidResourceTarget)
}

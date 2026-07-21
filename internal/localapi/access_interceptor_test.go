package localapi

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	diagapi "ardents/internal/diagnostics"
	localauth "ardents/internal/localapi/auth"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

type auditCapture struct {
	commands []diagapi.RecordEventCommand
}

func (c *auditCapture) RecordEventCommand(command diagapi.RecordEventCommand) diagapi.EventEnvelope {
	c.commands = append(c.commands, command)
	return diagapi.EventEnvelope{}
}

func TestAccessInterceptorRequiresExactSiblingAction(t *testing.T) {
	audit := &auditCapture{}
	interceptor := newAccessInterceptor(localauth.Config{
		Token: "secret-token", SubjectID: "operator-1", Capabilities: []string{"workload.start"},
	}, audit)
	header := bearerHeader("secret-token")

	require.NoError(t, interceptor.authorize(ardentsv1connect.WorkloadServiceStartWorkloadProcedure, header))
	err := interceptor.authorize(ardentsv1connect.WorkloadServiceStopWorkloadProcedure, header)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	require.Equal(t, "denied", audit.commands[0].Payload["outcome"])
	require.NotContains(t, fmt.Sprint(audit.commands), "secret-token")
}

func TestAccessInterceptorEnforcesExpiryBindingsAndScopeNarrowing(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	auth := localauth.Config{
		Token: "token", SubjectID: "operator", Capabilities: []string{"node.status", "node.start"},
		ExpiresAt: now.Add(time.Minute), TargetNode: "node-a", TargetPrincipal: "p_a", Now: func() time.Time { return now },
	}
	header := bearerHeader("token")
	header.Set(localauth.HeaderExpectedNode, "node-a")
	header.Set(localauth.HeaderExpectedPrincipal, "p_a")
	header.Set(localauth.HeaderScopes, "node.status")
	interceptor := newAccessInterceptor(auth, &auditCapture{})

	require.NoError(t, interceptor.authorize(ardentsv1connect.NodeServiceGetNodeStatusProcedure, header))
	require.Equal(t, connect.CodePermissionDenied,
		connect.CodeOf(interceptor.authorize(ardentsv1connect.NodeServiceStartNodeProcedure, header)))

	wrongNode := header.Clone()
	wrongNode.Set(localauth.HeaderExpectedNode, "node-b")
	require.Equal(t, connect.CodePermissionDenied,
		connect.CodeOf(interceptor.authorize(ardentsv1connect.NodeServiceGetNodeStatusProcedure, wrongNode)))

	wrongPrincipal := header.Clone()
	wrongPrincipal.Set(localauth.HeaderExpectedPrincipal, "p_b")
	require.Equal(t, connect.CodePermissionDenied,
		connect.CodeOf(interceptor.authorize(ardentsv1connect.NodeServiceGetNodeStatusProcedure, wrongPrincipal)))

	auth.ExpiresAt = now
	require.Equal(t, connect.CodeUnauthenticated,
		connect.CodeOf(newAccessInterceptor(auth, nil).authorize(ardentsv1connect.NodeServiceGetNodeStatusProcedure, header)))
}

func bearerHeader(token string) http.Header {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	return header
}

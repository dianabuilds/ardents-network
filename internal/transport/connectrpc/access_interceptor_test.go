package connectrpc

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	diagapi "ardents/internal/diagnostics/api"
	"ardents/proto/ardents/v1/ardentsv1connect"

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
	interceptor := newAccessInterceptor(AuthConfig{
		Token: "secret-token", SubjectID: "operator-1", Capabilities: []string{"workload.start"},
	}, audit)
	header := bearerHeader("secret-token")

	require.NoError(t, interceptor.authorize(ardentsv1connect.ArdentsServiceStartWorkloadProcedure, header))
	err := interceptor.authorize(ardentsv1connect.ArdentsServiceStopWorkloadProcedure, header)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	require.Equal(t, "denied", audit.commands[0].Payload["outcome"])
	require.NotContains(t, fmt.Sprint(audit.commands), "secret-token")
}

func TestAccessInterceptorEnforcesExpiryBindingsAndScopeNarrowing(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	auth := AuthConfig{
		Token: "token", SubjectID: "operator", Capabilities: []string{"node.status", "node.start"},
		ExpiresAt: now.Add(time.Minute), TargetNode: "node-a", TargetPrincipal: "p_a", Now: func() time.Time { return now },
	}
	header := bearerHeader("token")
	header.Set(headerExpectedNode, "node-a")
	header.Set(headerExpectedPrincipal, "p_a")
	header.Set(headerScopes, "node.status")
	interceptor := newAccessInterceptor(auth, &auditCapture{})

	require.NoError(t, interceptor.authorize(ardentsv1connect.ArdentsServiceGetNodeStatusProcedure, header))
	require.Equal(t, connect.CodePermissionDenied,
		connect.CodeOf(interceptor.authorize(ardentsv1connect.ArdentsServiceStartNodeProcedure, header)))

	wrongNode := header.Clone()
	wrongNode.Set(headerExpectedNode, "node-b")
	require.Equal(t, connect.CodePermissionDenied,
		connect.CodeOf(interceptor.authorize(ardentsv1connect.ArdentsServiceGetNodeStatusProcedure, wrongNode)))

	wrongPrincipal := header.Clone()
	wrongPrincipal.Set(headerExpectedPrincipal, "p_b")
	require.Equal(t, connect.CodePermissionDenied,
		connect.CodeOf(interceptor.authorize(ardentsv1connect.ArdentsServiceGetNodeStatusProcedure, wrongPrincipal)))

	auth.ExpiresAt = now
	require.Equal(t, connect.CodeUnauthenticated,
		connect.CodeOf(newAccessInterceptor(auth, nil).authorize(ardentsv1connect.ArdentsServiceGetNodeStatusProcedure, header)))
}

func bearerHeader(token string) http.Header {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	return header
}

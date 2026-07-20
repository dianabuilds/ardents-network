//go:build integration

package localcontrolsurface

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	diagapi "ardents/internal/diagnostics/api"
	runtimeinfra "ardents/internal/runtime/process"
	rpcadapter "ardents/internal/transport/connectrpc"
	ardentsv1 "ardents/proto/ardents/v1"
	"ardents/proto/ardents/v1/ardentsv1connect"
	"ardents/tests/testkit"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestOperatorAccessLeastPrivilegeAndAudit(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "local-control-surface", ScenarioID: "OAI-001",
		Suite: "integration", Tags: []string{"integration", "local-control-surface", "operator-access"},
		Speed: "fast", Environment: "local",
	})
	runtime := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "operator-access", Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	}).Runtime
	target := runtime.GetNodeRuntime()
	auth := rpcadapter.AuthConfig{
		Token: "operator-secret", SubjectID: "status-reader", Capabilities: []string{"node.status", "node.start"},
		TargetNode: target.Node.Name, TargetPrincipal: target.Identity.Principal,
	}
	client := newOperatorAccessClient(t, runtime, auth)

	allowed := operatorRequest(&ardentsv1.GetNodeStatusRequest{}, auth.Token, target.Node.Name, target.Identity.Principal, "node.status")
	_, err := client.GetNodeStatus(context.Background(), allowed)
	require.NoError(t, err)
	command := operatorRequest(&ardentsv1.StartNodeRequest{}, auth.Token, target.Node.Name, target.Identity.Principal, "node.start")
	_, err = client.StartNode(context.Background(), command)
	require.NoError(t, err)

	sibling := operatorRequest(&ardentsv1.GetNodeCapabilitiesRequest{}, auth.Token, target.Node.Name, target.Identity.Principal, "node.status")
	_, err = client.GetNodeCapabilities(context.Background(), sibling)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	wrongNode := operatorRequest(&ardentsv1.GetNodeStatusRequest{}, auth.Token, "wrong-node", target.Identity.Principal, "node.status")
	_, err = client.GetNodeStatus(context.Background(), wrongNode)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	wrongPrincipal := operatorRequest(&ardentsv1.GetNodeStatusRequest{}, auth.Token, target.Node.Name, "p_wrong", "node.status")
	_, err = client.GetNodeStatus(context.Background(), wrongPrincipal)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	events, _ := runtime.ListRecentEvents(100, "")
	assertSafeOperatorAudit(t, events, auth.Token)
}

func TestOperatorAccessRejectsExpiredCredential(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "local-control-surface", ScenarioID: "OAI-001",
		Suite: "integration", Tags: []string{"integration", "local-control-surface", "operator-access"},
		Speed: "fast", Environment: "local",
	})
	runtime := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "operator-expired", Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	}).Runtime
	target := runtime.GetNodeRuntime()
	auth := rpcadapter.AuthConfig{
		Token: "expired-secret", SubjectID: "stale-reader", Capabilities: []string{"node.status"},
		ExpiresAt: time.Now().Add(-time.Minute), TargetNode: target.Node.Name, TargetPrincipal: target.Identity.Principal,
	}
	client := newOperatorAccessClient(t, runtime, auth)
	request := operatorRequest(&ardentsv1.GetNodeStatusRequest{}, auth.Token, target.Node.Name, target.Identity.Principal, "node.status")

	_, err := client.GetNodeStatus(context.Background(), request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func newOperatorAccessClient(t *testing.T, runtime runtimeinfra.NodeRuntime, auth rpcadapter.AuthConfig) ardentsv1connect.ArdentsServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	path, handler, err := rpcadapter.NewHandler(testkit.ConnectDependencies(runtime), auth)
	require.NoError(t, err)
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return ardentsv1connect.NewArdentsServiceClient(server.Client(), server.URL, connect.WithGRPC())
}

func operatorRequest[T any](message *T, token, node, principal, scopes string) *connect.Request[T] {
	request := connect.NewRequest(message)
	request.Header().Set("Authorization", "Bearer "+token)
	request.Header().Set("Ardents-Expected-Node", node)
	request.Header().Set("Ardents-Expected-Principal", principal)
	request.Header().Set("Ardents-Scopes", scopes)
	return request
}

func assertSafeOperatorAudit(t *testing.T, events []diagapi.EventEnvelope, token string) {
	t.Helper()
	data, err := json.Marshal(events)
	require.NoError(t, err)
	require.NotContains(t, string(data), token)
	require.Contains(t, string(data), "operator_action_allowed")
	require.Contains(t, string(data), "operator_action_denied")
}

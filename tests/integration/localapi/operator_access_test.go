//go:build integration

package localapi_test

import (
	"context"
	"encoding/json"
	"testing"

	runtimeinfra "ardents/internal/daemon"
	diagapi "ardents/internal/diagnostics"
	identityaccess "ardents/internal/identity/access"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/tests/testkit"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestOperatorPrincipalSessionEnforcesLeastPrivilegeAndSafeAudit(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "local-control-surface", ScenarioID: "OAI-001",
		Suite: "integration", Tags: []string{"integration", "local-control-surface", "operator-access"},
		Speed: "fast", Environment: "local",
	})
	runtime := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "operator-access", Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	}).Runtime
	client := testkit.NewArdentsClientWithActions(t, runtime, []identityaccess.Action{"node.start", "node.status"})

	_, err := client.GetNodeStatus(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNodeStatusRequest{}))
	require.NoError(t, err)
	_, err = client.StartNode(context.Background(), testkit.AuthorizedRequest(&ardentsv1.StartNodeRequest{}))
	require.NoError(t, err)

	_, err = client.GetNodeFeatures(context.Background(), testkit.AuthorizedRequest(&ardentsv1.GetNodeFeaturesRequest{}))
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	events, _ := testkit.Diagnostics(runtime).ListRecentEvents(100, "")
	assertSafeOperatorAudit(t, events, "must-not-appear")
}

func TestOperatorPrincipalInterfaceRejectsBearerAndMalformedSessionSchemes(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "local-control-surface", ScenarioID: "OAI-002",
		Suite: "integration", Tags: []string{"integration", "local-control-surface", "operator-access", "negative"},
		Speed: "fast", Environment: "local",
	})
	runtime := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "operator-scheme-rejection", Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	}).Runtime
	client := testkit.NewArdentsClientWithActions(t, runtime, []identityaccess.Action{"node.status"})

	for name, authorization := range map[string]string{
		"bearer":            "Bearer must-not-appear",
		"opaque":            "must-not-appear",
		"malformed session": "ArdentsOperatorSession must-not-appear",
	} {
		t.Run(name, func(t *testing.T) {
			request := connect.NewRequest(&ardentsv1.GetNodeStatusRequest{})
			request.Header().Set("Authorization", authorization)
			_, err := client.GetNodeStatus(context.Background(), request)
			require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
		})
	}

	events, _ := testkit.Diagnostics(runtime).ListRecentEvents(100, "")
	assertSafeOperatorAudit(t, events, "must-not-appear")
}

func assertSafeOperatorAudit(t *testing.T, events []diagapi.EventEnvelope, protectedValue string) {
	t.Helper()
	data, err := json.Marshal(events)
	require.NoError(t, err)
	require.NotContains(t, string(data), protectedValue)
}

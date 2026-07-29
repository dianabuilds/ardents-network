package topology

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configurationcmd "ardents/internal/cli/configuration"
	"ardents/internal/deployment"
	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

type fakeFactory struct {
	opened []string
	closed []string
}

func (factory *fakeFactory) Open(cfg configurationcmd.Config) (openedClient, error) {
	factory.opened = append(factory.opened, cfg.ExpectedNode)
	calls := fakeProtectedCalls{node: cfg.ExpectedNode, principal: cfg.ExpectedPrincipal, image: imageForNode(cfg.ExpectedNode)}
	return openedClient{calls: calls, close: func() error {
		factory.closed = append(factory.closed, cfg.ExpectedNode)
		return nil
	}}, nil
}

type fakeProtectedCalls struct {
	node      string
	principal string
	image     string
}

func (calls fakeProtectedCalls) GetNodeRuntime(context.Context, *connect.Request[protocol.GetNodeRuntimeRequest]) (*connect.Response[protocol.NodeRuntimeResponse], error) {
	return connect.NewResponse(&protocol.NodeRuntimeResponse{Runtime: &protocol.NodeRuntimeSnapshot{
		Node: &protocol.NodeSnapshot{Name: calls.node}, Identity: &protocol.IdentitySnapshot{Principal: calls.principal},
		Readiness: &protocol.ReadinessSnapshot{Ready: true},
	}}), nil
}

func (calls fakeProtectedCalls) GetNetworkStatus(context.Context, *connect.Request[protocol.GetNetworkStatusRequest]) (*connect.Response[protocol.NetworkStatusResponse], error) {
	network := &protocol.NetworkStatusSnapshot{
		Joined: true, StoreEnabled: true, StoreState: "ready",
		ReachabilityMode: "public_direct", ReachabilityState: "public", Reachable: true,
	}
	if calls.node == "node-c" {
		network.ReachabilityMode = "outbound_only"
		network.ReachabilityState = "outbound_only"
		network.Reachable = false
		network.StoreEnabled = false
		network.StoreState = "disabled"
	}
	return connect.NewResponse(&protocol.NetworkStatusResponse{Network: network}), nil
}

func (calls fakeProtectedCalls) GetNodeFeatures(context.Context, *connect.Request[protocol.GetNodeFeaturesRequest]) (*connect.Response[protocol.NodeFeaturesResponse], error) {
	return connect.NewResponse(&protocol.NodeFeaturesResponse{Features: &protocol.NodeFeaturesSnapshot{ImageReference: calls.image}}), nil
}

func TestCommandUsesThreeSeparateManifestBoundContextsAndRedactsJSON(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "deployment", "testdata", "public-direct.json")
	contextFile := writeTopologyContexts(t)
	factory := &fakeFactory{}
	var stdout, stderr bytes.Buffer
	command := Command{
		Base: configurationcmd.Config{ContextFile: contextFile, Output: "json", Timeout: time.Second},
		Out:  &stdout, Err: &stderr, Factory: factory,
	}
	code := command.Run(context.Background(), []string{"status", "--manifest", manifestPath})
	require.Zero(t, code, stderr.String())
	require.Equal(t, []string{"node-a", "node-b", "node-c"}, factory.opened)
	require.ElementsMatch(t, factory.opened, factory.closed)
	var result map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &result))
	require.Equal(t, "ready", result["outcome"])
	for _, secret := range []string{"operator@", "host-pin-", "operator-primary", "p1_", "registry.example", "sha256:"} {
		require.NotContains(t, stdout.String()+stderr.String(), secret)
	}
}

func TestProbeRejectsManifestToContextPinMismatchBeforeOpeningClient(t *testing.T) {
	contextFile := writeTopologyContexts(t)
	factory := &fakeFactory{}
	probe := Probe{Base: configurationcmd.Config{ContextFile: contextFile, Timeout: time.Second}, Factory: factory}
	_, err := probe.Observe(context.Background(), topologyTarget("different-pin"))
	require.ErrorContains(t, err, "host_key_mismatch")
	require.Empty(t, factory.opened)
}

func TestCommandRejectsNonRegularManifestWithoutPathLeak(t *testing.T) {
	path := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := (Command{
		Base: configurationcmd.Config{Output: "json"},
		Out:  &stdout, Err: &stderr,
	}).Run(context.Background(), []string{"status", "--manifest", path})
	require.Equal(t, 2, code)
	require.Empty(t, stdout.String())
	require.NotContains(t, stderr.String(), path)
}

func writeTopologyContexts(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "deployment", "testdata", "public-direct.json"))
	require.NoError(t, err)
	var manifest struct {
		OperatorSignerAlias string `json:"operator_signer_alias"`
		Nodes               []struct {
			Slot string `json:"slot"`
			Host struct {
				SSHAlias      string `json:"ssh_alias"`
				HostKeyPinRef string `json:"host_key_pin_ref"`
			} `json:"host"`
			Principal string `json:"expected_node_principal"`
		} `json:"nodes"`
	}
	require.NoError(t, json.Unmarshal(raw, &manifest))
	contexts := configurationcmd.ContextFile{Contexts: map[string]configurationcmd.StoredContext{}}
	for _, node := range manifest.Nodes {
		contexts.Contexts[node.Host.SSHAlias] = configurationcmd.StoredContext{
			Addr: "unix:///run/ardents/operator.sock", SSH: "operator@" + node.Slot,
			SSHKnownHosts: "pins/" + node.Slot, SSHOperatorSocket: "/run/ardents/operator.sock",
			SignerFile: "signers/operator.json", SignerAlias: manifest.OperatorSignerAlias,
			HostKeyPinRef: node.Host.HostKeyPinRef, ExpectedNode: node.Slot, ExpectedPrincipal: node.Principal,
		}
	}
	encoded, err := json.Marshal(contexts)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "contexts.json")
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	return path
}

func topologyTarget(pin string) deployment.NodeStatusTarget {
	return deployment.NodeStatusTarget{
		Slot: "node-a", SSHAlias: "ssh-node-a", HostKeyPinRef: pin,
		OperatorSignerAlias:   "operator-primary",
		ExpectedNodePrincipal: "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
	}
}

func imageForNode(node string) string {
	digest := map[string]string{"node-a": "a", "node-b": "b", "node-c": "c"}[node]
	return "registry.example/ardents/node@sha256:" + strings.Repeat(digest, 64)
}

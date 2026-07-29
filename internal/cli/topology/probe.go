// Package topology owns the bounded multi-host status command and its
// protected-call adapters. It does not own topology admission, Node truth, or
// Operator authentication policy.
package topology

import (
	"context"
	"errors"

	"ardents/internal/cli/client"
	configurationcmd "ardents/internal/cli/configuration"
	identitycmd "ardents/internal/cli/identity"
	"ardents/internal/deployment"
	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
)

type protectedCalls interface {
	GetNodeRuntime(context.Context, *connect.Request[protocol.GetNodeRuntimeRequest]) (*connect.Response[protocol.NodeRuntimeResponse], error)
	GetNetworkStatus(context.Context, *connect.Request[protocol.GetNetworkStatusRequest]) (*connect.Response[protocol.NetworkStatusResponse], error)
	GetNodeFeatures(context.Context, *connect.Request[protocol.GetNodeFeaturesRequest]) (*connect.Response[protocol.NodeFeaturesResponse], error)
}

type openedClient struct {
	calls protectedCalls
	close func() error
}

type clientFactory interface {
	Open(configurationcmd.Config) (openedClient, error)
}

type operatorClientFactory struct{}

func (operatorClientFactory) Open(cfg configurationcmd.Config) (openedClient, error) {
	signer, err := identitycmd.OpenDeviceFileSigner(cfg.SignerFile)
	if err != nil {
		return openedClient{}, deployment.ProbeError(deployment.ProbeLocalSignerUnavailable)
	}
	opened, err := client.New(client.Config{
		BaseURL: cfg.Addr, SSH: cfg.SSH, SSHPort: cfg.SSHPort,
		SSHIdentity: cfg.SSHIdentity, SSHKnownHosts: cfg.SSHKnownHosts,
		SSHOperatorSocket: cfg.SSHOperatorSocket, Timeout: cfg.Timeout,
		ExpectedNode: cfg.ExpectedNode, ExpectedPrincipal: cfg.ExpectedPrincipal,
		Scopes: cfg.ScopeHints, Signer: signer,
	})
	if err != nil {
		return openedClient{}, classifyProbeError(err)
	}
	return openedClient{calls: opened.Service(), close: opened.Close}, nil
}

// Probe opens a separate pin-validated SSH/session boundary for every Node.
type Probe struct {
	Base    configurationcmd.Config
	Factory clientFactory
}

func (probe Probe) Observe(ctx context.Context, target deployment.NodeStatusTarget) (deployment.NodeObservation, error) {
	resolved, err := probe.Base.ResolveTopologyContext(target.SSHAlias)
	if err != nil {
		return deployment.NodeObservation{}, deployment.ProbeError(deployment.ProbeTunnelFailure)
	}
	if resolved.ContextName != target.SSHAlias ||
		resolved.HostKeyPinRef != target.HostKeyPinRef {
		return deployment.NodeObservation{}, deployment.ProbeError(deployment.ProbeHostKeyMismatch)
	}
	if resolved.SignerAlias != target.OperatorSignerAlias {
		return deployment.NodeObservation{}, deployment.ProbeError(deployment.ProbeLocalSignerUnavailable)
	}
	if resolved.ExpectedNode != target.Slot ||
		resolved.ExpectedPrincipal != target.ExpectedNodePrincipal {
		return deployment.NodeObservation{}, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	factory := probe.Factory
	if factory == nil {
		factory = operatorClientFactory{}
	}
	opened, err := factory.Open(resolved)
	if err != nil {
		return deployment.NodeObservation{}, classifyProbeError(err)
	}
	if opened.calls == nil || opened.close == nil {
		return deployment.NodeObservation{}, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	defer opened.close()

	runtimeResponse, err := opened.calls.GetNodeRuntime(ctx, client.Request(&protocol.GetNodeRuntimeRequest{}))
	if err != nil {
		return deployment.NodeObservation{}, classifyProbeError(err)
	}
	networkResponse, err := opened.calls.GetNetworkStatus(ctx, client.Request(&protocol.GetNetworkStatusRequest{}))
	if err != nil {
		return deployment.NodeObservation{}, classifyProbeError(err)
	}
	featureResponse, err := opened.calls.GetNodeFeatures(ctx, client.Request(&protocol.GetNodeFeaturesRequest{}))
	if err != nil {
		return deployment.NodeObservation{}, classifyProbeError(err)
	}
	if runtimeResponse == nil || runtimeResponse.Msg == nil ||
		networkResponse == nil || networkResponse.Msg == nil ||
		featureResponse == nil || featureResponse.Msg == nil {
		return deployment.NodeObservation{}, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	runtime := runtimeResponse.Msg.GetRuntime()
	network := networkResponse.Msg.GetNetwork()
	features := featureResponse.Msg.GetFeatures()
	if runtime == nil || runtime.GetNode() == nil || runtime.GetIdentity() == nil ||
		runtime.GetReadiness() == nil || network == nil || features == nil {
		return deployment.NodeObservation{}, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	return deployment.NodeObservation{
		NodeName: runtime.GetNode().GetName(), NodePrincipal: runtime.GetIdentity().GetPrincipal(),
		RuntimeReady: runtime.GetReadiness().GetReady(), RuntimeReason: runtime.GetReadiness().GetReason(),
		Joined: network.GetJoined(), ReachabilityMode: network.GetReachabilityMode(),
		ReachabilityState: network.GetReachabilityState(), Reachable: network.GetReachable(),
		StoreEnabled: network.GetStoreEnabled(), StoreState: network.GetStoreState(),
		ImageReference: features.GetImageReference(),
	}, nil
}

func classifyProbeError(err error) error {
	if err == nil {
		return nil
	}
	var probeError deployment.ProbeError
	if errors.As(err, &probeError) {
		return probeError
	}
	switch {
	case errors.Is(err, client.ErrSSHHostKeyMismatch):
		return deployment.ProbeError(deployment.ProbeHostKeyMismatch)
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return deployment.ProbeError(deployment.ProbeTunnelTimeout)
	}
	switch connect.CodeOf(err) {
	case connect.CodeUnauthenticated:
		return deployment.ProbeError(deployment.ProbeRemoteUnauthenticated)
	case connect.CodePermissionDenied:
		return deployment.ProbeError(deployment.ProbeRemoteDenied)
	case connect.CodeUnavailable, connect.CodeDeadlineExceeded:
		return deployment.ProbeError(deployment.ProbeNodeUnavailable)
	default:
		return deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
}

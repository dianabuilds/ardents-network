package docker

import (
	"ardents/internal/ingressproxy"
	"ardents/internal/workload/execution"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateIngressProxyLabels(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateIngressProxyLabels(map[string]string{
		ingressproxy.ProtocolLabel: ingressproxy.ProtocolVersion(),
	}))
	require.ErrorContains(t, validateIngressProxyLabels(nil), "protocol is incompatible")
	require.ErrorContains(t, validateIngressProxyLabels(map[string]string{
		ingressproxy.ProtocolLabel: "2",
	}), "protocol is incompatible")
}

func TestDockerIngressUsesOnlyAdmittedInternalNetworkPorts(t *testing.T) {
	executor := ingressTestExecutor()
	bindings, err := executor.admitIngress([]execution.IngressRequest{{Mode: "NetworkPublished",
		Endpoints: []string{"tcp://10.0.0.2:19000"}, ProbeEndpoints: []string{"tcp://127.0.0.1:19000"}}})
	require.NoError(t, err)
	require.Equal(t, []execution.IngressBinding{{Port: 19000, ProbeHost: "127.0.0.1", BindAddress: "10.0.0.2"}}, bindings)

	prepared := execution.PreparedWorkload{WorkloadID: "work", Generation: 1, Ingress: bindings}
	workloadOptions := executor.workloadIngressOptions(prepared)
	require.Empty(t, workloadOptions.portBindings)
	require.NotEqual(t, "ardents-ingress-test", string(workloadOptions.networkMode))
	options := executor.proxyIngressOptions(prepared)
	require.Equal(t, "ardents-ingress-test", string(options.networkMode))
	require.Len(t, options.exposedPorts, 1)
	require.Len(t, options.portBindings, 1)
	for _, ports := range options.portBindings {
		require.Len(t, ports, 2)
		require.Equal(t, "10.0.0.2", ports[0].HostIP.String())
		require.Equal(t, "127.0.0.1", ports[1].HostIP.String())
	}
	require.Contains(t, options.networking.EndpointsConfig, "ardents-ingress-test")
}

func TestDockerIngressAdmissionFailsClosed(t *testing.T) {
	executor := ingressTestExecutor()
	tests := []struct {
		name     string
		request  execution.IngressRequest
		contains string
	}{
		{name: "unpaired", request: execution.IngressRequest{Mode: "NetworkPublished", Endpoints: []string{"tcp://10.0.0.2:19000"}}, contains: "paired"},
		{name: "wrong host", request: execution.IngressRequest{Mode: "NetworkPublished", Endpoints: []string{"tcp://10.0.0.3:19000"}, ProbeEndpoints: []string{"tcp://127.0.0.1:19000"}}, contains: "not allowed"},
		{name: "dns host", request: execution.IngressRequest{Mode: "NetworkPublished", Endpoints: []string{"tcp://node.example:19000"}, ProbeEndpoints: []string{"tcp://127.0.0.1:19000"}}, contains: "literal"},
		{name: "privileged port", request: execution.IngressRequest{Mode: "NetworkPublished", Endpoints: []string{"tcp://10.0.0.2:443"}, ProbeEndpoints: []string{"tcp://127.0.0.1:443"}}, contains: "between 1024"},
		{name: "remote probe", request: execution.IngressRequest{Mode: "NetworkPublished", Endpoints: []string{"tcp://10.0.0.2:19000"}, ProbeEndpoints: []string{"tcp://10.0.0.2:19000"}}, contains: "loopback"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := executor.admitIngress([]execution.IngressRequest{test.request})
			require.ErrorContains(t, err, test.contains)
		})
	}
}

func TestDockerIngressRejectsDuplicateHostPort(t *testing.T) {
	executor := ingressTestExecutor()
	_, err := executor.admitIngress([]execution.IngressRequest{{Mode: "NetworkPublished",
		Endpoints:      []string{"tcp://10.0.0.2:19000", "http://10.0.0.2:19000/ready"},
		ProbeEndpoints: []string{"tcp://127.0.0.1:19000", "http://127.0.0.1:19000/ready"}}})
	require.ErrorContains(t, err, "duplicated")
}

func ingressTestExecutor() *Executor {
	return &Executor{allowedIngressHosts: normalizedSet([]string{"10.0.0.2"}),
		ingressBindAddress: "10.0.0.2", ingressNetworkName: "ardents-ingress-test",
		ingressProxyImage: "docker.io/ardents/ingress-proxy@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		allowedRegistries: normalizedSet([]string{"docker.io"}), nodeID: "node"}
}

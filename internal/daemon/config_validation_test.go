package daemon

import (
	"context"
	"testing"

	networkapi "ardents/internal/network"

	"github.com/stretchr/testify/require"
)

func TestConstrainedClientStartsDegradedWithoutProviders(t *testing.T) {
	n := NewNode(Config{
		Name: "constrained-client", NodeProfile: networkapi.NodeProfileConstrainedClient,
		Data: DataConfig{Dir: t.TempDir()},
	})
	require.NoError(t, n.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, n.Stop(context.Background())) })
	require.Equal(t, "degraded", n.trans.State())
	require.Contains(t, n.trans.Reason(), "no Filter, Lightpush, or Store providers")
}

func TestNodeRejectsUnsupportedProfileTransportCombinationBeforeStartup(t *testing.T) {
	n := NewNode(Config{
		Name: "invalid-profile-transport", NodeProfile: networkapi.NodeProfileLocalDevelopment,
		Transport: TransportConfig{Profile: networkapi.ProfileTCPWSS, WSSCertPath: "cert.pem", WSSKeyPath: "key.pem"},
		Data:      DataConfig{Dir: t.TempDir()},
	})
	beforeLifecycle, beforeTransport := n.life.State(), n.trans.State()

	err := n.Start(context.Background())

	require.ErrorContains(t, err, "does not support")
	require.Equal(t, beforeLifecycle, n.life.State())
	require.Equal(t, beforeTransport, n.trans.State())
}

func TestLocalDevelopmentRequiresLoopbackBeforeStartup(t *testing.T) {
	t.Setenv(networkapi.BindAddressEnv, "0.0.0.0")
	n := NewNode(Config{
		Name: "non-loopback-development", NodeProfile: networkapi.NodeProfileLocalDevelopment,
		Data: DataConfig{Dir: t.TempDir()},
	})
	beforeLifecycle := n.life.State()

	err := n.Start(context.Background())

	require.ErrorContains(t, err, "requires a loopback bind address")
	require.Equal(t, beforeLifecycle, n.life.State())
}

func TestLocalDevelopmentAcceptsExplicitLoopback(t *testing.T) {
	t.Setenv(networkapi.BindAddressEnv, "0.0.0.0")
	err := ValidateConfig(Config{
		NodeProfile: networkapi.NodeProfileLocalDevelopment,
		Transport:   TransportConfig{BindAddress: "127.0.0.1"},
	})

	require.NoError(t, err)
}

func TestLocalDevelopmentRejectsNetworkDNSDiscovery(t *testing.T) {
	err := ValidateConfig(Config{
		NodeProfile: networkapi.NodeProfileLocalDevelopment,
		Transport: TransportConfig{
			BindAddress: "127.0.0.1",
			DNSDiscoveryURLs: []string{
				"enrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@nodes.example.org",
			},
		},
	})

	require.ErrorContains(t, err, "does not allow network DNS discovery")
}

func TestServiceNodeRejectsLocalOnlyReachability(t *testing.T) {
	err := ValidateConfig(Config{
		NodeProfile: networkapi.NodeProfileServiceNode,
		Transport: TransportConfig{
			BindAddress:      "0.0.0.0",
			ReachabilityMode: networkapi.ReachabilityLocalOnly,
		},
	})

	require.ErrorContains(t, err, "does not allow reachability mode")
}

func TestPublicDirectRequiresExplicitPublicAddress(t *testing.T) {
	err := ValidateConfig(Config{
		NodeProfile: networkapi.NodeProfileServiceNode,
		Transport: TransportConfig{
			BindAddress:      "0.0.0.0",
			ReachabilityMode: networkapi.ReachabilityPublicDirect,
		},
	})

	require.ErrorContains(t, err, "requires at least one public advertised address")
}

func TestBrowserReachabilityModeIsRejected(t *testing.T) {
	err := ValidateConfig(Config{
		NodeProfile: networkapi.NodeProfileServiceNode,
		Transport: TransportConfig{
			BindAddress:      "0.0.0.0",
			ReachabilityMode: "browser",
		},
	})

	require.ErrorContains(t, err, "unsupported reachability mode")
}

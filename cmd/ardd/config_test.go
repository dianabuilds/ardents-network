package main

import (
	"os"
	"path/filepath"
	"testing"

	transport "ardents/internal/network/api"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestLoadRuntimeConfigDefaults(t *testing.T) {
	t.Setenv("ARDENTS_API_TOKEN", "token")
	t.Setenv(apiTokenFileEnv, "")
	t.Setenv("ARDENTS_ADDR", "127.0.0.1:19090")
	t.Setenv(nodeNameEnv, "")
	t.Setenv(dataDirEnv, "")
	t.Setenv(storePathEnv, "")
	t.Setenv(bootstrapPeersEnv, "")
	t.Setenv(trustAnchorsEnv, "")
	t.Setenv(transportProfileEnv, "")
	t.Setenv(transportPortEnv, "")
	t.Setenv(nodeProfileEnv, "")
	t.Setenv(wssPortEnv, "")
	t.Setenv(wssCertPathEnv, "")
	t.Setenv(wssKeyPathEnv, "")
	t.Setenv(wssCAPathEnv, "")
	t.Setenv(wssAdvertiseEnv, "")
	t.Setenv(dnsDiscoveryURLsEnv, "")
	t.Setenv(dnsNameServerEnv, "")
	t.Setenv(reachabilityModeEnv, "")
	t.Setenv(advertiseAddrsEnv, "")
	t.Setenv(transport.BindAddressEnv, "")
	t.Setenv(workloadExecutorEnv, "")
	t.Setenv(workloadRegistriesEnv, "")
	t.Setenv(workloadPolicyRefsEnv, "")
	t.Setenv(workloadTrustedRuntimeEnv, "")
	t.Setenv(workloadUntrustedRuntimeEnv, "")

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, "token", cfg.APIToken)
	require.Equal(t, "127.0.0.1:19090", cfg.ListenAddr)
	require.Equal(t, "ardd", cfg.Node.Name)
	require.Equal(t, filepath.Join("var", "ardd"), cfg.Node.Data.Dir)
	require.Equal(t, filepath.Join("var", "ardd", "waku-store.db"), cfg.Node.Transport.StorePath)
	require.Empty(t, cfg.Node.Boot.Sources)
	require.Empty(t, cfg.Node.Trust.Anchors)
	require.Zero(t, cfg.Node.Transport.ListenPort)
	require.Equal(t, transport.ProfileTCPOnly, transport.NormalizeProfile(cfg.Node.Transport.Profile))
	require.Equal(t, transport.NodeProfileServiceNode, transport.NormalizeNodeProfile(cfg.Node.NodeProfile))
	require.Equal(t, transport.ReachabilityPrivateLAN, cfg.Node.Transport.ReachabilityMode)
}

func TestLoadRuntimeConfigAppliesWorkloadSecurityPolicy(t *testing.T) {
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(workloadRegistriesEnv, "docker.io, registry.example")
	t.Setenv(workloadPolicyRefsEnv, "trusted")
	t.Setenv(workloadTrustedRuntimeEnv, "runc-secure")
	t.Setenv(workloadUntrustedRuntimeEnv, "runsc-secure")

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, []string{"trusted"}, cfg.Node.Policy.AllowedPolicyRefs)
	require.NotNil(t, cfg.Node.WorkloadExecutor)
}

func TestLoadRuntimeConfigRejectsTrustedProcessOutsideDevelopmentProfile(t *testing.T) {
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(workloadExecutorEnv, "trusted-process")
	t.Setenv(nodeProfileEnv, string(transport.NodeProfileServiceNode))

	_, err := loadRuntimeConfig()
	require.ErrorContains(t, err, "requires local_development")
}

func TestLoadRuntimeConfigAllowsExplicitTrustedProcessDevelopmentMode(t *testing.T) {
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(workloadExecutorEnv, "trusted-process")
	t.Setenv(nodeProfileEnv, string(transport.NodeProfileLocalDevelopment))
	t.Setenv(transport.BindAddressEnv, "127.0.0.1")

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg.Node.WorkloadExecutor)
}

func TestLoadRuntimeConfigRejectsUnknownWorkloadExecutor(t *testing.T) {
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(workloadExecutorEnv, "process-fallback")

	_, err := loadRuntimeConfig()
	require.ErrorContains(t, err, "unsupported mode")
}

func TestLoadRuntimeConfigAppliesBootstrapTrustAndProfile(t *testing.T) {
	t.Setenv("ARDENTS_API_TOKEN", "token")
	t.Setenv(transport.BindAddressEnv, "127.0.0.1")
	t.Setenv(nodeNameEnv, "node-a")
	t.Setenv(dataDirEnv, "/data/node-a")
	t.Setenv(storePathEnv, "/store/node-a.db")
	t.Setenv(bootstrapPeersEnv, "/ip4/10.0.0.2/tcp/60000/p2p/peer-a, /ip4/10.0.0.3/tcp/60001/p2p/peer-b")
	t.Setenv(trustAnchorsEnv, "anchor-a\nanchor-b")
	t.Setenv(transportProfileEnv, string(transport.ProfileTCPOnly))
	t.Setenv(nodeProfileEnv, string(transport.NodeProfileLocalDevelopment))
	t.Setenv(transportPortEnv, "61000")

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, "node-a", cfg.Node.Name)
	require.Equal(t, "/data/node-a", cfg.Node.Data.Dir)
	require.Equal(t, "/store/node-a.db", cfg.Node.Transport.StorePath)
	require.Equal(t, 61000, cfg.Node.Transport.ListenPort)
	require.Equal(t, []string{
		"/ip4/10.0.0.2/tcp/60000/p2p/peer-a",
		"/ip4/10.0.0.3/tcp/60001/p2p/peer-b",
	}, cfg.Node.Boot.Sources)
	require.Equal(t, []string{"anchor-a", "anchor-b"}, cfg.Node.Trust.Anchors)
	require.Equal(t, transport.ProfileTCPOnly, cfg.Node.Transport.Profile)
	require.Equal(t, transport.NodeProfileLocalDevelopment, cfg.Node.NodeProfile)
	require.Equal(t, transport.ReachabilityLocalOnly, cfg.Node.Transport.ReachabilityMode)
}

func TestLoadRuntimeConfigAppliesPublicDirectReachability(t *testing.T) {
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(transport.BindAddressEnv, "0.0.0.0")
	t.Setenv(reachabilityModeEnv, string(transport.ReachabilityPublicDirect))
	t.Setenv(advertiseAddrsEnv, "/dns4/node.example/tcp/61000")

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, transport.ReachabilityPublicDirect, cfg.Node.Transport.ReachabilityMode)
	require.Equal(t, []string{"/dns4/node.example/tcp/61000"}, cfg.Node.Transport.AdvertiseAddresses)
}

func TestLoadRuntimeConfigRejectsPublicDirectOnLoopback(t *testing.T) {
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(transport.BindAddressEnv, "127.0.0.1")
	t.Setenv(reachabilityModeEnv, string(transport.ReachabilityPublicDirect))
	t.Setenv(advertiseAddrsEnv, "/dns4/node.example/tcp/61000")

	_, err := loadRuntimeConfig()
	require.ErrorContains(t, err, "non-loopback")
}

const signedDNSDiscoveryTreeForTest = "enrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@nodes.example.org"

func TestLoadRuntimeConfigAppliesDNSDiscovery(t *testing.T) {
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(dnsDiscoveryURLsEnv, signedDNSDiscoveryTreeForTest)
	t.Setenv(dnsNameServerEnv, "1.1.1.1")

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, []string{signedDNSDiscoveryTreeForTest}, cfg.Node.Transport.DNSDiscoveryURLs)
	require.Equal(t, "1.1.1.1", cfg.Node.Transport.DNSDiscoveryNameServer)
}

func TestLoadRuntimeConfigRejectsUnsignedDNSDiscoverySource(t *testing.T) {
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(dnsDiscoveryURLsEnv, "https://nodes.example.org")

	_, err := loadRuntimeConfig()
	require.ErrorContains(t, err, "signed enrtree")
}

func TestLoadRuntimeConfigAcceptsConstrainedClientProfile(t *testing.T) {
	t.Setenv("ARDENTS_API_TOKEN", "token")
	t.Setenv(nodeProfileEnv, string(transport.NodeProfileConstrainedClient))

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, transport.NodeProfileConstrainedClient, cfg.Node.NodeProfile)
	require.Equal(t, transport.ReachabilityOutboundOnly, cfg.Node.Transport.ReachabilityMode)
}

func TestLoadRuntimeConfigRejectsWSSBeforeCertificateConfigurationIsComplete(t *testing.T) {
	t.Setenv("ARDENTS_API_TOKEN", "token")
	t.Setenv(transportProfileEnv, string(transport.ProfileTCPWSS))

	_, err := loadRuntimeConfig()
	require.ErrorContains(t, err, "requires secure websocket certificate")
}

func TestLoadRuntimeConfigAcceptsCompleteWSSOperatorConfiguration(t *testing.T) {
	certPath, keyPath := testkit.WriteWSSCert(t)
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(transportProfileEnv, string(transport.ProfileTCPWSS))
	t.Setenv(wssPortEnv, "61443")
	t.Setenv(wssCertPathEnv, certPath)
	t.Setenv(wssKeyPathEnv, keyPath)
	t.Setenv(wssCAPathEnv, testkit.WSSCAPath(certPath))
	t.Setenv(wssAdvertiseEnv, "127.0.0.1")

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, 61443, cfg.Node.Transport.WSSPort)
	require.Equal(t, "127.0.0.1", cfg.Node.Transport.WSSAdvertiseAddress)
}

func TestLoadRuntimeConfigRejectsWSSSettingsForTCPOnly(t *testing.T) {
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(transportProfileEnv, string(transport.ProfileTCPOnly))
	t.Setenv(wssPortEnv, "61443")

	_, err := loadRuntimeConfig()
	require.ErrorContains(t, err, "does not accept secure websocket settings")
}

func TestLoadRuntimeConfigRejectsInvalidPort(t *testing.T) {
	t.Setenv("ARDENTS_API_TOKEN", "token")
	t.Setenv(transportPortEnv, "70000")

	_, err := loadRuntimeConfig()
	require.Error(t, err)
	require.Contains(t, err.Error(), transportPortEnv)
}

func TestLoadRuntimeConfigAppliesNetworkLimits(t *testing.T) {
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(maxMessageBytesEnv, "131072")
	t.Setenv(maxPeerConnsEnv, "48")
	t.Setenv(maxConnsPerIPEnv, "3")
	t.Setenv(maxNetworkOpsEnv, "12")
	t.Setenv(networkRateEnv, "15")
	t.Setenv(networkBurstEnv, "30")
	t.Setenv(maxFilterSubsEnv, "24")
	t.Setenv(maxStoreResultsEnv, "96")

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, transport.Limits{
		MaxMessageBytes: 131072, MaxPeerConnections: 48, MaxConnectionsPerIP: 3,
		MaxConcurrentOperations: 12, OperationRate: 15, OperationBurst: 30,
		MaxFilterSubscribers: 24, MaxStoreResults: 96,
	}, cfg.Node.Transport.Limits)
}

func TestLoadRuntimeConfigRejectsUnsafeNetworkLimit(t *testing.T) {
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(maxMessageBytesEnv, "200000")

	_, err := loadRuntimeConfig()
	require.ErrorContains(t, err, "maximum message bytes")
}

func TestLoadRuntimeConfigRejectsInvalidProfile(t *testing.T) {
	t.Setenv("ARDENTS_API_TOKEN", "token")
	t.Setenv(transportProfileEnv, "broken")

	_, err := loadRuntimeConfig()
	require.Error(t, err)
	require.Contains(t, err.Error(), transportProfileEnv)
}

func TestLoadRuntimeConfigRejectsRemotePlaintextAPI(t *testing.T) {
	t.Setenv(apiTokenEnv, "token")
	t.Setenv(apiTokenFileEnv, "")
	t.Setenv("ARDENTS_ADDR", "0.0.0.0:8080")

	_, err := loadRuntimeConfig()
	require.Error(t, err)
	require.Contains(t, err.Error(), "loopback")
}

func TestLoadRuntimeConfigRequiresToken(t *testing.T) {
	t.Setenv("ARDENTS_API_TOKEN", "")
	t.Setenv(apiTokenFileEnv, "")

	_, err := loadRuntimeConfig()
	require.Error(t, err)
	require.Contains(t, err.Error(), apiTokenEnv)
}

func TestLoadRuntimeConfigReadsTokenFromRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-token")
	require.NoError(t, os.WriteFile(path, []byte("file-token\n"), 0o600))
	t.Setenv(apiTokenEnv, "")
	t.Setenv(apiTokenFileEnv, path)

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, "file-token", cfg.APIToken)
}

func TestLoadRuntimeConfigRejectsAmbiguousTokenSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-token")
	require.NoError(t, os.WriteFile(path, []byte("file-token"), 0o600))
	t.Setenv(apiTokenEnv, "env-token")
	t.Setenv(apiTokenFileEnv, path)

	_, err := loadRuntimeConfig()
	require.Error(t, err)
	require.Contains(t, err.Error(), "only one")
}

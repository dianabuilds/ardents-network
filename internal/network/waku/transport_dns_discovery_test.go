package waku

import (
	"ardents/internal/network"
	"context"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	gethdns "github.com/ethereum/go-ethereum/p2p/dnsdisc"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/stretchr/testify/require"
)

const signedTestTree = "enrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@nodes.example.org"

type fakeDNSDiscovery struct {
	peers []string
	err   error
}

type mapTXTResolver map[string]string

func (r mapTXTResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	if record, ok := r[name]; ok {
		return []string{record}, nil
	}
	return nil, errors.New("record not found")
}

func (f *fakeDNSDiscovery) Retrieve(
	context.Context, []string, string, network.Profile,
) ([]string, error) {
	return cloneStrings(f.peers), f.err
}

func TestDiscoveryConfigAcceptsSignedTreeAndIPNameserver(t *testing.T) {
	err := validateDiscoveryConfig(network.Config{
		DNSDiscoveryURLs: []string{signedTestTree}, DNSDiscoveryNameServer: "1.1.1.1",
	})
	require.NoError(t, err)
}

func TestDiscoveryConfigRejectsUnsignedOrAmbiguousInputs(t *testing.T) {
	require.Error(t, validateDiscoveryConfig(network.Config{DNSDiscoveryURLs: []string{"https://nodes.example.org"}}))
	require.Error(t, validateDiscoveryConfig(network.Config{DNSDiscoveryNameServer: "1.1.1.1"}))
	require.Error(t, validateDiscoveryConfig(network.Config{
		DNSDiscoveryURLs: []string{signedTestTree}, DNSDiscoveryNameServer: "resolver.example.org",
	}))
}

func TestDNSRefreshReplacesPeersAndClearsStaleResultsOnFailure(t *testing.T) {
	peerAddress := "/ip4/127.0.0.1/tcp/61000/p2p/12D3KooWJ5YxrcQ6z7E9jcwQJ2u2dP1zrVt1a1dN1oT2f3x4a5b6"
	resolver := &fakeDNSDiscovery{peers: []string{peerAddress}}
	svc := New(network.Config{DNSDiscoveryURLs: []string{signedTestTree}})
	svc.dnsDiscovery = resolver

	require.NoError(t, svc.refreshDNSPeers(context.Background()))
	require.Equal(t, []string{peerAddress}, svc.effectiveBootstrapNodesLocked())
	svc.observed[peerAddress] = endpointObservation{usable: true}

	resolver.err = errors.New("resolver unavailable")
	require.Error(t, svc.refreshDNSPeers(context.Background()))
	require.Empty(t, svc.effectiveBootstrapNodesLocked())
	_, retained := svc.observed[peerAddress]
	require.False(t, retained)
	require.Equal(t, "bootstrap source discovery failed", svc.currentBootstrapStatusViewLocked().Reason)
}

func TestWakuDNSDiscoveryVerifiesSignedTreeAndReturnsTCPPeers(t *testing.T) {
	const rawENR = "enr:-Ji4QAa0VR5P27XvDEZzuFf1lnO6OGzm4hPhVtVYPFqlB-9vZnZtc-lzmEqY4stHFTIazRnSzwhlYne0UMIAmFMZ8o2GAYwawiLNgmlkgnY0gmlwhMCoAWSJc2VjcDI1NmsxoQLtnTLtFmyU8AFqO8Jw4X9zBfB6fWJxsMk9YpyrPeNPkoN0Y3CCw6qDdWRwgsm6hXdha3UyAQ"
	var node enode.Node
	require.NoError(t, node.UnmarshalText([]byte(rawENR)))
	tree, err := gethdns.MakeTree(1, []*enode.Node{&node}, nil)
	require.NoError(t, err)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	url, err := tree.Sign(key, "nodes.test")
	require.NoError(t, err)
	resolver := mapTXTResolver(tree.ToTXT("nodes.test"))

	peers, err := (wakuDNSPeerDiscovery{resolver: resolver}).Retrieve(
		context.Background(), []string{signedTestTree, url}, "", network.ProfileTCPOnly,
	)
	require.NoError(t, err)
	require.NotEmpty(t, peers)
	for _, address := range peers {
		require.Contains(t, address, "/tcp/")
		require.Contains(t, address, "/p2p/")
	}
}

func TestDiscoveredAddressFilterPreservesConfiguredCarrierFamilies(t *testing.T) {
	tcp := "/ip4/192.0.2.1/tcp/60000/p2p/peer"
	wss := "/dns4/node.example/tcp/443/tls/ws/p2p/peer"
	plaintextWS := "/dns4/node.example/tcp/80/ws/p2p/peer"
	quic := "/ip4/192.0.2.1/udp/60000/quic-v1/p2p/peer"

	require.True(t, allowedDiscoveredAddress(tcp, network.ProfileTCPOnly))
	require.False(t, allowedDiscoveredAddress(wss, network.ProfileTCPOnly))
	require.True(t, allowedDiscoveredAddress(wss, network.ProfileTCPWSS))
	require.False(t, allowedDiscoveredAddress(plaintextWS, network.ProfileTCPWSS))
	require.False(t, allowedDiscoveredAddress(quic, network.ProfileTCPWSS))
}

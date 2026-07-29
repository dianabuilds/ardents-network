//go:build integration

package waku

import (
	"context"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"ardents/internal/network"

	"github.com/ethereum/go-ethereum/crypto"
	gethdns "github.com/ethereum/go-ethereum/p2p/dnsdisc"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	wenr "github.com/waku-org/go-waku/waku/v2/protocol/enr"
	"go.uber.org/zap"
)

func TestSignedDNSColdStartAndPeerRestartRecovery(t *testing.T) {
	port := reserveTCPPort(t)
	keyPath := filepath.Join(t.TempDir(), "remote-waku-key.json")
	remote := New(network.Config{
		BindAddress: "127.0.0.1", ListenPort: port, PrivateKeyPath: keyPath,
	})
	require.NoError(t, remote.Start(t.Context()))
	t.Cleanup(func() { _ = remote.Stop(context.Background()) })

	url, resolver := signedTreeForTransport(t, keyPath, remote.Endpoints()[0])
	local := New(network.Config{BindAddress: "127.0.0.1", DNSDiscoveryURLs: []string{url}})
	local.dnsDiscovery = wakuDNSPeerDiscovery{resolver: resolver}
	deadStatic := strings.Replace(remote.Endpoints()[0], "/tcp/"+strconv.Itoa(port)+"/", "/tcp/1/", 1)
	local.SetBootstrapNodes([]string{deadStatic})
	require.NoError(t, local.Start(t.Context()))
	t.Cleanup(func() { _ = local.Stop(context.Background()) })
	waitForRelayPeers(t, local, 1)
	require.True(t, local.BootstrapStatus().Joined)

	require.NoError(t, remote.Stop(t.Context()))
	waitForBootstrapState(t, local, "degraded")
	require.NoError(t, remote.Start(t.Context()))
	waitForRelayPeers(t, local, 1)
	require.True(t, local.BootstrapStatus().Joined)

	local.mu.Lock()
	local.dnsDiscovery = &fakeDNSDiscovery{}
	local.mu.Unlock()
	require.Error(t, local.refreshDNSPeers(t.Context()))
	waitForRelayPeerLoss(t, local, 0)
	require.Equal(t, "bootstrap source discovery failed", local.BootstrapStatus().Reason)
}

func TestSignedDNSReplenishesToRelayPeerTarget(t *testing.T) {
	remotes := make([]*Service, 0, desiredRelayPeers)
	peers := make([]dnsTreePeer, 0, desiredRelayPeers)
	for index := 0; index < desiredRelayPeers; index++ {
		keyPath := filepath.Join(t.TempDir(), "remote-waku-key.json")
		remote := New(network.Config{
			BindAddress: "127.0.0.1", ListenPort: reserveTCPPort(t), PrivateKeyPath: keyPath,
		})
		require.NoError(t, remote.Start(t.Context()))
		remotes = append(remotes, remote)
		peers = append(peers, dnsTreePeer{keyPath: keyPath, endpoint: remote.Endpoints()[0]})
	}
	t.Cleanup(func() {
		for _, remote := range remotes {
			_ = remote.Stop(context.Background())
		}
	})

	url, resolver := signedTreeForTransports(t, peers)
	local := New(network.Config{BindAddress: "127.0.0.1", DNSDiscoveryURLs: []string{url}})
	local.dnsDiscovery = wakuDNSPeerDiscovery{resolver: resolver}
	require.NoError(t, local.Start(t.Context()))
	t.Cleanup(func() { _ = local.Stop(context.Background()) })
	waitForRelayPeers(t, local, desiredRelayPeers)

	require.NoError(t, remotes[2].Stop(t.Context()))
	waitForRelayPeerLoss(t, local, desiredRelayPeers-1)
	require.NoError(t, remotes[2].Start(t.Context()))
	waitForRelayPeers(t, local, desiredRelayPeers)
}

type dnsTreePeer struct {
	keyPath  string
	endpoint string
}

func signedTreeForTransport(t *testing.T, keyPath, endpoint string) (string, mapTXTResolver) {
	t.Helper()
	return signedTreeForTransports(t, []dnsTreePeer{{keyPath: keyPath, endpoint: endpoint}})
}

func signedTreeForTransports(t *testing.T, peers []dnsTreePeer) (string, mapTXTResolver) {
	t.Helper()
	nodes := make([]*enode.Node, 0, len(peers))
	for _, item := range peers {
		nodes = append(nodes, enrNodeForTransport(t, item.keyPath, item.endpoint))
	}
	tree, err := gethdns.MakeTree(1, nodes, nil)
	require.NoError(t, err)
	signingKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	url, err := tree.Sign(signingKey, "nodes.test")
	require.NoError(t, err)
	return url, tree.ToTXT("nodes.test")
}

func enrNodeForTransport(t *testing.T, keyPath, endpoint string) *enode.Node {
	t.Helper()
	key, err := newTransportKeyStore(keyPath).load()
	require.NoError(t, err)
	require.NotNil(t, key)
	address, err := multiaddr.NewMultiaddr(endpoint)
	require.NoError(t, err)
	ipText, err := address.ValueForProtocol(multiaddr.P_IP4)
	require.NoError(t, err)
	portText, err := address.ValueForProtocol(multiaddr.P_TCP)
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	db, err := enode.OpenDB("")
	require.NoError(t, err)
	t.Cleanup(db.Close)
	localNode := enode.NewLocalNode(db, key)
	err = wenr.UpdateLocalNode(zap.NewNop(), localNode, &wenr.LocalNodeParams{
		IPAddr:    &net.TCPAddr{IP: net.ParseIP(ipText), Port: port},
		WakuFlags: wenr.NewWakuEnrBitfield(true, true, true, true),
	})
	require.NoError(t, err)
	return localNode.Node()
}

func waitForRelayPeers(t *testing.T, svc *Service, count int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if svc.RelayPeerCount("") >= count {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.FailNow(t, "relay peer count did not recover")
}

func waitForBootstrapState(t *testing.T, svc *Service, state string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if svc.BootstrapStatus().State == state {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.FailNowf(t, "bootstrap state did not change", "status=%+v", svc.BootstrapStatus())
}

func waitForRelayPeerLoss(t *testing.T, svc *Service, maximum int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if svc.RelayPeerCount("") <= maximum {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.FailNow(t, "relay peer loss was not observed")
}

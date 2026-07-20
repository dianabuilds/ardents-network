package transport

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	networkreadiness "ardents/internal/network/readiness"

	"github.com/stretchr/testify/require"
)

func TestTransportRejectsUnsupportedProfileBeforePersistenceCreation(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "waku-store.db")
	keyPath := filepath.Join(dir, "waku-key.json")
	svc := New(Config{Profile: networkreadiness.ProfileTCPQUIC, StorePath: storePath, PrivateKeyPath: keyPath})

	err := svc.Start(context.Background())

	require.ErrorContains(t, err, "not implemented")
	_, storeErr := os.Stat(storePath)
	require.ErrorIs(t, storeErr, os.ErrNotExist)
	_, keyErr := os.Stat(keyPath)
	require.ErrorIs(t, keyErr, os.ErrNotExist)
}

func TestLibP2POptionsForDefinitionSupportsTCPWSS(t *testing.T) {
	definition, err := networkreadiness.ResolveProfile(networkreadiness.ProfileTCPWSS)
	require.NoError(t, err)
	opts, err := libP2POptionsForDefinition(definition)
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestTransportStartUsesConfiguredListenPort(t *testing.T) {
	port := reserveTCPPort(t)
	svc := New(Config{ListenPort: port})
	require.NoError(t, svc.Start(t.Context()))
	t.Cleanup(func() {
		_ = svc.Stop(t.Context())
	})

	endpoints := svc.Endpoints()
	require.NotEmpty(t, endpoints)
	for _, endpoint := range endpoints {
		if strings.Contains(endpoint, "/tcp/"+strconv.Itoa(port)+"/") {
			return
		}
	}
	require.FailNowf(t, "configured port missing", "endpoints = %v", endpoints)
}

func TestListenPortClampsNegativeValuesToEphemeral(t *testing.T) {
	require.Zero(t, listenPort(Config{ListenPort: -1}))
	require.Equal(t, 61020, listenPort(Config{ListenPort: 61020}))
}

func TestTransportProfileTCPWSSRequiresCertMaterial(t *testing.T) {
	svc := New(Config{Profile: networkreadiness.ProfileTCPWSS})

	err := svc.Start(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires secure websocket certificate and key paths")
}

func TestAdvertisedEndpointsReplaceWSSBindHostOnly(t *testing.T) {
	endpoints := []string{
		"/ip4/127.0.0.1/tcp/61020/p2p/peer",
		"/ip4/0.0.0.0/tcp/61443/tls/ws/p2p/peer",
	}
	got := advertisedEndpoints(endpoints, Config{
		Profile: networkreadiness.ProfileTCPWSS, WSSPort: 61443, WSSAdvertiseAddress: "wss.example.test",
	})

	require.Equal(t, "/ip4/127.0.0.1/tcp/61020/p2p/peer", got[0])
	require.Equal(t, "/dns4/wss.example.test/tcp/61443/tls/ws/p2p/peer", got[1])
}

func reserveTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

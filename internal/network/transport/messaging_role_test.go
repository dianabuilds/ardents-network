package transport

import (
	"context"
	"path/filepath"
	"testing"

	networkprivacy "ardents/internal/network/privacy"
	networkreadiness "ardents/internal/network/readiness"

	"github.com/stretchr/testify/require"
)

func TestConstrainedClientDoesNotExposeRelayOperations(t *testing.T) {
	dir := t.TempDir()
	svc := New(Config{
		NodeProfile: networkreadiness.NodeProfileConstrainedClient,
		StorePath:   filepath.Join(dir, "unused-store.db"), PrivateKeyPath: filepath.Join(dir, "key.json"),
		BindAddress: "127.0.0.1", Profile: networkreadiness.ProfileTCPOnly,
		ReachabilityMode: networkreadiness.ReachabilityOutboundOnly,
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })

	envelope := networkprivacy.SealedEnvelope{ContentTopic: "opaque", Payload: []byte("ciphertext")}
	require.ErrorContains(t, svc.PublishPrivateEnvelope(context.Background(), envelope), "unavailable")
	_, err := svc.SubscribePrivateEnvelopes(context.Background(), "opaque")
	require.ErrorContains(t, err, "unavailable")
	require.Zero(t, svc.RelayPeerCount(networkreadiness.DefaultPubsubTopic()))
	require.NoFileExists(t, filepath.Join(dir, "unused-store.db"))
}

func TestFullNodeDoesNotExposeFilterClientOperation(t *testing.T) {
	dir := t.TempDir()
	svc := New(Config{
		NodeProfile: networkreadiness.NodeProfileServiceNode,
		StorePath:   filepath.Join(dir, "store.db"), PrivateKeyPath: filepath.Join(dir, "key.json"),
		BindAddress: "127.0.0.1", Profile: networkreadiness.ProfileTCPOnly,
		ReachabilityMode: networkreadiness.ReachabilityLocalOnly,
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })

	_, err := svc.SubscribePrivateFilter(context.Background(), []string{"/ip4/127.0.0.1/tcp/1"}, "opaque")
	require.ErrorContains(t, err, "only in constrained")
}

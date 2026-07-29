package waku

import (
	"context"
	"path/filepath"
	"testing"

	"ardents/internal/network"

	"github.com/stretchr/testify/require"
)

func TestConstrainedClientDoesNotExposeRelayOperations(t *testing.T) {
	dir := t.TempDir()
	svc := New(network.Config{
		NodeProfile: network.NodeProfileConstrainedClient,
		StorePath:   filepath.Join(dir, "unused-store.db"), PrivateKeyPath: filepath.Join(dir, "key.json"),
		BindAddress: "127.0.0.1", Profile: network.ProfileTCPOnly,
		ReachabilityMode: network.ReachabilityOutboundOnly,
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })

	envelope := network.Envelope{ContentTopic: "opaque", Payload: []byte("ciphertext")}
	require.ErrorContains(t, svc.PublishRelayEnvelope(context.Background(), envelope), "unavailable")
	_, err := svc.SubscribeRelayEnvelopes(context.Background(), network.DefaultPubsubTopic, "opaque")
	require.ErrorContains(t, err, "unavailable")
	require.Zero(t, svc.RelayPeerCount(network.DefaultPubsubTopic))
	require.NoFileExists(t, filepath.Join(dir, "unused-store.db"))
}

func TestFullNodeDoesNotExposeFilterClientOperation(t *testing.T) {
	dir := t.TempDir()
	svc := New(network.Config{
		NodeProfile: network.NodeProfileServiceNode,
		StorePath:   filepath.Join(dir, "store.db"), PrivateKeyPath: filepath.Join(dir, "key.json"),
		BindAddress: "127.0.0.1", Profile: network.ProfileTCPOnly,
		ReachabilityMode: network.ReachabilityOutboundOnly,
	})
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, svc.Stop(context.Background())) })

	_, err := svc.SubscribeFilterEnvelopes(context.Background(), []string{"/ip4/127.0.0.1/tcp/1"}, "opaque")
	require.ErrorContains(t, err, "only in constrained")
}

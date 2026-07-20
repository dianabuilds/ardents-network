package testkit

import (
	"testing"
	"time"

	networkprivacy "ardents/internal/network/privacy"

	"github.com/stretchr/testify/require"
)

func TestDiscoveryAndDataFixturesUseDistinctStoreTopics(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	discovery := NewDiscoveryPrivacyFixture(t, now)
	data := NewDataPrivacyFixture(t, now)
	discoveryTopic, err := discovery.Sender.StoreContentTopic()
	require.NoError(t, err)
	dataTopic, err := data.Sender.StoreContentTopic()
	require.NoError(t, err)
	require.NotEqual(t, discoveryTopic, dataTopic)
}

func TestDataPrivacyGroupUsesIndependentReplayLedgers(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	group := NewDataPrivacyGroupFixture(t, now, 3)
	sealed, err := group.Channels[0].Seal(networkprivacy.MessageClassBlobReplicaControl, 1, []byte("group message"))
	require.NoError(t, err)
	for _, receiver := range group.Channels[1:] {
		opened, openErr := receiver.Open(sealed)
		require.NoError(t, openErr)
		require.Equal(t, []byte("group message"), opened.Payload)
	}
}

func TestDataPrivacyGroupCanRevokeOneSenderAtOneReceiver(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	group := NewDataPrivacyGroupFixture(t, now, 3)
	group.RevokeSender(t, 1, 0, now)
	sealed, err := group.Channels[0].Seal(networkprivacy.MessageClassBlobReplicaControl, 1, []byte("revoked"))
	require.NoError(t, err)
	_, err = group.Channels[1].Open(sealed)
	require.Error(t, err)
	opened, err := group.Channels[2].Open(sealed)
	require.NoError(t, err)
	require.Equal(t, []byte("revoked"), opened.Payload)
}

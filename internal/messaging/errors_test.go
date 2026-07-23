package messaging

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsChannelGrantFailureRejectsLegacyCapabilityPrefix(t *testing.T) {
	require.True(t, IsChannelGrantFailure(channelFailure{"privacy.channel_grant.revoked"}))
	require.False(t, IsChannelGrantFailure(channelFailure{"privacy.capability.revoked"}))
	require.False(t, IsChannelGrantFailure(channelFailure{"privacy.envelope.expired"}))
}

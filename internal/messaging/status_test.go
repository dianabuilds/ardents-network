package messaging

import (
	"ardents/internal/network"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatusSnapshotReportsActivePrivateProfile(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	discovery := newTestChannel(t, &channelResolver{resolved: fixture.senderResolved}, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xf1)
	data := newTestChannel(t, &channelResolver{resolved: fixture.senderResolved}, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xf2)

	status := Snapshot(discovery, data)

	require.Equal(t, ProfileV1, status.Profile)
	require.Equal(t, StatusActive, status.State)
	require.Equal(t, ReasonChannelGrantReady, status.SwitchReason)
	require.Equal(t, RecoverySteady, status.RecoveryState)
	require.Empty(t, status.ReducedFeatures)
	require.Empty(t, status.ErrorCategories)
}

func TestStatusSnapshotReportsSafeChannelGrantFailureOnly(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	resolver := &channelResolver{resolved: fixture.senderResolved, err: channelFailure{"privacy.channel_grant.revoked"}}
	channel := newTestChannel(t, resolver, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xf3)

	status := Snapshot(channel, channel)

	require.Equal(t, StatusDegraded, status.State)
	require.Equal(t, "privacy.channel_grant.revoked", status.SwitchReason)
	require.Equal(t, RecoveryPending, status.RecoveryState)
	require.ElementsMatch(t, []network.TransportFeature{
		network.TransportFeaturePrivatePublication,
		network.TransportFeaturePrivateDiscovery,
		network.TransportFeaturePrivateDataExchange,
	}, status.ReducedFeatures)
	require.Equal(t, []string{"privacy.channel_grant.revoked"}, status.ErrorCategories)
}

func TestStatusSnapshotReturnsToSteadyAfterChannelGrantRecovery(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	resolver := &channelResolver{resolved: fixture.senderResolved, err: channelFailure{"privacy.channel_grant.expired"}}
	channel := newTestChannel(t, resolver, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xf5)
	require.Equal(t, RecoveryPending, Snapshot(channel, channel).RecoveryState)

	resolver.err = nil
	recovered := Snapshot(channel, channel)

	require.Equal(t, StatusActive, recovered.State)
	require.Equal(t, RecoverySteady, recovered.RecoveryState)
	require.Equal(t, ReasonChannelGrantReady, recovered.SwitchReason)
	require.Empty(t, recovered.ErrorCategories)
}

func TestStatusSnapshotDoesNotExposeUnexpectedResolverError(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	resolver := &channelResolver{resolved: fixture.senderResolved, err: errors.New("secret path C:/sensitive/capability.db")}
	channel := newTestChannel(t, resolver, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xf4)

	status := Snapshot(channel, channel)

	require.Equal(t, CodeChannelGrantUnavailable, status.SwitchReason)
	require.NotContains(t, status.SwitchReason, "sensitive")
	require.Equal(t, []string{CodeChannelGrantUnavailable}, status.ErrorCategories)
}

func TestStatusSnapshotRejectsLegacyChannelGrantCode(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	resolver := &channelResolver{resolved: fixture.senderResolved, err: channelFailure{"privacy.capability.revoked"}}
	channel := newTestChannel(t, resolver, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xf6)

	status := Snapshot(channel, channel)

	require.Equal(t, CodeChannelGrantUnavailable, status.SwitchReason)
	require.Equal(t, []string{CodeChannelGrantUnavailable}, status.ErrorCategories)
}

func TestStatusSnapshotRejectsUnrecognizedChannelGrantCode(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	resolver := &channelResolver{resolved: fixture.senderResolved, err: channelFailure{"privacy.channel_grant.secret_path"}}
	channel := newTestChannel(t, resolver, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xf7)

	status := Snapshot(channel, channel)

	require.Equal(t, CodeChannelGrantUnavailable, status.SwitchReason)
	require.Equal(t, []string{CodeChannelGrantUnavailable}, status.ErrorCategories)
}

func TestStatusSnapshotReportsMissingConfigurationAsBlocked(t *testing.T) {
	status := Snapshot(nil, nil)

	require.Equal(t, StatusDegraded, status.State)
	require.Equal(t, CodeChannelGrantMissing, status.SwitchReason)
	require.Equal(t, RecoveryBlocked, status.RecoveryState)
	require.Equal(t, []string{CodeChannelGrantMissing}, status.ErrorCategories)
}

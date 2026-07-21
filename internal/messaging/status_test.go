package messaging

import (
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
	require.Equal(t, ReasonCapabilityReady, status.SwitchReason)
	require.Equal(t, RecoverySteady, status.RecoveryState)
	require.Empty(t, status.ReducedCapabilities)
	require.Empty(t, status.ErrorCategories)
}

func TestStatusSnapshotReportsSafeCapabilityFailureOnly(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	resolver := &channelResolver{resolved: fixture.senderResolved, err: channelFailure{"privacy.capability.revoked"}}
	channel := newTestChannel(t, resolver, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xf3)

	status := Snapshot(channel, channel)

	require.Equal(t, StatusDegraded, status.State)
	require.Equal(t, "privacy.capability.revoked", status.SwitchReason)
	require.Equal(t, RecoveryPending, status.RecoveryState)
	require.ElementsMatch(t, []string{"private_publication", "private_discovery", "private_data_exchange"}, status.ReducedCapabilities)
	require.Equal(t, []string{"privacy.capability.revoked"}, status.ErrorCategories)
}

func TestStatusSnapshotReturnsToSteadyAfterCapabilityRecovery(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	resolver := &channelResolver{resolved: fixture.senderResolved, err: channelFailure{"privacy.capability.expired"}}
	channel := newTestChannel(t, resolver, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xf5)
	require.Equal(t, RecoveryPending, Snapshot(channel, channel).RecoveryState)

	resolver.err = nil
	recovered := Snapshot(channel, channel)

	require.Equal(t, StatusActive, recovered.State)
	require.Equal(t, RecoverySteady, recovered.RecoveryState)
	require.Equal(t, ReasonCapabilityReady, recovered.SwitchReason)
	require.Empty(t, recovered.ErrorCategories)
}

func TestStatusSnapshotDoesNotExposeUnexpectedResolverError(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	resolver := &channelResolver{resolved: fixture.senderResolved, err: errors.New("secret path C:/sensitive/capability.db")}
	channel := newTestChannel(t, resolver, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xf4)

	status := Snapshot(channel, channel)

	require.Equal(t, CodeCapabilityUnavailable, status.SwitchReason)
	require.NotContains(t, status.SwitchReason, "sensitive")
	require.Equal(t, []string{CodeCapabilityUnavailable}, status.ErrorCategories)
}

func TestStatusSnapshotRejectsUnrecognizedCapabilityCode(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	resolver := &channelResolver{resolved: fixture.senderResolved, err: channelFailure{"privacy.capability.secret_path"}}
	channel := newTestChannel(t, resolver, fixture.receiverAuthority, fixture.senderResolved, fixture.senderPrivate, 0xf6)

	status := Snapshot(channel, channel)

	require.Equal(t, CodeCapabilityUnavailable, status.SwitchReason)
	require.Equal(t, []string{CodeCapabilityUnavailable}, status.ErrorCategories)
}

func TestStatusSnapshotReportsMissingConfigurationAsBlocked(t *testing.T) {
	status := Snapshot(nil, nil)

	require.Equal(t, StatusDegraded, status.State)
	require.Equal(t, CodeCapabilityMissing, status.SwitchReason)
	require.Equal(t, RecoveryBlocked, status.RecoveryState)
	require.Equal(t, []string{CodeCapabilityMissing}, status.ErrorCategories)
}

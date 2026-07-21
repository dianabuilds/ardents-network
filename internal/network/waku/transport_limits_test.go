package waku

import (
	"ardents/internal/network"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNetworkOperationRejectsOversizedMessageAndReportsIt(t *testing.T) {
	svc := New(network.Config{Limits: network.Limits{MaxMessageBytes: 1024}})

	_, err := svc.acquireNetworkOperation(1025, "")

	require.ErrorContains(t, err, "exceeds 1024 byte limit")
	require.Equal(t, uint64(1), svc.AbuseSnapshot().OversizedMessages)
}

func TestNetworkOperationAppliesConcurrencyBackpressure(t *testing.T) {
	svc := New(network.Config{Limits: network.Limits{
		MaxConcurrentOperations: 1,
		OperationRate:           1000,
		OperationBurst:          1000,
	}})
	done, err := svc.acquireNetworkOperation(0, "")
	require.NoError(t, err)

	_, err = svc.acquireNetworkOperation(0, "")

	require.ErrorContains(t, err, "concurrency limit exceeded")
	require.Equal(t, uint64(1), svc.AbuseSnapshot().BackpressuredOperations)
	done(nil)
}

func TestNetworkOperationAppliesRateLimit(t *testing.T) {
	svc := New(network.Config{Limits: network.Limits{OperationRate: 1, OperationBurst: 1}})
	done, err := svc.acquireNetworkOperation(0, "")
	require.NoError(t, err)
	done(nil)

	_, err = svc.acquireNetworkOperation(0, "")

	require.ErrorContains(t, err, "rate limit exceeded")
	require.Equal(t, uint64(1), svc.AbuseSnapshot().RateLimitedOperations)
}

func TestProviderPenaltyTemporarilyBansAndRecovers(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	previous := timeNowUTC
	timeNowUTC = func() time.Time { return now }
	t.Cleanup(func() { timeNowUTC = previous })
	svc := New(network.Config{Limits: network.Limits{OperationRate: 1000, OperationBurst: 1000}})

	for range providerFailureThreshold {
		done, err := svc.acquireNetworkOperation(0, "provider-a")
		require.NoError(t, err)
		done(errors.New("provider failure"))
	}

	require.Equal(t, 1, svc.AbuseSnapshot().BannedProviders)
	_, err := svc.acquireNetworkOperation(0, "provider-a")
	require.ErrorContains(t, err, "temporarily banned")

	now = now.Add(providerBanDuration + time.Second)
	done, err := svc.acquireNetworkOperation(0, "provider-a")
	require.NoError(t, err)
	done(nil)
	require.Zero(t, svc.AbuseSnapshot().BannedProviders)
}

func TestValidateLimitsRejectsNegativeAndUnsafeValues(t *testing.T) {
	require.ErrorContains(t, validateLimits(network.Limits{OperationRate: -1}), "cannot be negative")
	require.ErrorContains(t, validateLimits(network.Limits{MaxMessageBytes: 200 * 1024}), "maximum message")
	require.ErrorContains(t, validateLimits(network.Limits{MaxPeerConnections: 4, MaxConnectionsPerIP: 5}), "connection limits")
}

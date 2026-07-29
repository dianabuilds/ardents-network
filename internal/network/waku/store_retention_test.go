package waku

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ardents/internal/network"

	"github.com/stretchr/testify/require"
	"github.com/waku-org/go-waku/waku/v2/protocol"
	wpb "github.com/waku-org/go-waku/waku/v2/protocol/pb"
	"github.com/waku-org/go-waku/waku/v2/timesource"
	"google.golang.org/protobuf/proto"
)

const testStoreMaxBytes int64 = 8 << 20

func TestMessageProviderRetentionBoundsGrowthAndSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waku-store.db")
	retention := network.StoreRetention{MaxMessages: 3, MaxAge: time.Hour, MaxBytes: testStoreMaxBytes}
	provider, err := NewMessageProvider(path, retention)
	require.NoError(t, err)
	require.NoError(t, provider.Start(context.Background(), timesource.NewDefaultClock()))

	now := time.Now()
	for i := 0; i < 8; i++ {
		storedAt := now.Add(time.Duration(i) * time.Millisecond).UnixNano()
		message := &wpb.WakuMessage{
			Payload:      []byte{byte(i)},
			ContentTopic: "retention/test",
			Timestamp:    proto.Int64(storedAt),
		}
		require.NoError(t, provider.Put(protocol.NewEnvelope(message, storedAt, network.DefaultPubsubTopic)))
		count, countErr := provider.Count()
		require.NoError(t, countErr)
		require.LessOrEqual(t, count, retention.MaxMessages)
	}
	provider.Stop()

	provider, err = NewMessageProvider(path, retention)
	require.NoError(t, err)
	require.NoError(t, provider.Start(context.Background(), timesource.NewDefaultClock()))
	t.Cleanup(provider.Stop)

	count, err := provider.Count()
	require.NoError(t, err)
	require.Equal(t, retention.MaxMessages, count)
}

func TestMessageProviderRetentionRemovesExpiredMessagesOnRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waku-store.db")
	retention := network.StoreRetention{MaxMessages: 10, MaxAge: time.Second, MaxBytes: testStoreMaxBytes}
	provider, err := NewMessageProvider(path, retention)
	require.NoError(t, err)
	require.NoError(t, provider.Start(context.Background(), timesource.NewDefaultClock()))

	now := time.Now().UnixNano()
	message := &wpb.WakuMessage{Payload: []byte("retained"), ContentTopic: "retention/age", Timestamp: proto.Int64(now)}
	require.NoError(t, provider.Put(protocol.NewEnvelope(message, now, network.DefaultPubsubTopic)))
	provider.Stop()
	time.Sleep(1100 * time.Millisecond)

	provider, err = NewMessageProvider(path, retention)
	require.NoError(t, err)
	require.NoError(t, provider.Start(context.Background(), timesource.NewDefaultClock()))
	t.Cleanup(provider.Stop)

	items, err := provider.GetAll()
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestMessageProviderNormalizesUntrustedFutureTimestamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waku-store.db")
	retention := network.StoreRetention{MaxMessages: 10, MaxAge: time.Hour, MaxBytes: testStoreMaxBytes}
	provider, err := NewMessageProvider(path, retention)
	require.NoError(t, err)
	require.NoError(t, provider.Start(context.Background(), timesource.NewDefaultClock()))
	t.Cleanup(provider.Stop)

	future := time.Now().Add(365 * 24 * time.Hour).UnixNano()
	message := &wpb.WakuMessage{Payload: []byte("future"), ContentTopic: "retention/future", Timestamp: proto.Int64(future)}
	require.NoError(t, provider.Put(protocol.NewEnvelope(message, future, network.DefaultPubsubTopic)))
	items, err := provider.GetAll()
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Less(t, items[0].ReceiverTime, time.Now().Add(time.Minute).UnixNano())
}

func TestMessageProviderHardByteQuotaIncludesWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waku-store.db")
	const quota int64 = 4 << 20
	svc := New(network.Config{
		NodeProfile: network.NodeProfileServiceNode,
		StorePath:   path,
		Limits: network.Limits{
			StoreMaxMessages: 1000, StoreMaxAgeSeconds: 3600, StoreMaxBytes: quota,
		},
	})
	provider, _, err := svc.prepareMessageProviderLocked()
	require.NoError(t, err)
	require.NoError(t, provider.Start(context.Background(), timesource.NewDefaultClock()))
	t.Cleanup(provider.Stop)

	var quotaErr error
	for i := 0; i < 64; i++ {
		now := time.Now().UnixNano() + int64(i)
		message := &wpb.WakuMessage{
			Payload: bytes.Repeat([]byte{byte(i)}, 128<<10), ContentTopic: "retention/bytes", Timestamp: proto.Int64(now),
		}
		quotaErr = provider.Put(protocol.NewEnvelope(message, now, network.DefaultPubsubTopic))
		if quotaErr != nil {
			break
		}
	}
	require.Error(t, quotaErr)
	used, err := storeDiskBytes(path)
	require.NoError(t, err)
	require.LessOrEqual(t, used, quota)
	pressure := svc.AbuseSnapshot()
	require.Equal(t, "degraded", pressure.State)
	require.Equal(t, "degraded", pressure.StoreState)
	require.GreaterOrEqual(t, pressure.StoreUsageRatio, storePressureDegradedRatio)
	svc.state = "ready"
	require.Equal(t, network.HealthStateDegraded, svc.ProfileSnapshot().Health)
}

func TestStoreDiskBytesRequiresMainDatabaseAndIncludesSidecars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waku-store.db")
	_, err := storeDiskBytes(path)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte{1}, 10), 0o600))
	require.NoError(t, os.WriteFile(path+"-wal", bytes.Repeat([]byte{1}, 20), 0o600))
	require.NoError(t, os.WriteFile(path+"-shm", bytes.Repeat([]byte{1}, 30), 0o600))
	used, err := storeDiskBytes(path)
	require.NoError(t, err)
	require.Equal(t, int64(60), used)
}

func TestMessageProviderRejectsUnboundedRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waku-store.db")
	_, err := NewMessageProvider(path, network.StoreRetention{})
	require.ErrorContains(t, err, "finite and positive")
	require.NoFileExists(t, path)
}

func TestServiceNodeRejectsExplicitlyInvalidStoreRetentionBeforeStartup(t *testing.T) {
	svc := New(network.Config{
		NodeProfile: network.NodeProfileServiceNode,
		Limits: network.Limits{
			StoreMaxMessages:   -1,
			StoreMaxAgeSeconds: 3600,
			StoreMaxBytes:      testStoreMaxBytes,
		},
	})
	require.ErrorContains(t, svc.Start(context.Background()), "cannot be negative")
	require.Equal(t, "failed", svc.State())
}

func TestStorePressureIsObservableAndProfileAware(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waku-store.db")
	svc := New(network.Config{
		NodeProfile: network.NodeProfileServiceNode,
		StorePath:   path,
		Limits: network.Limits{
			StoreMaxMessages:   10,
			StoreMaxAgeSeconds: 3600,
		},
	})
	provider, _, err := svc.prepareMessageProviderLocked()
	require.NoError(t, err)
	require.NoError(t, provider.Start(context.Background(), timesource.NewDefaultClock()))
	t.Cleanup(provider.Stop)

	now := time.Now().UnixNano()
	for i := 0; i < 9; i++ {
		message := &wpb.WakuMessage{
			Payload:      []byte{byte(i)},
			ContentTopic: "pressure/test",
			Timestamp:    proto.Int64(now + int64(i)),
		}
		require.NoError(t, provider.Put(protocol.NewEnvelope(message, now+int64(i), network.DefaultPubsubTopic)))
	}

	snapshot := svc.AbuseSnapshot()
	require.Equal(t, "degraded", snapshot.State)
	require.Equal(t, 9, snapshot.StoreMessages)
	require.Equal(t, 10, snapshot.StoreCapacityMessages)
	require.Greater(t, snapshot.StoreFileBytes, int64(0))
	svc.state = "ready"
	require.Equal(t, "degraded", svc.State())
	require.Equal(t, network.HealthStateDegraded, svc.ProfileSnapshot().Health)

	constrained := New(network.Config{NodeProfile: network.NodeProfileConstrainedClient})
	constrainedSnapshot := constrained.AbuseSnapshot()
	require.False(t, constrainedSnapshot.StoreEnabled)
	require.Equal(t, "ready", constrainedSnapshot.State)

	svc.activeMode = network.ModeRestrictedDefense
	restrictedProvider, _, err := svc.prepareMessageProviderLocked()
	require.NoError(t, err)
	require.Nil(t, restrictedProvider)
}

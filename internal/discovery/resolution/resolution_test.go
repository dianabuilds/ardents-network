package resolution

import (
	"testing"
	"time"

	discoveryrecord "ardents/internal/discovery/records"
	"github.com/stretchr/testify/require"
)

func TestResolveReturnsWithdrawnOutcome(t *testing.T) {
	now := time.Now().UTC()
	entry := discoveryrecord.Entry{Record: serviceRecord("svc.echo", "echo", nil, now.Add(time.Hour))}
	got, outcome, found := Resolve([]discoveryrecord.Entry{entry}, "svc.echo", "service", now)
	require.True(t, found)
	require.Equal(t, "withdrawn", outcome)
	require.Equal(t, entry.Record.RecordID(), got.Record.RecordID())
}

func TestFindServiceSkipsExpiredEntries(t *testing.T) {
	now := time.Now().UTC()
	entries := []discoveryrecord.Entry{
		{Record: serviceRecord("svc.echo.fresh", "echo", []string{"tcp://fresh"}, now.Add(time.Hour))},
		{Record: serviceRecord("svc.echo.expired", "echo", []string{"tcp://expired"}, now.Add(-time.Minute))},
	}
	got := FindService(entries, "echo", now)
	require.Len(t, got, 1)
	require.Equal(t, "svc.echo.fresh", got[0].Record.RecordID())
}

func TestFindServiceBoundedFailsClosedBeforeReturningOversizedTruth(t *testing.T) {
	now := time.Now().UTC()
	t.Run("catalog", func(t *testing.T) {
		entries := []discoveryrecord.Entry{
			{Record: serviceRecord("svc.echo.a", "echo", []string{"tcp://a"}, now.Add(time.Hour))},
			{Record: serviceRecord("svc.echo.b", "echo", []string{"tcp://b"}, now.Add(time.Hour))},
		}

		got, overflow := FindServiceBounded(entries, "echo", now, 1, 8)

		require.True(t, overflow)
		require.Nil(t, got)
	})
	t.Run("matching endpoints", func(t *testing.T) {
		entries := []discoveryrecord.Entry{
			{Record: serviceRecord(
				"svc.echo", "echo", []string{"tcp://a", "tcp://b"}, now.Add(time.Hour),
			)},
		}

		got, overflow := FindServiceBounded(entries, "echo", now, 8, 1)

		require.True(t, overflow)
		require.Nil(t, got)
	})
}

func TestResolutionUsesExactValidityBoundaries(t *testing.T) {
	now := time.Now().UTC()
	atExpiry := serviceRecord("svc.expiry", "echo", []string{"tcp://expiry"}, now)
	future := serviceRecord("svc.future", "echo", []string{"tcp://future"}, now.Add(time.Hour))
	future.IssuedAt = now.Add(time.Nanosecond)

	_, outcome, found := Resolve([]discoveryrecord.Entry{{Record: atExpiry}}, atExpiry.Subject(), atExpiry.Kind(), now)
	require.True(t, found)
	require.Equal(t, "expired", outcome)
	require.Empty(t, FindService([]discoveryrecord.Entry{{Record: atExpiry}, {Record: future}}, "echo", now))
	require.Zero(t, Count([]discoveryrecord.Entry{{Record: atExpiry}, {Record: future}}, "service", now))
}

func TestCountSkipsWithdrawnServices(t *testing.T) {
	now := time.Now().UTC()
	entries := []discoveryrecord.Entry{
		{Record: serviceRecord("svc.withdrawn", "echo", nil, now.Add(time.Hour))},
		{Record: serviceRecord("svc.live", "echo", []string{"tcp://live"}, now.Add(time.Hour))},
		{Record: nodeRecord([]string{"tcp://node"}, now.Add(time.Hour))},
		{Record: nodeRecord([]string{"tcp://expired"}, now.Add(-time.Minute))},
	}
	require.Equal(t, 1, Count(entries, "service", now))
	require.Equal(t, 2, Count(entries, "", now))
}

func serviceRecord(id, serviceType string, endpoints []string, expires time.Time) discoveryrecord.Record {
	return discoveryrecord.Record{Version: discoveryrecord.Version, Service: &discoveryrecord.ServiceFacts{ID: discoveryrecord.ServiceID(id), Type: serviceType, Endpoints: endpoints}, ExpiresAt: expires}
}

func nodeRecord(endpoints []string, expires time.Time) discoveryrecord.Record {
	return discoveryrecord.Record{Version: discoveryrecord.Version, Node: &discoveryrecord.NodeFacts{Endpoints: endpoints}, ExpiresAt: expires}
}

package resolution

import (
	"testing"
	"time"

	discoveryrecord "ardents/internal/discovery/record"

	"github.com/stretchr/testify/require"
)

func TestResolveReturnsWithdrawnOutcome(t *testing.T) {
	now := time.Now().UTC()
	entry := discoveryrecord.Entry{
		Record: discoveryrecord.Record{
			ID:        "svc.echo",
			Kind:      "service",
			Subject:   "svc.echo",
			Service:   "echo",
			Endpoints: nil,
			ExpiresAt: now.Add(time.Hour),
		},
	}
	got, outcome, found := Resolve([]discoveryrecord.Entry{entry}, "svc.echo", "service", now)
	require.Falsef(t, !found || outcome !=
		"withdrawn", "found=%v outcome=%q, want withdrawn", found, outcome)
	require.Falsef(t, got.Record.ID != entry.
		Record.
		ID, "resolved id = %q, want %q", got.Record.ID, entry.Record.ID)

}

func TestFindServiceSkipsExpiredEntries(t *testing.T) {
	now := time.Now().UTC()
	entries := []discoveryrecord.Entry{
		{Record: discoveryrecord.Record{
			ID:        "svc.echo.fresh",
			Kind:      "service",
			Subject:   "svc.echo.fresh",
			Service:   "echo",
			Endpoints: []string{"tcp://fresh"},
			ExpiresAt: now.Add(time.Hour),
		}},
		{Record: discoveryrecord.Record{
			ID:        "svc.echo.expired",
			Kind:      "service",
			Subject:   "svc.echo.expired",
			Service:   "echo",
			Endpoints: []string{"tcp://expired"},
			ExpiresAt: now.Add(-time.Minute),
		}},
	}
	got := FindService(entries, "echo", now)
	require.Falsef(t, len(got) != 1 || got[0].Record.
		ID != "svc.echo.fresh", "services = %#v, want only fresh entry", got)

}

func TestCountSkipsWithdrawnServices(t *testing.T) {
	now := time.Now().UTC()
	entries := []discoveryrecord.Entry{
		{Record: discoveryrecord.Record{Kind: "service", Endpoints: nil, ExpiresAt: now.Add(time.Hour)}},
		{Record: discoveryrecord.Record{Kind: "service", Endpoints: []string{"tcp://live"}, ExpiresAt: now.Add(time.Hour)}},
		{Record: discoveryrecord.Record{Kind: "node", Endpoints: []string{"tcp://node"}, ExpiresAt: now.Add(time.Hour)}},
		{Record: discoveryrecord.Record{Kind: "node", Endpoints: []string{"tcp://expired"}, ExpiresAt: now.Add(-time.Minute)}},
	}
	{
		got := Count(entries, "service", now)
		require.Falsef(t, got != 1, "count = %d, want 1", got)
	}
	{

		got := Count(entries, "", now)
		require.Falsef(t, got != 2, "count = %d, want 2 visible entries", got)
	}

}

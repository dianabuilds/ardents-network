package publication

import (
	"context"
	"errors"
	"time"

	discovery "ardents/internal/discovery"
	discoverysource "ardents/internal/discovery/source"
	transport "ardents/internal/network/api"
)

func requiresNetworkCompensation(err error) bool {
	var publishErr *transport.DiscoveryPublishError
	return errors.As(err, &publishErr) && publishErr.Published > 0
}

func (m *Manager) compensateNetworkLocked(ctx context.Context, entries []discovery.Entry) error {
	if len(entries) == 0 {
		return nil
	}
	return m.publishDiscoveryEntries(ctx, discoverysource.RefreshSeenAt(discoverysource.LocalEntries(entries), time.Now().UTC()))
}

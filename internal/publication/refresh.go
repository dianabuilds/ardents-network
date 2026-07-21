package publication

import (
	"context"
	"errors"
	"time"

	"ardents/internal/discovery"
	"ardents/internal/discovery/records"
	"ardents/internal/network"
)

func requiresNetworkCompensation(err error) bool {
	publishErr, ok := errors.AsType[*network.DiscoveryPublishError](err)
	return ok && publishErr.Published > 0
}

func (m *Manager) compensateNetworkLocked(ctx context.Context, entries []discovery.Entry) error {
	if len(entries) == 0 {
		return nil
	}
	return m.publishDiscoveryEntries(ctx, records.RefreshSeenAt(records.LocalEntries(entries), time.Now().UTC()))
}

func (m *Manager) RefreshNetworkPublicationLocked(ctx context.Context) error {
	id := m.ident.NodeSummary()
	private := m.privateKey()
	if id.Principal == "" || len(private) == 0 {
		return nil
	}
	if err := PublishLocalNode(m.disco, id, private, m.trans.Endpoints()); err != nil {
		return err
	}
	if entry, _, ok := m.disco.Resolve(id.Principal, "node"); ok {
		m.trust.Remember(m.trust.Evaluate(entry.Record))
	}
	return m.SyncDesiredLocked(ctx)
}

func (m *Manager) WithdrawNetworkPublicationLocked(ctx context.Context) error {
	id := m.ident.NodeSummary()
	private := m.privateKey()
	if id.Principal == "" || len(private) == 0 {
		return nil
	}
	previousEntries := m.localDiscoveryEntriesLocked()
	if err := WithdrawLocalNode(m.disco, id, private); err != nil {
		return err
	}
	if m.publicationAttempted && !m.networkPublished {
		return nil
	}
	if err := m.publishDiscoveryEntriesWithCompensationLocked(ctx, previousEntries); err != nil {
		return err
	}
	m.networkPublished = false
	m.publicationAttempted = true
	return nil
}

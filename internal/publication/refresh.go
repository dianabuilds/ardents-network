package publication

import "context"

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

package process

import "context"

func (n *Node) Start(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := ValidateConfig(n.cfg); err != nil {
		return err
	}
	if n.cancel != nil {
		return nil
	}
	startBlobExchange := n.authorityCtl.StartBlobExchangeLocked
	if n.startBlobExchange != nil {
		startBlobExchange = n.startBlobExchange
	}
	networkCtx, cancel, err := n.commandService.StartLocked(ctx, startBlobExchange)
	if err != nil {
		return err
	}
	n.network = networkCtx
	n.cancel = cancel
	n.restartDiscoveryRefreshLocked()
	return nil
}

func (n *Node) Stop(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.refreshStop != nil {
		n.refreshStop()
		n.refreshStop = nil
	}
	err := n.commandService.StopLocked(ctx, n.cancel)
	n.cancel = nil
	n.network = nil
	return err
}

func (n *Node) restartDiscoveryRefreshLocked() {
	if n.refreshStop != nil {
		n.refreshStop()
		n.refreshStop = nil
	}
	if n.network == nil {
		return
	}
	refreshCtx, cancel := context.WithCancel(n.network)
	n.refreshStop = cancel
	n.commandService.StartDiscoveryRefreshLoop(refreshCtx, n.cfg.DiscoveryRefreshInterval, n.refreshDiscoveryPublication)
}

func (n *Node) refreshDiscoveryPublication(ctx context.Context) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.cancel == nil {
		return
	}
	n.runtimeMgr.RefreshDiscoveryPublicationLocked(ctx)
}

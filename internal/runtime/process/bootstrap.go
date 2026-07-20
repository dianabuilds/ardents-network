package process

import (
	transport "ardents/internal/network/api"
	noderecovery "ardents/internal/node/recovery"
)

func (n *Node) handleBootstrapDialLocked(report transport.BootstrapDialReport) {
	n.mu.Lock()
	defer n.mu.Unlock()
	noderecovery.RecordBootstrapDial(n.diag, n.cfg.Name, report)
}

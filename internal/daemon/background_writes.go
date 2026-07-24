package daemon

func (n *Node) startBackgroundWrites() {
	n.backgroundMu.Lock()
	defer n.backgroundMu.Unlock()
	if !n.backgroundStopping {
		return
	}
	n.backgroundStopping = false
	n.backgroundStop = make(chan struct{})
}

func (n *Node) runOwnedBackgroundWrite(write func()) {
	n.backgroundMu.Lock()
	if n.backgroundStopping {
		n.backgroundMu.Unlock()
		return
	}
	n.backgroundWriters.Add(1)
	n.backgroundMu.Unlock()

	defer n.backgroundWriters.Done()
	write()
}

func (n *Node) stopBackgroundWrites() {
	n.backgroundMu.Lock()
	if !n.backgroundStopping {
		n.backgroundStopping = true
		close(n.backgroundStop)
	}
	n.backgroundMu.Unlock()
	n.backgroundWriters.Wait()
}

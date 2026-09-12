package route

// Close retires every child and wakes blocked lane readers without sending
// frames. The transport owner must interrupt its connection before calling
// Close, so an in-flight writer cannot hold a lane lock indefinitely.
// It then joins the child handlers before closing their handshake owner;
// queued-byte and child reservations remain held through that cleanup.
func (bridge *ClosedOuterBridge) Close() {
	if bridge == nil {
		return
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.closed {
		return
	}
	bridge.closed = true
	for id, lane := range bridge.lanes {
		lane.closeInput()
		delete(bridge.lanes, id)
	}
}

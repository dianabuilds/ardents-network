//go:build linux

package route

import "sync"

// BeginTerminalWrite registers one terminal write scheduling pass on this
// lane's inner stream and returns the function that releases the
// registration exactly once. The outgoing client terminal-priority switch
// calls it when an authenticated inner terminal rides an outer bridge lane;
// the lane's own write scheduler keeps prioritizing registered terminals.
func (lane *ClosedOuterBridgeLane) BeginTerminalWrite() func() {
	inner := lane.lane
	inner.mu.Lock()
	inner.terminalWriters++
	inner.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			inner.mu.Lock()
			inner.terminalWriters--
			inner.mu.Unlock()
		})
	}
}

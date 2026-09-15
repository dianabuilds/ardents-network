//go:build linux

package route

// All nested JOIN queues debit the same Source/Responder parent. Empty
// Connections reserve bounded metadata, not eagerly allocated receive windows.
func (owner *closedSourceChannels) reserveQueuedLocked(size uint64, control bool) bool {
	limit := uint64(4 << 20)
	if !control {
		limit -= 16 << 10
	}
	if size > limit || owner.queued > limit-size {
		return false
	}
	if parent := owner.queueParent; parent != nil {
		parent.mu.Lock()
		defer parent.mu.Unlock()
		if parent.terminal != nil || parent.queued > limit-size {
			return false
		}
		parent.queued += size
	}
	owner.queued += size
	return true
}

func (owner *closedSourceChannels) releaseQueuedLocked(size uint64) {
	owner.queued -= size
	if parent := owner.queueParent; parent != nil {
		parent.mu.Lock()
		parent.queued -= size
		parent.signalLocked()
		parent.mu.Unlock()
	}
}

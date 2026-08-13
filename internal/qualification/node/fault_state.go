package node

import "time"

type nodeResourceReset struct {
	at     time.Time
	faults map[string]bool
}

func (observer *nodeObserver) setFault(name string, active bool) {
	observer.mu.Lock()
	observer.activeFaults[name] = active
	faults := copyNodeFaults(observer.activeFaults)
	observer.mu.Unlock()
	observer.notifyResourceReset(faults)
}

func (observer *nodeObserver) notifyResourceReset(faults map[string]bool) {
	if observer.resourceReset == nil {
		return
	}
	if observer.ctx == nil {
		observer.resourceReset <- nodeResourceReset{at: time.Now().UTC(), faults: faults}
		return
	}
	select {
	case observer.resourceReset <- nodeResourceReset{at: time.Now().UTC(), faults: faults}:
	case <-observer.ctx.Done():
	}
}

func (observer *nodeObserver) faultSnapshot() map[string]bool {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return copyNodeFaults(observer.activeFaults)
}

func copyNodeFaults(source map[string]bool) map[string]bool {
	result := make(map[string]bool, len(source))
	for name, active := range source {
		if active {
			result[name] = true
		}
	}
	return result
}

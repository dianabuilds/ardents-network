package node

import "sync"

// closedHostingLifetime keeps the shared Hosting handle available until an
// unfinished duty has joined every child that can still release a reservation.
type closedHostingLifetime struct {
	host     closedForwardingHost
	mu       sync.Mutex
	closed   bool
	handed   bool
	done     chan struct{}
	closeErr error
}

func newClosedHostingLifetime(host closedForwardingHost) *closedHostingLifetime {
	if host == nil {
		return nil
	}
	return &closedHostingLifetime{host: host, done: make(chan struct{})}
}

func (lifetime *closedHostingLifetime) close() error {
	if lifetime == nil {
		return nil
	}
	lifetime.mu.Lock()
	if lifetime.handed {
		lifetime.mu.Unlock()
		return nil
	}
	return lifetime.closeLocked()
}

func (lifetime *closedHostingLifetime) deferCloseUntil(joined <-chan struct{}) bool {
	if lifetime == nil || joined == nil {
		return false
	}
	lifetime.mu.Lock()
	if lifetime.closed || lifetime.handed {
		lifetime.mu.Unlock()
		return false
	}
	lifetime.handed = true
	lifetime.mu.Unlock()
	go func() {
		<-joined
		lifetime.mu.Lock()
		_ = lifetime.closeLocked()
	}()
	return true
}

func (lifetime *closedHostingLifetime) closeLocked() error {
	if lifetime.closed {
		done := lifetime.done
		lifetime.mu.Unlock()
		<-done
		lifetime.mu.Lock()
		err := lifetime.closeErr
		lifetime.mu.Unlock()
		return err
	}
	lifetime.closed = true
	host := lifetime.host
	lifetime.mu.Unlock()
	err := host.Close()
	lifetime.mu.Lock()
	lifetime.closeErr = err
	close(lifetime.done)
	lifetime.mu.Unlock()
	return err
}

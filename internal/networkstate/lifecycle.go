package networkstate

import (
	"context"
	"errors"
)

// Wait reports source-server termination or returns after ctx cancellation.
func (s *store) Wait(ctx context.Context) error {
	s.mu.RLock()
	done := s.serverDone
	s.mu.RUnlock()
	if done == nil {
		return errors.New("network state source server is not configured")
	}
	select {
	case <-ctx.Done():
		return nil
	case <-done:
		s.mu.RLock()
		err := s.serverErr
		s.mu.RUnlock()
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
}

// Close prevents further work through this Store and releases its root lease.
func (s *store) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	done, workCancel, lease := s.serverDone, s.workCancel, s.lease
	s.mu.Unlock()
	if workCancel != nil {
		workCancel()
	}
	s.work.Wait()
	if done != nil {
		<-done
	}
	s.mu.RLock()
	serverErr := s.serverErr
	s.mu.RUnlock()
	leaseErr := lease.release()
	if serverErr != nil && !errors.Is(serverErr, context.Canceled) {
		return serverErr
	}
	return leaseErr
}

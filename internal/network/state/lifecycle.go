package state

import (
	"context"
	"errors"
	"time"
)

// Wait reports terminal background-work failure or returns after ctx cancellation.
func (s *networkState) Wait(ctx context.Context) error {
	s.mu.RLock()
	done, automatic := s.serverDone, s.config.automatic
	s.mu.RUnlock()
	if done == nil {
		if automatic == 0 {
			return errors.New("network state has no background work")
		}
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if _, err := s.Current(); err != nil {
					return err
				}
			}
		}
	}
	select {
	case <-ctx.Done():
		return nil
	case <-done:
		s.mu.RLock()
		err := errors.Join(s.serverErr, s.resourceErr)
		s.mu.RUnlock()
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
}

// Close prevents further work through this Store and releases its root lease.
func (s *networkState) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	done, workCancel, storage := s.serverDone, s.workCancel, s.storage
	s.mu.Unlock()
	if workCancel != nil {
		workCancel()
	}
	s.work.Wait()
	if done != nil {
		<-done
	}
	s.mu.RLock()
	serverErr := errors.Join(s.serverErr, s.resourceErr)
	s.mu.RUnlock()
	storageErr := storage.Close()
	if serverErr != nil && !errors.Is(serverErr, context.Canceled) {
		return serverErr
	}
	return storageErr
}

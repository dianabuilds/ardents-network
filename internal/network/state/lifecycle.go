package state

import (
	"context"
	"errors"
)

// Wait reports terminal background-work failure or returns after ctx cancellation.
func (s *networkState) Wait(ctx context.Context) error {
	s.mu.RLock()
	serverDone, automaticDone, automatic := s.serverDone, s.automaticDone, s.config.automatic
	s.mu.RUnlock()
	if serverDone == nil && automatic == 0 {
		return errors.New("network state has no background work")
	}
	for serverDone != nil || automaticDone != nil {
		select {
		case <-ctx.Done():
			return nil
		case <-serverDone:
			serverDone = nil
		case <-automaticDone:
			automaticDone = nil
		}
		s.mu.RLock()
		err := errors.Join(s.serverErr, s.automaticErr, s.resourceErr)
		s.mu.RUnlock()
		if errors.Is(err, context.Canceled) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
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
	storageErr := storage.close()
	roleErr := s.releaseSourceServer()
	if serverErr != nil && !errors.Is(serverErr, context.Canceled) {
		return serverErr
	}
	return errors.Join(storageErr, roleErr)
}

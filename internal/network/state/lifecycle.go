package state

import (
	"context"
	"errors"
)

// Wait reports terminal owner failure or returns after ctx cancellation.
func (s *networkState) Wait(ctx context.Context) error {
	s.mu.RLock()
	serverDone, automaticDone, resourceDone, terminalErr := s.serverDone, s.automaticDone, s.resourceDone, s.terminalErr
	s.mu.RUnlock()
	if terminalErr != nil {
		return terminalErr
	}
	if serverDone == nil && automaticDone == nil && resourceDone == nil {
		return errors.New("network state has no background work")
	}
	for serverDone != nil || automaticDone != nil || resourceDone != nil {
		select {
		case <-ctx.Done():
			return nil
		case <-serverDone:
			serverDone = nil
		case <-automaticDone:
			automaticDone = nil
		case <-resourceDone:
			resourceDone = nil
		}
		s.mu.RLock()
		serverErr, automaticErr, resourceErr, terminalErr := s.serverErr, s.automaticErr, s.resourceErr, s.terminalErr
		s.mu.RUnlock()
		// A canceled Source listener is the expected result of State shutdown,
		// but it must not hide a terminal failure of another background owner.
		if serverErr == context.Canceled {
			serverErr = nil
		}
		if err := errors.Join(serverErr, automaticErr, resourceErr, terminalErr); err != nil {
			return err
		}
	}
	return nil
}

// Close prevents further work through this Store and releases its root lease.
// Concurrent callers wait for the same cleanup and observe its terminal result.
func (s *networkState) Close() error {
	s.closeOnce.Do(func() { s.closeErr = s.closeOwned() })
	return s.closeErr
}

func (s *networkState) closeOwned() error {
	s.mu.Lock()
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
	serverErr, automaticErr, resourceErr, terminalErr := s.serverErr, s.automaticErr, s.resourceErr, s.terminalErr
	s.mu.RUnlock()
	storageErr := storage.Close()
	roleErr := s.releaseSourceServer()
	if serverErr == context.Canceled {
		serverErr = nil
	}
	return errors.Join(serverErr, automaticErr, resourceErr, terminalErr, storageErr, roleErr)
}

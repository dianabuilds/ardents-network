package state

import (
	"context"
	"errors"
)

// Wait reports terminal owner failure or returns after ctx cancellation.
func (s *State) Wait(ctx context.Context) error {
	s.mu.RLock()
	serverDone, automaticDone, terminalErr := s.serverDone, s.automaticDone, s.terminalErr
	s.mu.RUnlock()
	if terminalErr != nil {
		return terminalErr
	}
	if serverDone == nil && automaticDone == nil {
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
		serverErr, automaticErr, terminalErr := s.serverErr, s.automaticErr, s.terminalErr
		s.mu.RUnlock()
		// A canceled Source listener is the expected result of State shutdown,
		// but it must not hide a terminal failure of another background owner.
		if serverErr == context.Canceled {
			serverErr = nil
		}
		if err := errors.Join(serverErr, automaticErr, terminalErr); err != nil {
			return err
		}
	}
	return nil
}

// Close prevents further work through this Store and releases its root lease.
// Concurrent callers wait for the same cleanup and observe its terminal result.
func (s *State) Close() error {
	s.closeOnce.Do(func() { s.closeErr = s.closeOwned() })
	return s.closeErr
}

func (s *State) closeOwned() error {
	s.mu.Lock()
	s.closed = true
	s.closing = true
	s.invalidateRuntime()
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
	serverErr, automaticErr, terminalErr := s.serverErr, s.automaticErr, s.terminalErr
	s.mu.RUnlock()
	var exposureErr error
	if terminalErr == nil {
		// A terminal journal permits time-bound release after contacts join.
		// An interrupted active journal retains its live guard for recovery.
		exposureErr = s.releaseSourceWave()
	}
	var roleErr error
	// An explicitly retained command completion keeps the live producer until
	// its original State and command-owned users have all joined successfully.
	s.mu.RLock()
	completion := s.sourceCompletion
	s.mu.RUnlock()
	if completion == nil {
		roleErr = s.releaseSourceServer()
	}
	// Ordinary Close removes its serving producer before releasing State. With
	// retained command completion, Claim instead fences successors until Finish
	// removes that producer. No old mutation follows successful Finish, and a
	// copied Finish cannot remove a later successor's producer.
	storageErr := storage.Close()
	if serverErr == context.Canceled {
		serverErr = nil
	}
	result := errors.Join(serverErr, automaticErr, terminalErr, exposureErr, roleErr, storageErr)
	if completion != nil {
		completion.joined <- result
		close(completion.joined)
	}
	return result
}

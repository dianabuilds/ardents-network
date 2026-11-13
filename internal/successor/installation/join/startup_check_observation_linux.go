package join

import "runtime"

// Each request waits for a complete observation that starts after its own
// admission. Only requests already queued when physical I/O begins share that
// observation; a request arriving during I/O needs the next complete round.
// There is no retained positive result and no change to an observer's checks.
type startupCheck struct{ done chan error }

// Production supplies only this original Startup's observe method. Keeping the
// physical observation separate lets concurrency controls hold an observation
// without fabricating native Startup authority.
func (s *startup) checkObservation(observe func() error) error {
	if err := s.begin(); err != nil {
		return err
	}
	defer s.uses.Done()
	request := &startupCheck{done: make(chan error, 1)}
	s.mu.Lock()
	s.checks = append(s.checks, request)
	if !s.checking {
		s.checking = true
		// Close denies admission before waiting. This admitted request still
		// retains its use, including when Close has already started waiting.
		s.uses.Add(1)
		go s.observeChecks(observe)
	}
	s.mu.Unlock()
	if err := <-request.done; err != nil {
		return err
	}
	return s.local()
}

func (s *startup) observeChecks(observe func() error) {
	defer s.uses.Done()
	for {
		// On the fixed one-processor runtime, let just-resumed consumers run
		// before sealing the next batch. Otherwise the observer closes that
		// batch before they can finish I/O and queue their fresh after-check.
		// This is a scheduling handoff, not a timer or reusable observation:
		// only requests admitted before physical I/O begins join this round.
		runtime.Gosched()
		s.ioMu.Lock()
		s.mu.Lock()
		requests := s.checks
		s.checks = nil
		s.mu.Unlock()
		// The batch is closed before any native read. Neither a later request
		// nor an operation following a previous result may join this round.
		err := s.local()
		if err == nil {
			err = observe()
		}
		if err != nil {
			err = s.record(err)
		} else {
			err = s.local()
		}
		s.ioMu.Unlock()
		for _, request := range requests {
			request.done <- err
		}
		s.mu.Lock()
		if len(s.checks) == 0 {
			s.checking = false
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
	}
}

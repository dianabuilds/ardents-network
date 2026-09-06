package connection

import "time"

// startTerminalTail retains only terminal-control ownership after RunBounded
// has returned successfully. A final receipt confirmation has no further
// record that could prove that its peer received it. The tail therefore keeps
// the existing recovery-capable connection available until its Context or
// Work Safety expires, so that it can replay the settled Terminal and its
// acknowledgement proof after a carrier failure. It never reads or presents
// Application bytes.
func (stream *Stream) startTerminalTail(cleanup func(), receiver <-chan error) bool {
	stream.mu.Lock()
	for stream.applicationWriting && stream.terminal == nil {
		stream.cond.Wait()
	}
	eligible := stream.opener != nil && stream.terminal == nil && stream.localTerminal && stream.remoteTerminal &&
		stream.terminalConfirmationSent
	if eligible {
		stream.postClose = true
	}
	stream.mu.Unlock()
	if !eligible {
		return false
	}
	go stream.runTerminalTail(stream.boundTerminalTail(cleanup), receiver)
	return true
}

// boundTerminalTail applies NoNewRecoveryAfter only after the Application
// outcome is complete. It leaves an active ordinary stream untouched while
// ensuring a tail never retains a carrier after it can no longer recover it.
func (stream *Stream) boundTerminalTail(cleanup func()) func() {
	if stream.recovery.NoNewRecoveryAfter == 0 {
		return cleanup
	}
	remaining := time.Unix(stream.recovery.NoNewRecoveryAfter, 0).Sub(stream.authorizationTime())
	releaseTimer := acquireResource(stream.resources, "timer")
	timer := time.AfterFunc(remaining, func() { stream.fail(errTerminalTailExpired) })
	return func() {
		timer.Stop()
		releaseTimer()
		cleanup()
	}
}

func (stream *Stream) runTerminalTail(cleanup func(), receiver <-chan error) {
	defer func() {
		stream.close()
		cleanup()
		close(stream.done)
	}()

	results := make(chan error, 2)
	go func() { results <- stream.sendBoundedAcknowledgements() }()
	if receiver == nil {
		receiver = stream.startTerminalTailReceive()
	}
	if err := <-receiver; err != nil {
		stream.fail(err)
	} else {
		// The ordinary receiver may have observed completion immediately before
		// postClose was set. Replace it with a receiver that now owns only
		// terminal-control records.
		if err := <-stream.startTerminalTailReceive(); err != nil {
			stream.fail(err)
		}
	}
	<-results
}

func (stream *Stream) startTerminalTailReceive() <-chan error {
	stream.mu.Lock()
	limit := stream.recvNext
	stream.mu.Unlock()
	result := make(chan error, 1)
	go func() {
		err := stream.receiveApplicationBounded(limit)
		if err != nil {
			stream.fail(err)
		}
		result <- err
	}()
	return result
}

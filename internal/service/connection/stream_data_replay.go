package connection

// startRecoveredDataReplay sends the already accepted Data suffix after a
// replacement Attachment commits while the ordinary sender is blocked in local
// Application I/O. It never reads Application bytes or creates a second logical
// send owner: flushAvailable serializes the complete pending range.
func (stream *Stream) startRecoveredDataReplay() {
	stream.mu.Lock()
	replay := stream.terminal == nil && stream.current != nil && stream.sendNext < stream.sendEnd && !stream.dataReplaying
	if replay {
		stream.dataReplaying = true
		stream.dataReplayDone = make(chan struct{})
	}
	done := stream.dataReplayDone
	stream.mu.Unlock()
	if !replay {
		return
	}
	go func() {
		defer close(done)
		err := stream.flushAvailable()
		if err != nil {
			stream.fail(err)
		}
		stream.mu.Lock()
		stream.dataReplaying = false
		stream.cond.Broadcast()
		stream.mu.Unlock()
	}()
}

func (stream *Stream) waitForDataReplay() {
	stream.mu.Lock()
	done := stream.dataReplayDone
	stream.mu.Unlock()
	if done != nil {
		<-done
	}
}

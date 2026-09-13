package route

import "errors"

// Admission reserves control capacity before any channel data can compete for
// the duty queue. These bytes remain inside the 64 MiB ancestor bound, even
// when unused. Adding channels therefore reduces available data capacity.
func (channel *closedDutyChannel) queueControl(size uint64) error {
	if channel == nil || channel.limits == nil || size == 0 {
		return errors.New("closed duty control queue is unavailable")
	}
	channel.limits.mu.Lock()
	defer channel.limits.mu.Unlock()
	if channel.released || size > closedChannelControlBytes-channel.controlQueued {
		return errors.New("closed duty control queue is exhausted")
	}
	channel.controlQueued += size
	return nil
}

func (channel *closedDutyChannel) dequeueControl(size uint64) {
	if channel == nil || channel.limits == nil {
		return
	}
	channel.limits.mu.Lock()
	defer channel.limits.mu.Unlock()
	if size <= channel.controlQueued {
		channel.controlQueued -= size
	}
}

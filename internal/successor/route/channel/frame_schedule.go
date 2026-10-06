package channel

import (
	"errors"
	"net"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// frameTurn accounts queued output before it can contend for the physical
// writer. Data uses the principal budget; control uses the channel's separately
// reserved 16 KiB. A lane contributes at most one serialized payload frame.
type frameTurn struct {
	lane              *Lane
	control, selected bool
	bytes             uint64
	bounded           bool
	ready             chan struct{}
}

// frameExpiry records a deadline refusal before physical output starts. It
// cannot be constructed from a Carrier error or a failed started write.
type frameExpiry struct{ end time.Time }

func (e *frameExpiry) Error() string { return os.ErrDeadlineExceeded.Error() }
func (e *frameExpiry) Unwrap() error { return os.ErrDeadlineExceeded }

// scheduleLocked serves new control first, then one round-robin data frame
// before another control. A credit waiter never enters this queue.
func (s *Session) scheduleLocked() {
	if s.stopped || len(s.output) == 0 || len(s.writer) != 0 {
		return
	}
	control, data, wrap := -1, -1, -1
	for i, t := range s.output {
		if t.control {
			if control < 0 {
				control = i
			}
			continue
		}
		if wrap < 0 || t.lane.id < s.output[wrap].lane.id {
			wrap = i
		}
		if t.lane.id > s.lastData && (data < 0 || t.lane.id < s.output[data].lane.id) {
			data = i
		}
	}
	if data < 0 {
		data = wrap
	}
	chosen := data
	if control >= 0 && (!s.lastControl || data < 0) {
		chosen = control
	}
	if chosen < 0 {
		return
	}
	t := s.output[chosen]
	s.output = append(s.output[:chosen], s.output[chosen+1:]...)
	t.selected = true
	s.lastControl = t.control
	if !t.control {
		s.lastData = t.lane.id
	}
	s.writer <- struct{}{}
	close(t.ready)
}

func (s *Session) turn(l *Lane, f ardp.Frame, terminal bool, bytes uint64) (func(), error) {
	t := &frameTurn{lane: l, control: f.Kind != ardp.KindBytes, bytes: bytes, ready: make(chan struct{})}
	s.mu.Lock()
	if s.stopped || s.finishingRole || (!terminal && l.closed) {
		s.mu.Unlock()
		return nil, net.ErrClosed
	}
	end := l.frameDeadline(f, terminal)
	if !time.Now().Before(end) {
		s.mu.Unlock()
		return nil, &frameExpiry{end: end}
	}
	if l.queueBound != nil && !(terminal && f.Kind == ardp.KindClose && l.queueTermination) {
		if !l.queueBound.reserve(bytes) {
			s.mu.Unlock()
			return nil, errors.New("route child output queue exhausted")
		}
		t.bounded = true
	}
	if t.control {
		if bytes > (16<<10)-s.controlQueued {
			if t.bounded {
				l.queueBound.release(bytes)
			}
			s.mu.Unlock()
			return nil, errors.New("route control queue exhausted")
		}
		s.controlQueued += bytes
	} else {
		if bytes > (4<<20)-s.queued-s.outbound || !s.queues.reserve(bytes) {
			if t.bounded {
				l.queueBound.release(bytes)
			}
			s.mu.Unlock()
			return nil, errors.New("route output queue exhausted")
		}
		s.outbound += bytes
	}
	l.queuedOutput += bytes
	s.output = append(s.output, t)
	s.writes.Add(1)
	l.writes.Add(1)
	s.scheduleLocked()
	s.mu.Unlock()
	finish := func() {
		s.mu.Lock()
		l.queuedOutput -= bytes
		if t.bounded {
			l.queueBound.release(bytes)
		}
		if t.selected {
			<-s.writer
		} else {
			for i, queued := range s.output {
				if queued == t {
					s.output = append(s.output[:i], s.output[i+1:]...)
					break
				}
			}
		}
		if t.control {
			s.controlQueued -= bytes
		} else {
			s.outbound -= bytes
			s.queues.release(bytes)
		}
		s.scheduleLocked()
		s.mu.Unlock()
		l.writes.Done()
		s.writes.Done()
	}
	for {
		s.mu.Lock()
		if t.selected {
			s.mu.Unlock()
			return finish, nil
		}
		end, changed := l.frameDeadline(f, terminal), l.changed
		stopped := s.stopped || (!terminal && l.closed)
		s.mu.Unlock()
		if stopped {
			finish()
			return nil, net.ErrClosed
		}
		if !time.Now().Before(end) {
			finish()
			return nil, &frameExpiry{end: end}
		}
		timer := time.NewTimer(time.Until(end))
		select {
		case <-t.ready:
		case <-changed:
		case <-s.ctx.Done():
		case <-timer.C:
		}
		timer.Stop()
	}
}

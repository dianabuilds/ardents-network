package outer

import (
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// A deadline update can interrupt only its own in-flight frame.
// Queued writes observe their live deadline independently of serialization;
// cancellation removes unemitted work before releasing its caller.
// Partial-frame failure poisons the physical framing boundary and closes it.
type writer struct {
	connection      net.Conn
	closeConnection func() error
	writer          sync.Mutex
	state           sync.Mutex
	active          *writeRequest
	terminals       []*writeRequest
	controls        []*writeRequest
	data            []*writeRequest
	dataDue         bool
	terminalServed  bool
	running         bool
	failure         error
}

type writeRequest struct {
	frame             ardp.Frame
	deadline          func() time.Time
	end               time.Time
	control, terminal bool
	done              chan struct{}
	changed           chan struct{}
	err               error
}

func (owner *writer) write(frame ardp.Frame, deadline func() time.Time, control, terminal bool) error {
	request := &writeRequest{frame: frame, deadline: deadline, control: control, terminal: terminal, done: make(chan struct{}), changed: make(chan struct{}, 1)}
	owner.state.Lock()
	switch {
	case terminal:
		owner.terminals = append(owner.terminals, request)
	case control:
		owner.controls = append(owner.controls, request)
	default:
		owner.data = append(owner.data, request)
	}
	// A local terminal may interrupt only an ordinary payload already active on
	// that same lane. CREDIT and nested terminal records must finish so their
	// shared framing boundary remains valid; sibling lanes remain independent.
	if terminal && owner.active != nil && owner.active.frame.Lane == frame.Lane && !owner.active.control && !owner.active.terminal {
		_ = owner.connection.SetWriteDeadline(time.Now())
	}
	if !owner.running {
		owner.running = true
		go owner.drain()
	}
	owner.state.Unlock()
	for {
		end := deadline()
		var expired <-chan time.Time
		var timer *time.Timer
		if !end.IsZero() {
			timer = time.NewTimer(time.Until(end))
			expired = timer.C
		}
		select {
		case <-request.done:
			if timer != nil {
				timer.Stop()
			}
			return request.err
		case <-request.changed:
			if timer != nil {
				timer.Stop()
			}
		case <-expired:
			owner.state.Lock()
			// Selection and cancellation use the same state lock. Once selected,
			// the physical writer owns completion and its deadline/poisoning rules.
			// Recheck live authority: a concurrent update may have extended it.
			current := deadline()
			canceled := !current.IsZero() && !time.Now().Before(current) && owner.removeQueuedLocked(request)
			if canceled {
				request.err = os.ErrDeadlineExceeded
				close(request.done)
			}
			active := owner.active == request
			owner.state.Unlock()
			if active || canceled {
				<-request.done
				return request.err
			}
		}
	}
}

// Removal finishes before the caller can reclaim its frame. No detached
// timer can leave canceled work queued or race a later physical emission.
func (owner *writer) removeQueuedLocked(request *writeRequest) bool {
	for _, queue := range []*[]*writeRequest{&owner.terminals, &owner.controls, &owner.data} {
		for index, queued := range *queue {
			if queued == request {
				copy((*queue)[index:], (*queue)[index+1:])
				(*queue)[len(*queue)-1] = nil
				*queue = (*queue)[:len(*queue)-1]
				return true
			}
		}
	}
	return false
}

func (owner *writer) nextLocked() *writeRequest {
	if len(owner.terminals) != 0 && (len(owner.data) == 0 || !owner.terminalServed) {
		request := owner.terminals[0]
		owner.terminals[0] = nil
		owner.terminals = owner.terminals[1:]
		owner.terminalServed = true
		owner.dataDue = true
		return request
	}
	if len(owner.controls) != 0 && (len(owner.data) == 0 || !owner.dataDue) {
		request := owner.controls[0]
		owner.controls[0] = nil
		owner.controls = owner.controls[1:]
		owner.dataDue = true
		return request
	}
	if len(owner.data) != 0 {
		request := owner.data[0]
		owner.data[0] = nil
		owner.data = owner.data[1:]
		owner.dataDue = false
		owner.terminalServed = false
		return request
	}
	return nil
}

func (owner *writer) drain() {
	for {
		owner.writer.Lock()
		owner.state.Lock()
		request := owner.nextLocked()
		if request == nil {
			owner.running = false
			owner.state.Unlock()
			owner.writer.Unlock()
			return
		}
		owner.active = request
		end := request.deadline()
		request.end = end
		// Cancellation while waiting for serialization has emitted no frame.
		// Refuse that owner without damaging other lanes on the same Carrier.
		// After a physical write is attempted, every failure still poisons it.
		if !end.IsZero() && !time.Now().Before(end) {
			request.err = os.ErrDeadlineExceeded
			owner.active = nil
			close(request.done)
			owner.state.Unlock()
			owner.writer.Unlock()
			continue
		}
		err := owner.connection.SetWriteDeadline(end)
		owner.state.Unlock()
		if err == nil {
			err = ardp.WriteFrame(owner.connection, request.frame)
		}
		// An attempted frame may have broken the shared framing boundary.
		// Retain its caller until physical poisoning has joined, including its
		// failure. The caller can then release the child's termination reserve.
		// Do not hold state while Close runs: cancellation may update deadlines.
		if err != nil {
			if owner.closeConnection != nil {
				err = errors.Join(err, owner.closeConnection())
			} else {
				err = errors.Join(err, owner.connection.Close())
			}
		}
		owner.state.Lock()
		owner.active = nil
		owner.failure = errors.Join(owner.failure, err)
		request.err = err
		close(request.done)
		owner.state.Unlock()
		owner.writer.Unlock()
	}
}

// result is read by the Carrier owner after all admitted children joined.
// A role may report an operation failure separately from cleanup, but a failed
// physical frame must still survive in the Carrier's final cleanup result.
func (owner *writer) result() error {
	owner.state.Lock()
	defer owner.state.Unlock()
	return owner.failure
}

func (owner *writer) update(lane uint32, deadline time.Time) error {
	owner.state.Lock()
	defer owner.state.Unlock()
	for _, queue := range [][]*writeRequest{owner.terminals, owner.controls, owner.data} {
		for _, request := range queue {
			if request.frame.Lane == lane {
				select {
				case request.changed <- struct{}{}:
				default:
				}
			}
		}
	}
	if owner.active != nil && owner.active.frame.Kind != 9 && owner.active.frame.Lane == lane {
		if !deadline.IsZero() && (owner.active.end.IsZero() || deadline.Before(owner.active.end)) {
			owner.active.end = deadline
		}
		return owner.connection.SetWriteDeadline(owner.active.end)
	}
	return nil
}

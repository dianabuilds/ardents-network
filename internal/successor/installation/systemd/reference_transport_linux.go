package systemd

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/dianabuilds/ardents-network/internal/successor/systemdbus"
)

// The client decoder is never given an unchecked frame or auth line. In
// particular a finite outer frame alone does not bound nested length fields.
type referenceTransport struct {
	socket           *net.UnixConn
	mu               sync.Mutex
	closed           bool
	active           sync.WaitGroup
	closeOnce        sync.Once
	closeErr         error
	readMu           sync.Mutex
	writeMu          sync.Mutex
	binary           atomic.Bool
	owner            atomic.Value // original manager unique bus name, never rebound
	line             int
	pending          []byte
	reader           *bufio.Reader    // binary input only; unverified read-ahead never drains after Close
	properties       bool             // closed read-only Startup replies; references keep scalar admission
	unitReply        managerUnitReply // immutable before the decoder starts
	sourceJobs       atomic.Bool      // only fixed root Source control admits canonical job replies
	introductionJobs atomic.Bool      // distinct fixed root Introduction control
	descriptorJobs   atomic.Bool      // distinct fixed root Descriptor control
	joinJobs         atomic.Bool      // only distinct root JOIN control
	forwardingJobs   atomic.Bool      // distinct fixed root Forwarding control; never Source permission
}

func (t *referenceTransport) begin() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return false
	}
	t.active.Add(1)
	return true
}

func (t *referenceTransport) Read(p []byte) (int, error) {
	t.readMu.Lock()
	if !t.begin() {
		defer t.readMu.Unlock()
		// The decoder must finish the already verified finite frame, including
		// header padding, without touching the closed socket. Its upstream
		// padding read otherwise panics outside the decoder's error boundary.
		// Owner cancellation and post-observation checks still deny authority.
		if t.binary.Load() && len(t.pending) != 0 {
			n := copy(p, t.pending)
			t.pending = t.pending[n:]
			return n, nil
		}
		return 0, net.ErrClosed
	}
	defer func() {
		t.readMu.Unlock()
		t.active.Done()
	}()
	if len(p) == 0 {
		return 0, nil
	}
	if !t.binary.Load() {
		n, err := t.socket.Read(p[:1]) // no bufio readahead across BEGIN
		if n != 0 {
			t.line++
			if p[0] == '\n' {
				t.line = 0
			}
			if t.line > 4096 {
				return 0, ErrObservation
			}
		}
		return n, err
	}
	if len(t.pending) == 0 {
		if t.reader == nil {
			// AUTH reads stay bytewise through BEGIN. Binary reads may collect a
			// whole reply in one syscall, but every frame is independently bounded
			// and verified before any of its bytes reach the client decoder.
			t.reader = bufio.NewReaderSize(t.socket, 65536)
		}
		if t.reader.Buffered() != 0 {
			// Buffered, unverified bytes cannot hide loss of the original socket.
			// Control only pins/checks the Go descriptor; it performs no raw I/O.
			raw, err := t.socket.SyscallConn()
			if err != nil {
				return 0, err
			}
			if err := raw.Control(func(uintptr) {}); err != nil {
				return 0, err
			}
		}
		header := make([]byte, 16)
		if _, err := io.ReadFull(t.reader, header); err != nil {
			return 0, err
		}
		order, ok := systemdbus.ByteOrder(header[0])
		if !ok {
			return 0, ErrObservation
		}
		head, body := uint64(order.Uint32(header[12:16])), uint64(order.Uint32(header[4:8]))
		total := uint64(16) + (head+7)&^uint64(7) + body
		if total > 65536 {
			return 0, ErrObservation
		}
		frame := make([]byte, int(total))
		copy(frame, header)
		if _, err := io.ReadFull(t.reader, frame[16:]); err != nil {
			return 0, err
		}
		owner, _ := t.owner.Load().(string)
		var frameErr error
		if t.unitReply == introductionControlReply {
			frameErr = verifyNodeManagerFrame(frame, owner, t.properties, false, false, t.introductionJobs.Load(), introductionControlReply)
		} else if t.unitReply == descriptorControlReply {
			frameErr = verifyNodeManagerFrame(frame, owner, t.properties, false, false, t.descriptorJobs.Load(), descriptorControlReply)
		} else if t.unitReply == joinControlReply {
			frameErr = verifyNodeManagerFrame(frame, owner, t.properties, false, false, t.joinJobs.Load(), joinControlReply)
		} else if t.unitReply == forwardingUnitReply {
			frameErr = verifyForwardingManagerFrame(frame, owner, t.properties, t.forwardingJobs.Load())
		} else {
			frameErr = verifyManagerFrame(frame, owner, t.properties, t.sourceJobs.Load(), t.unitReply)
		}
		if frameErr != nil {
			return 0, frameErr
		}
		t.pending = frame
	}
	n := copy(p, t.pending)
	t.pending = t.pending[n:]
	return n, nil
}

func (t *referenceTransport) Write(p []byte) (int, error) {
	if !t.begin() {
		return 0, net.ErrClosed
	}
	defer t.active.Done()
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if len(p) > 65536 {
		return 0, ErrObservation
	}
	n, err := t.socket.Write(p)
	if err == nil && n == len(p) && bytes.Equal(p, []byte("BEGIN\r\n")) {
		t.binary.Store(true)
	}
	return n, err
}

// Closing first interrupts blocked I/O; Add is prohibited before Wait begins.
// Queued and later library reads cannot borrow the original descriptor again.
// Only the remainder of an already verified frame may drain from memory.
func (t *referenceTransport) Close() error {
	if t == nil {
		return nil
	}
	t.closeOnce.Do(func() { t.mu.Lock(); t.closed = true; t.mu.Unlock(); t.closeErr = t.socket.Close(); t.active.Wait() })
	return t.closeErr
}

// Closed scalar reply vocabulary covers Hello/GetNameOwner/RefUnit/UnrefUnit/
// Ping and broker notifications. Property observations additionally admit the
// bounded GetAll dictionary grammar. Header variants retain exact scalar types.
func verifyManagerFrame(frame []byte, owner string, properties, sourceJobs bool, reply managerUnitReply) error {
	return verifyClosedManagerFrame(frame, owner, properties, sourceJobs, false, reply)
}

func verifyForwardingManagerFrame(frame []byte, owner string, properties, forwardingJobs bool) error {
	return verifyClosedManagerFrame(frame, owner, properties, false, forwardingJobs, forwardingUnitReply)
}

func verifyClosedManagerFrame(frame []byte, owner string, properties, sourceJobs, forwardingJobs bool, reply managerUnitReply) error {
	return verifyNodeManagerFrame(frame, owner, properties, sourceJobs, forwardingJobs, false, reply)
}

func verifyNodeManagerFrame(frame []byte, owner string, properties, sourceJobs, forwardingJobs, joinJobs bool, reply managerUnitReply) error {
	if !reply.valid() {
		return ErrObservation
	}
	allowJob := reply == sourceUnitReply && sourceJobs || reply == forwardingUnitReply && forwardingJobs || (reply == joinControlReply || reply == descriptorControlReply || reply == introductionControlReply) && joinJobs
	return systemdbus.ValidateFrame(frame, systemdbus.FramePolicy{Owner: owner, Properties: properties, AllowObject: reply != nodeSlotsReply, ExpectedObject: reply.object(), AllowJob: allowJob})
}

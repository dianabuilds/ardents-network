package systemd

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
)

// The client decoder is never given an unchecked frame or auth line. In
// particular a finite outer frame alone does not bound nested length fields.
type referenceTransport struct {
	socket             *net.UnixConn
	mu                 sync.Mutex
	closed             bool
	active             sync.WaitGroup
	closeOnce          sync.Once
	closeErr           error
	readMu             sync.Mutex
	writeMu            sync.Mutex
	binary             atomic.Bool
	owner              atomic.Value // original manager unique bus name, never rebound
	line               int
	auth               [128]byte
	authStart, authEnd int
	pending            []byte
	reader             *bufio.Reader    // binary input only; unverified read-ahead never drains after Close
	properties         bool             // closed read-only Startup replies; references keep scalar admission
	unitReply          managerUnitReply // immutable before the decoder starts
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
		n, err := t.readAuthByte(p[:1])
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
		order, ok := referenceByteOrder(header[0])
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
		if err := verifyManagerFrame(frame, owner, t.properties, t.unitReply); err != nil {
			return 0, err
		}
		t.pending = frame
	}
	n := copy(p, t.pending)
	t.pending = t.pending[n:]
	return n, nil
}

// readAuthByte supplies the original bytewise authentication interface while
// consuming each available line prefix in one socket read. Peek observes only
// length and the first newline. There is no buffered binary read before BEGIN.
func (t *referenceTransport) readAuthByte(p []byte) (int, error) {
	if t.authStart == t.authEnd {
		limit := min(len(t.auth), 4096-t.line+1)
		raw, err := t.socket.SyscallConn()
		if err != nil {
			return 0, err
		}
		var available int
		var receiveErr error
		if err := raw.Read(func(fd uintptr) bool {
			for {
				available, _, receiveErr = syscall.Recvfrom(int(fd), t.auth[:limit], syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
				if receiveErr != syscall.EINTR {
					return receiveErr != syscall.EAGAIN
				}
			}
		}); err != nil {
			return 0, err
		}
		if receiveErr != nil {
			return 0, receiveErr
		}
		if available == 0 {
			return 0, io.EOF
		}
		if end := bytes.IndexByte(t.auth[:available], '\n'); end >= 0 {
			available = end + 1
		}
		if _, err := io.ReadFull(t.socket, t.auth[:available]); err != nil {
			return 0, err
		}
		t.authStart, t.authEnd = 0, available
	}
	p[0] = t.auth[t.authStart]
	t.authStart++
	return 1, nil
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

func referenceByteOrder(marker byte) (binary.ByteOrder, bool) {
	if marker == 'l' {
		return binary.LittleEndian, true
	}
	if marker == 'B' {
		return binary.BigEndian, true
	}
	return nil, false
}

// Closed scalar reply vocabulary covers Hello/GetNameOwner/RefUnit/UnrefUnit/
// Ping and broker notifications. Property observations additionally admit the
// bounded GetAll dictionary grammar. Header variants retain exact scalar types.
func verifyManagerFrame(frame []byte, owner string, properties bool, reply managerUnitReply) error {
	if len(frame) < 16 || len(frame) > 65536 || !reply.valid() {
		return ErrObservation
	}
	order, ok := referenceByteOrder(frame[0])
	if !ok || frame[3] != 1 || frame[1] < 2 || frame[1] > 4 {
		return ErrObservation
	}
	hlen, blen := uint64(order.Uint32(frame[12:16])), uint64(order.Uint32(frame[4:8]))
	end := uint64(16) + hlen
	bodyStart := (end + 7) &^ uint64(7)
	if end > uint64(len(frame)) || bodyStart+blen != uint64(len(frame)) {
		return ErrObservation
	}
	fields := make(map[byte]string)
	for pos := 16; pos < int(end); {
		pos = (pos + 7) &^ 7
		if pos+4 > int(end) {
			return ErrObservation
		}
		code := frame[pos]
		if code < 1 || code > 9 {
			return ErrObservation
		}
		if _, exists := fields[code]; exists {
			return ErrObservation
		}
		kind := byte('s')
		switch code {
		case 1:
			kind = 'o'
		case 5, 9:
			kind = 'u'
		case 8:
			kind = 'g'
		}
		if frame[pos+1] != 1 || frame[pos+2] != kind || frame[pos+3] != 0 {
			return ErrObservation
		}
		pos += 4
		if kind == 'u' {
			if pos+4 > int(end) {
				return ErrObservation
			}
			value := order.Uint32(frame[pos : pos+4])
			if code == 9 && value != 0 {
				return ErrObservation
			}
			fields[code] = "u"
			pos += 4
			continue
		}
		var value string
		var err error
		value, pos, err = referenceScalar(frame, pos, int(end), kind, order)
		if err != nil {
			return err
		}
		fields[code] = value
	}
	sender := fields[7]
	if sender != "org.freedesktop.DBus" && (owner == "" || sender != owner) {
		return ErrObservation
	}
	if frame[1] == 4 && (sender != "org.freedesktop.DBus" || fields[2] != "org.freedesktop.DBus" || (fields[3] != "NameAcquired" && fields[3] != "NameLost")) {
		return ErrObservation
	}
	if frame[1] != 4 && fields[5] != "u" {
		return ErrObservation
	}
	sig := fields[8]
	if properties && frame[1] == 2 && owner != "" && sender == owner && (sig == "v" || sig == "a{sv}") {
		return verifyManagerValues(frame, int(bodyStart), sig, order)
	}
	// A read-only role connection admits only its fixed unit's object.
	// Absence-only connections refuse all object replies before decode.
	if properties && frame[1] == 2 && owner != "" && sender == owner && sig == "o" {
		if reply == nodeSlotsReply {
			return ErrObservation
		}
		object, pos, err := referenceScalar(frame, int(bodyStart), len(frame), 'o', order)
		if err != nil || pos != len(frame) || object != reply.object() {
			return ErrObservation
		}
		return nil
	}
	if sig == "" {
		if blen != 0 || frame[1] != 2 {
			return ErrObservation
		}
		return nil
	}
	if sig != "s" {
		return ErrObservation
	}
	_, pos, err := referenceScalar(frame, int(bodyStart), len(frame), 's', order)
	if err != nil || pos != len(frame) {
		return ErrObservation
	}
	return nil
}

func referenceScalar(frame []byte, pos, end int, kind byte, order binary.ByteOrder) (string, int, error) {
	var size uint64
	if kind == 'g' {
		if pos >= end {
			return "", pos, ErrObservation
		}
		size = uint64(frame[pos])
		pos++
	} else {
		pos = (pos + 3) &^ 3
		if pos+4 > end {
			return "", pos, ErrObservation
		}
		size = uint64(order.Uint32(frame[pos : pos+4]))
		pos += 4
	}
	if size > uint64(end-pos) || size+1 > uint64(end-pos) {
		return "", pos, ErrObservation
	}
	finish := pos + int(size)
	if frame[finish] != 0 || bytes.IndexByte(frame[pos:finish], 0) >= 0 {
		return "", pos, ErrObservation
	}
	return string(frame[pos:finish]), finish + 1, nil
}

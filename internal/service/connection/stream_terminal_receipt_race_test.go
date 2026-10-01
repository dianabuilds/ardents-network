package connection

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

// receiptBoundaryConn holds the first complete outbound record after the peer
// has read it but before its writer can publish completion. It also reports
// when the inbound reader has entered its next record read.
type receiptBoundaryConn struct {
	net.Conn
	mu                sync.Mutex
	written           []byte
	read              []byte
	firstWritten      chan struct{}
	releaseFirst      chan struct{}
	nextRead          chan struct{}
	writeHeld         bool
	firstReadComplete bool
	nextReadSignaled  bool
}

func (conn *receiptBoundaryConn) Write(buffer []byte) (int, error) {
	count, err := conn.Conn.Write(buffer)
	if count > 0 {
		conn.mu.Lock()
		conn.written = append(conn.written, buffer[:count]...)
		complete := !conn.writeHeld && completeFirstReceiptFrame(conn.written)
		if complete {
			conn.writeHeld = true
		}
		conn.mu.Unlock()
		if complete {
			close(conn.firstWritten)
			<-conn.releaseFirst
		}
	}
	return count, err
}

func (conn *receiptBoundaryConn) Read(buffer []byte) (int, error) {
	conn.mu.Lock()
	if conn.firstReadComplete && !conn.nextReadSignaled {
		conn.nextReadSignaled = true
		close(conn.nextRead)
	}
	conn.mu.Unlock()
	count, err := conn.Conn.Read(buffer)
	if count > 0 {
		conn.mu.Lock()
		if !conn.firstReadComplete {
			conn.read = append(conn.read, buffer[:count]...)
			conn.firstReadComplete = completeFirstReceiptFrame(conn.read)
		}
		conn.mu.Unlock()
	}
	return count, err
}

func completeFirstReceiptFrame(raw []byte) bool {
	header := len(connectionPrefix) + 2
	return len(raw) >= header && len(raw) >= header+int(binary.BigEndian.Uint16(raw[len(connectionPrefix):header]))
}

func TestTerminalTailOwnsDuplicateReceiptAfterAcknowledgementWorkerExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	local, peer := net.Pipe()
	if err := peer.SetDeadline(time.Now().Add(12 * time.Second)); err != nil {
		t.Fatal(err)
	}
	carrier := &receiptBoundaryConn{Conn: local, firstWritten: make(chan struct{}),
		releaseFirst: make(chan struct{}), nextRead: make(chan struct{})}
	attachment := terminalRecoveryAttachment(t, carrier, 2, [32]byte{4}, [32]byte{5})
	application, user := halfClosePair()
	defer user.Close()
	stream, err := NewStream(StreamConfig{
		Context: ctx, Application: application, Initial: attachment,
		ContinuityKey: [32]byte{6}, Authorized: time.Now(),
		Recovery: Recovery{NoNewRecoveryAfter: time.Now().Add(time.Minute).Unix()},
		OpenAttachment: func(context.Context, Recovery) (*Attachment, error) {
			return nil, errors.New("unexpected recovery")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tailStarted := false
	var releaseOnce sync.Once
	defer func() {
		releaseOnce.Do(func() { close(carrier.releaseFirst) })
		cancel()
		peer.Close()
		if tailStarted {
			select {
			case <-stream.done:
			case <-time.After(time.Second):
				t.Error("terminal tail did not join during cleanup")
			}
		} else {
			stream.close()
		}
	}()

	// A generation-2 receipt for the local Terminal is already authenticated.
	// The peer's remote Terminal acknowledgement is still being written.
	stream.mu.Lock()
	stream.sendBase, stream.sendEnd, stream.sendNext = 7, 7, 7
	stream.localTerminal, stream.terminalSettled, stream.remoteTerminal = true, true, true
	stream.terminalGeneration, stream.terminalOffset = 2, 7
	stream.terminalAcknowledgedGeneration = 2
	stream.terminalConfirmationPending = true
	stream.terminalConfirmationGeneration, stream.terminalConfirmationOffset = 2, 7
	stream.terminalAckPending = true
	stream.terminalAckPendingGeneration = 2
	stream.mu.Unlock()

	receiver := make(chan error, 1)
	go func() { receiver <- stream.receiveApplicationBounded(0) }()
	ackWorker := make(chan error, 1)
	go func() { ackWorker <- stream.sendBoundedAcknowledgements() }()
	stream.signalAcknowledgement()

	first, err := ReadStream(peer)
	if err != nil {
		t.Fatalf("read first Terminal acknowledgement: %v", err)
	}
	if first.Acknowledgement == nil || !first.Acknowledgement.Terminal || first.Acknowledgement.TerminalConfirmation ||
		first.Acknowledgement.AttachmentGeneration != 2 || first.Acknowledgement.Offset != 0 {
		t.Fatalf("first Terminal acknowledgement = %+v", first.Acknowledgement)
	}
	select {
	case <-carrier.firstWritten:
	case <-ctx.Done():
		t.Fatal("first Terminal acknowledgement write did not reach gate")
	}

	// Its peer confirmation can arrive while the first write is held. The
	// receiver must reenter ReadStream before the writer publishes AckSent.
	confirmation := &Acknowledgement{AttachmentGeneration: 2, Offset: 0, Terminal: true, TerminalConfirmation: true}
	if err := Write(peer, Record{Acknowledgement: confirmation}); err != nil {
		t.Fatalf("write peer Terminal receipt confirmation: %v", err)
	}
	select {
	case <-carrier.nextRead:
	case <-ctx.Done():
		t.Fatal("receiver did not await the next generation-2 record")
	}
	releaseOnce.Do(func() { close(carrier.releaseFirst) })
	firstConfirmation, err := ReadStream(peer)
	if err != nil {
		t.Fatalf("read initial local Terminal confirmation: %v", err)
	}
	if firstConfirmation.Acknowledgement == nil || !firstConfirmation.Acknowledgement.TerminalConfirmation ||
		firstConfirmation.Acknowledgement.AttachmentGeneration != 2 || firstConfirmation.Acknowledgement.Offset != 7 {
		t.Fatalf("initial local Terminal confirmation = %+v", firstConfirmation.Acknowledgement)
	}
	select {
	case err := <-ackWorker:
		if err != nil {
			t.Fatalf("ordinary acknowledgement worker: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("ordinary acknowledgement worker did not finish")
	}

	duplicate := &Acknowledgement{AttachmentGeneration: 2, Offset: 7, Terminal: true}
	if err := Write(peer, Record{Acknowledgement: duplicate}); err != nil {
		t.Fatalf("write duplicate generation-2 Terminal receipt: %v", err)
	}
	select {
	case err := <-receiver:
		if err != nil {
			t.Fatalf("ordinary receiver: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("ordinary receiver did not finish")
	}
	stream.mu.Lock()
	pending, sent := stream.terminalConfirmationPending, stream.terminalConfirmationSent
	writtenGeneration, writtenOffset := stream.terminalConfirmationWrittenGeneration, stream.terminalConfirmationWrittenOffset
	stream.mu.Unlock()
	if !pending || sent {
		t.Fatalf("duplicate receipt did not require replay: pending=%t sent=%t", pending, sent)
	}
	if writtenGeneration != 2 || writtenOffset != 7 {
		t.Fatalf("successful confirmation witness = generation %d offset %d, want 2/7", writtenGeneration, writtenOffset)
	}

	tailStarted = stream.startTerminalTail(func() {}, nil)
	if !tailStarted {
		t.Fatal("successful ordinary ACK completion lost terminal-control tail after duplicate Terminal receipt")
	}
	replay, err := ReadStream(peer)
	if err != nil {
		t.Fatalf("read replayed Terminal confirmation: %v", err)
	}
	if replay.Acknowledgement == nil || !replay.Acknowledgement.TerminalConfirmation ||
		replay.Acknowledgement.AttachmentGeneration != 2 || replay.Acknowledgement.Offset != 7 {
		t.Fatalf("replayed Terminal confirmation = %+v", replay.Acknowledgement)
	}
	if err := stream.RetireTerminalTail(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stream.done:
	case <-ctx.Done():
		t.Fatal("terminal-control tail did not join")
	}
}

func TestFailedTerminalConfirmationWriteDoesNotCreateTailWitness(t *testing.T) {
	carrier, peer := net.Pipe()
	peer.Close()
	attachment := terminalRecoveryAttachment(t, carrier, 1, [32]byte{4}, [32]byte{5})
	defer attachment.retireCarrier()
	stream := &Stream{ctx: t.Context(), current: attachment, localTerminal: true, remoteTerminal: true,
		terminalAcknowledgedGeneration: 1, terminalAckPending: true, terminalAckSent: true,
		terminalAckGeneration: 1, terminalConfirmationPending: true,
		terminalConfirmationGeneration: 1, ackSignal: make(chan struct{}, 1)}
	stream.cond = sync.NewCond(&stream.mu)
	stream.signalAcknowledgement()
	if err := stream.sendBoundedAcknowledgements(); err == nil {
		t.Fatal("failed confirmation write returned success")
	}
	if stream.terminalConfirmationWrittenGeneration != 0 || stream.terminalConfirmationSent {
		t.Fatal("failed confirmation write created a tail witness")
	}
}

func TestTerminalTailRejectsConfirmationWitnessForDifferentTerminal(t *testing.T) {
	for _, test := range []struct {
		name       string
		generation uint64
		offset     uint64
	}{
		{name: "no successful write"},
		{name: "wrong offset", generation: 2, offset: 8},
		{name: "unacknowledged generation", generation: 3, offset: 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			stream := &Stream{localTerminal: true, remoteTerminal: true, terminalOffset: 7,
				terminalAcknowledgedGeneration:        2,
				terminalConfirmationWrittenGeneration: test.generation,
				terminalConfirmationWrittenOffset:     test.offset,
				opener:                                func(context.Context, Recovery) (*Attachment, error) { return nil, nil }}
			stream.cond = sync.NewCond(&stream.mu)
			if stream.startTerminalTail(func() {}, nil) {
				t.Fatal("different Terminal confirmation took tail ownership")
			}
		})
	}
}

package connection

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
)

func TestRecoveredDataReplayJoinsAfterCancellation(t *testing.T) {
	carrier := &blockingDataReplayCarrier{started: make(chan struct{}), closed: make(chan struct{}), returned: make(chan struct{})}
	attachment, err := NewAttachment(carrier, 1, [32]byte{1}, [32]byte{2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := &Stream{current: attachment, sendData: []byte("suffix"), sendEnd: uint64(len("suffix")), ackSignal: make(chan struct{}, 1)}
	stream.cond = sync.NewCond(&stream.mu)
	stream.startRecoveredDataReplay()
	select {
	case <-carrier.started:
	case <-t.Context().Done():
		t.Fatal("Data replay did not start")
	}
	stream.fail(context.Canceled)
	select {
	case <-carrier.returned:
	case <-t.Context().Done():
		t.Fatal("cancellation did not interrupt Data replay write")
	}
	stream.mu.Lock()
	for stream.dataReplaying {
		stream.cond.Wait()
	}
	terminal := stream.terminal
	stream.mu.Unlock()
	if !errors.Is(terminal, context.Canceled) {
		t.Fatalf("cancellation terminal = %v", terminal)
	}
}

func TestEnsureTerminalFlushesCommittedDataBeforeTerminal(t *testing.T) {
	writer, reader := net.Pipe()
	attachment, err := NewAttachment(writer, 2, [32]byte{1}, [32]byte{2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := &Stream{current: attachment, sendData: []byte("suffix"), sendEnd: uint64(len("suffix")), localTerminal: true}
	stream.cond = sync.NewCond(&stream.mu)
	first := make(chan StreamRecord, 1)
	second := make(chan StreamRecord, 1)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		record, readErr := ReadStream(reader)
		if readErr == nil {
			first <- record
			record, readErr = ReadStream(reader)
			if readErr == nil {
				second <- record
			}
		}
	}()
	t.Cleanup(func() {
		_ = writer.Close()
		_ = reader.Close()
		<-readDone
	})
	if err := stream.ensureTerminal(); err != nil {
		t.Fatal(err)
	}
	firstRecord := <-first
	secondRecord := <-second
	if firstRecord.Data == nil || string(firstRecord.Data.Payload) != "suffix" || firstRecord.Data.Offset != 0 {
		t.Fatalf("first committed-replay record = %+v", firstRecord)
	}
	if secondRecord.Terminal == nil || secondRecord.Terminal.Offset != uint64(len("suffix")) {
		t.Fatalf("Terminal after committed replay = %+v", secondRecord)
	}
}

type blockingDataReplayCarrier struct {
	started, closed, returned chan struct{}
	closeOnce                 sync.Once
}

func (carrier *blockingDataReplayCarrier) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (carrier *blockingDataReplayCarrier) Write([]byte) (int, error) {
	close(carrier.started)
	<-carrier.closed
	close(carrier.returned)
	return 0, io.ErrClosedPipe
}

func (carrier *blockingDataReplayCarrier) Close() error {
	carrier.closeOnce.Do(func() { close(carrier.closed) })
	return nil
}

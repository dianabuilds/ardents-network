package reader

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

func TestResultStreamReceivesBytesAfterClosingItsSendingDirection(t *testing.T) {
	_, cancel := context.WithCancel(t.Context())
	defer cancel()
	request, input := io.Pipe()
	output, response := io.Pipe()
	stream := &resultStream{input: input, output: output, cancel: cancel, joined: make(chan struct{})}
	defer stream.Close()
	sent := []byte{0, 0xff}
	reply := []byte{0x80, 0}
	go func() {
		defer close(stream.joined)
		defer request.Close()
		body, err := io.ReadAll(request)
		if err == nil && !bytes.Equal(body, sent) {
			err = errors.New("sending direction changed the bytes")
		}
		if err == nil {
			_, err = response.Write(reply)
		}
		stream.err = err
		_ = response.CloseWithError(err)
	}()
	if _, err := stream.Write(sent); err != nil {
		t.Fatal(err)
	}
	if err := stream.CloseInput(); err != nil {
		t.Fatal(err)
	}
	if body, err := io.ReadAll(stream); err != nil || !bytes.Equal(body, reply) {
		t.Fatalf("received %x, %v", body, err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestResultStreamCloseJoinsOriginalSettlement(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	request, input := io.Pipe()
	output, response := io.Pipe()
	defer request.Close()
	defer response.Close()
	release := make(chan struct{})
	settled := errors.New("original cleanup failed")
	stream := &resultStream{input: input, output: output, cancel: cancel, joined: make(chan struct{})}
	go func() {
		<-ctx.Done()
		<-release
		stream.err = settled
		close(stream.joined)
	}()
	completed := make(chan error, 1)
	go func() { completed <- stream.Close() }()
	<-ctx.Done()
	select {
	case <-stream.Done():
		t.Fatal("cancellation reported completion before settlement")
	default:
	}
	select {
	case <-completed:
		t.Fatal("Close returned before original settlement")
	default:
	}
	close(release)
	if err := <-completed; !errors.Is(err, settled) {
		t.Fatal("lost original cleanup failure", err)
	}
	if err := stream.Close(); !errors.Is(err, settled) {
		t.Fatal("repeated Close lost retained failure", err)
	}
}

package connection

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type testStream struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	done   chan Outcome

	inputOnce sync.Once
	closeOnce sync.Once
}

func (stream *testStream) Done() <-chan Outcome { return stream.done }

func testStreamPair(done chan Outcome) (*testStream, *testStream) {
	leftReader, rightWriter := io.Pipe()
	rightReader, leftWriter := io.Pipe()
	return &testStream{reader: leftReader, writer: leftWriter, done: done},
		&testStream{reader: rightReader, writer: rightWriter, done: done}
}

func (stream *testStream) Read(destination []byte) (int, error) {
	return stream.reader.Read(destination)
}

func (stream *testStream) Write(source []byte) (int, error) {
	return stream.writer.Write(source)
}

func (stream *testStream) CloseInput() error {
	var result error
	stream.inputOnce.Do(func() { result = stream.writer.Close() })
	return result
}

func (stream *testStream) Close() error {
	var result error
	stream.closeOnce.Do(func() { result = errors.Join(stream.CloseInput(), stream.reader.Close()) })
	return result
}

type testInterface func(context.Context, string) (Stream, error)

func (open testInterface) Open(ctx context.Context, targetLink string) (Stream, error) {
	return open(ctx, targetLink)
}

func TestLocalTransportCarriesOnlyTargetLinkBytesAndTerminalOutcome(t *testing.T) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("ac-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(path) })
	opened := make(chan string, 1)
	server, err := Listen(path, testInterface(func(_ context.Context, targetLink string) (Stream, error) {
		opened <- targetLink
		done := make(chan Outcome, 1)
		serverSide, applicationSide := testStreamPair(done)
		go func() {
			request, readErr := io.ReadAll(serverSide)
			if readErr == nil {
				_, _ = io.WriteString(serverSide, "reply:"+string(request))
			}
			_ = serverSide.CloseInput()
			done <- Outcome{Class: CleanClose, Reason: "fixture complete"}
			close(done)
		}()
		return applicationSide, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	client, err := Dial(t.Context(), path, "ardents-alpha://reference")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if got := <-opened; got != "ardents-alpha://reference" {
		t.Fatalf("opened Target Link = %q", got)
	}
	if _, err := io.WriteString(client, "hello\n"); err != nil {
		t.Fatal(err)
	}
	if err := client.CloseInput(); err != nil {
		t.Fatal(err)
	}
	reply, err := io.ReadAll(client)
	if err != nil || string(reply) != "reply:hello\n" {
		t.Fatalf("reply = %q, %v", reply, err)
	}
	if outcome := <-client.Done(); outcome.Class != CleanClose || outcome.Reason != "fixture complete" {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestLocalTransportPreservesTypedRefusal(t *testing.T) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("ar-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(path) })
	server, err := Listen(path, testInterface(func(context.Context, string) (Stream, error) {
		return nil, Refuse(Outcome{Class: "transit grant exhausted", Reason: "current issuer budget is exhausted"})
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	if _, err := Dial(t.Context(), path, "ardents-alpha://reference"); err == nil ||
		err.Error() != "transit grant exhausted: current issuer budget is exhausted" {
		t.Fatalf("typed refusal = %v", err)
	}
}

func TestLocalTransportDoesNotTreatAbruptOrMalformedTerminalAsClean(t *testing.T) {
	for _, test := range []struct {
		name      string
		malformed bool
	}{
		{name: "abrupt"},
		{name: "malformed", malformed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(os.TempDir(), fmt.Sprintf("ae-%d.sock", time.Now().UnixNano()))
			t.Cleanup(func() { _ = os.Remove(path) })
			listener, err := net.Listen("unix", path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Close() })
			go func() {
				connection, acceptErr := listener.Accept()
				if acceptErr != nil {
					return
				}
				defer connection.Close()
				header := make([]byte, len(localMagic)+2)
				if _, err := io.ReadFull(connection, header); err != nil {
					return
				}
				link := make([]byte, binary.BigEndian.Uint16(header[len(localMagic):]))
				if _, err := io.ReadFull(connection, link); err != nil {
					return
				}
				if _, err := connection.Write([]byte{1}); err != nil || !test.malformed {
					return
				}
				_, _ = connection.Write([]byte{0, 0, 0, 0})
			}()
			client, err := Dial(t.Context(), path, "ardents-alpha://reference")
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			_, _ = io.ReadAll(client)
			outcome, open := <-client.Done()
			if !open || outcome.Class == CleanClose {
				t.Fatalf("%s terminal outcome = %+v, open=%t", test.name, outcome, open)
			}
		})
	}
}

func TestDialRejectsPreTransportEOF(t *testing.T) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("ap-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(path) })
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		header := make([]byte, len(localMagic)+2)
		if _, err := io.ReadFull(connection, header); err != nil {
			return
		}
		link := make([]byte, binary.BigEndian.Uint16(header[len(localMagic):]))
		_, _ = io.ReadFull(connection, link)
	}()
	if client, err := Dial(t.Context(), path, "ardents-alpha://reference"); err == nil || client != nil {
		t.Fatalf("pre-transport EOF opened client=%v err=%v", client, err)
	}
}

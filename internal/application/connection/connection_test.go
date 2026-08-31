package connection

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testStream struct {
	net.Conn
	done chan Outcome
}

func (stream *testStream) Done() <-chan Outcome { return stream.done }

type testInterface func(context.Context, string) (Stream, error)

func (open testInterface) Open(ctx context.Context, serviceLink string) (Stream, error) {
	return open(ctx, serviceLink)
}

func TestLocalTransportCarriesOnlyServiceLinkBytesAndTerminalOutcome(t *testing.T) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("ac-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(path) })
	opened := make(chan string, 1)
	server, err := Listen(path, testInterface(func(_ context.Context, serviceLink string) (Stream, error) {
		opened <- serviceLink
		serverSide, applicationSide := net.Pipe()
		done := make(chan Outcome, 1)
		go func() {
			request, readErr := bufio.NewReader(serverSide).ReadString('\n')
			if readErr == nil {
				_, _ = io.WriteString(serverSide, "reply:"+request)
			}
			_ = serverSide.Close()
			done <- Outcome{Class: CleanClose, Reason: "fixture complete"}
			close(done)
		}()
		return &testStream{Conn: applicationSide, done: done}, nil
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
		t.Fatalf("opened Service Link = %q", got)
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

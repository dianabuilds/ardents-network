package endpoint

import (
	"bufio"
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestLocalConnectionInterfaceCarriesOnlyServiceLinkBytesAndTerminal(t *testing.T) {
	t.Parallel()
	path := shortApplicationPath(t)
	opened := make(chan string, 1)
	server, err := openLocalConnectionInterface(localConnectionInterfaceConfig{Path: path,
		Open: func(ctx context.Context, serviceLink string) (*ApplicationConnection, error) {
			opened <- serviceLink
			endpointSide, applicationSide := net.Pipe()
			done := make(chan ApplicationOutcome, 1)
			go func() {
				request, readErr := bufio.NewReader(endpointSide).ReadString('\n')
				if readErr == nil {
					_, readErr = io.WriteString(endpointSide, "reply:"+request)
				}
				_ = endpointSide.Close()
				outcome := ApplicationOutcome{Class: "clean service connection close", Reason: "fixture complete"}
				if readErr != nil {
					outcome = ApplicationOutcome{Class: "indeterminate failure", Reason: "fixture failed"}
				}
				done <- outcome
				close(done)
			}()
			return &ApplicationConnection{stream: applicationSide, cancel: func() {}, done: done}, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	client, err := DialLocalApplication(t.Context(), path, "ardents-alpha://reference")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if got := <-opened; got != "ardents-alpha://reference" {
		t.Fatalf("local Connection Interface opened %q", got)
	}
	if _, err := io.WriteString(client, "hello\n"); err != nil {
		t.Fatal(err)
	}
	reply, err := bufio.NewReader(client).ReadString('\n')
	if err != nil || reply != "reply:hello\n" {
		t.Fatalf("local Application reply = %q, %v", reply, err)
	}
	if _, err := io.ReadAll(client); err != nil {
		t.Fatal(err)
	}
	select {
	case outcome := <-client.Done():
		if outcome.Class != "clean service connection close" || outcome.Reason != "fixture complete" {
			t.Fatalf("local terminal outcome = %+v", outcome)
		}
	case <-time.After(time.Second):
		t.Fatal("local Application terminal outcome was not delivered")
	}
}

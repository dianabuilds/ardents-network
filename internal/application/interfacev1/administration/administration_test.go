package administration

import (
	"context"
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

type testInterface struct {
	publish  func(context.Context) error
	withdraw func(context.Context) error
}

func (owner testInterface) Publish(ctx context.Context) error  { return owner.publish(ctx) }
func (owner testInterface) Withdraw(ctx context.Context) error { return owner.withdraw(ctx) }

func TestLocalAdministrationDispatchesOnlyClosedOperations(t *testing.T) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("aa-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(path) })
	published, withdrawn := make(chan struct{}, 1), make(chan struct{}, 1)
	server, err := Listen(path, testInterface{
		publish:  func(context.Context) error { published <- struct{}{}; return nil },
		withdraw: func(context.Context) error { withdrawn <- struct{}{}; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	if outcome, err := Request(t.Context(), path, Publish); err != nil || outcome != Published {
		t.Fatalf("publish = %q, %v", outcome, err)
	}
	<-published
	if outcome, err := Request(t.Context(), path, Withdraw); err != nil || outcome != Withdrawn {
		t.Fatalf("withdraw = %q, %v", outcome, err)
	}
	<-withdrawn
	if _, err := Request(t.Context(), path, Operation("route")); err == nil {
		t.Fatal("unknown Administration operation was accepted")
	}
	raw, err := (&net.Dialer{}).DialContext(t.Context(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	connection := raw.(*net.UnixConn)
	_, _ = connection.Write([]byte("publish\nsurplus"))
	_ = connection.CloseWrite()
	response := make([]byte, 12)
	read, _ := connection.Read(response)
	_ = connection.Close()
	if string(response[:read]) != "unavailable\n" {
		t.Fatalf("surplus response = %q", response[:read])
	}
}

func TestRequestCancellationInterruptsAcceptedOperation(t *testing.T) {
	for _, test := range []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
	}{
		{"without deadline", func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) }},
		{"before later deadline", func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), 5*time.Second)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path, received, release := heldRequestPeer(t)
			ctx, cancel := test.ctx()
			defer cancel()
			result := make(chan error, 1)
			go func() {
				_, err := Request(ctx, path, Publish)
				result <- err
			}()
			select {
			case <-received:
			case <-time.After(time.Second):
				t.Fatal("peer did not receive the accepted request")
			}
			cancel()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("Request error = %v, want context cancellation", err)
				}
			case <-time.After(time.Second):
				t.Fatal("Request remained blocked after caller cancellation")
			}
			release()
		})
	}
}

func TestRequestWithCancelledContextDoesNotDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Request(ctx, filepath.Join(os.TempDir(), "absent-administration.sock"), Publish); !errors.Is(err, context.Canceled) {
		t.Fatalf("Request error = %v, want context cancellation", err)
	}
}

func TestRequestDeadlineReturnsContextCause(t *testing.T) {
	path, received, release := heldRequestPeer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := Request(ctx, path, Publish)
		result <- err
	}()
	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("peer did not receive the accepted request")
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Request error = %v, want context deadline", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Request remained blocked after caller deadline")
	}
	release()
}

func TestRequestCancellationRacesNormalOutcome(t *testing.T) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("aa-race-%d.sock", time.Now().UnixNano()))
	started, release := make(chan struct{}, 1), make(chan struct{})
	server, err := Listen(path, testInterface{
		publish:  func(context.Context) error { started <- struct{}{}; <-release; return nil },
		withdraw: func(context.Context) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// The completed client read can precede the handler's deferred Close.
		// Server.Close joins that expected duplicate close with its own cleanup.
		if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			t.Errorf("close Administration server: %v", closeErr)
		}
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("Administration socket remains after server close: %v", statErr)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		outcome Outcome
		err     error
	}
	completed := make(chan result, 1)
	go func() {
		outcome, requestErr := Request(ctx, path, Publish)
		completed <- result{outcome, requestErr}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("server did not accept the published operation")
	}
	close(release)
	cancel()
	select {
	case result := <-completed:
		if (result.outcome != Published || result.err != nil) && !errors.Is(result.err, context.Canceled) {
			t.Fatalf("Request race result = %q, %v", result.outcome, result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("Request deadlocked while cancellation raced the outcome")
	}
}

func heldRequestPeer(t *testing.T) (string, <-chan struct{}, func()) {
	t.Helper()
	path := filepath.Join(os.TempDir(), fmt.Sprintf("aa-cancel-%d.sock", time.Now().UnixNano()))
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	received, release := make(chan struct{}, 1), make(chan struct{})
	var releaseOnce sync.Once
	releasePeer := func() { releaseOnce.Do(func() { close(release) }) }
	var work sync.WaitGroup
	work.Add(1)
	go func() {
		defer work.Done()
		connection, acceptErr := listener.AcceptUnix()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		request := make([]byte, len("publish\n"))
		if _, readErr := io.ReadFull(connection, request); readErr == nil && string(request) == "publish\n" {
			received <- struct{}{}
		}
		<-release
	}()
	t.Cleanup(func() {
		releasePeer()
		if closeErr := listener.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			t.Errorf("close held request listener: %v", closeErr)
		}
		work.Wait()
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			t.Errorf("remove held request socket: %v", removeErr)
		}
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("held request socket remains after cleanup: %v", statErr)
		}
	})
	return path, received, releasePeer
}

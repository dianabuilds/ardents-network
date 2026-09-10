package connection

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestDialCancellationInterruptsSetupAndClosesTransport(t *testing.T) {
	for _, test := range []struct {
		name           string
		beginRefusal   bool
		contextFactory func() (context.Context, context.CancelFunc)
		want           error
	}{
		{
			name: "waiting for status without deadline",
			contextFactory: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			want: context.Canceled,
		},
		{
			name:         "waiting for incomplete refusal with distant deadline",
			beginRefusal: true,
			contextFactory: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 10*time.Second)
			},
			want: context.Canceled,
		},
		{
			name: "waiting for status until context deadline",
			contextFactory: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 50*time.Millisecond)
			},
			want: context.DeadlineExceeded,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := shortClientSocketPath(t)
			listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			cleanupUnixListener(t, listener)
			peerReady := make(chan *net.UnixConn, 1)
			go func() {
				peer, acceptErr := listener.AcceptUnix()
				if acceptErr != nil {
					return
				}
				if _, readErr := readSetupRequest(peer); readErr != nil {
					_ = peer.Close()
					return
				}
				if test.beginRefusal {
					if _, writeErr := peer.Write([]byte{0}); writeErr != nil {
						_ = peer.Close()
						return
					}
				}
				peerReady <- peer
			}()
			ctx, cancel := test.contextFactory()
			defer cancel()
			result := make(chan error, 1)
			go func() {
				_, dialErr := Dial(ctx, path, Request{Destination: TargetLink, Value: "ardents-target:v1:cancel-setup"})
				result <- dialErr
			}()
			var peer *net.UnixConn
			select {
			case peer = <-peerReady:
				cleanupUnixConnection(t, peer)
			case <-time.After(time.Second):
				t.Fatal("peer did not receive the setup request")
			}
			if test.want == context.Canceled {
				cancel()
			}
			select {
			case dialErr := <-result:
				if !errors.Is(dialErr, test.want) {
					t.Fatalf("Dial error = %v, want %v", dialErr, test.want)
				}
			case <-time.After(time.Second):
				t.Fatal("Dial waited for the silent peer after context completion")
			}
			if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			var one [1]byte
			if _, err := peer.Read(one[:]); err == nil {
				t.Fatal("canceled setup left its local transport open")
			}
		})
	}
}

func readSetupRequest(peer *net.UnixConn) (string, error) {
	header := make([]byte, len(localMagic)+3)
	if _, err := io.ReadFull(peer, header); err != nil {
		return "", err
	}
	if string(header[:len(localMagic)]) != localMagic {
		return "", errors.New("invalid setup magic")
	}
	link := make([]byte, int(binary.BigEndian.Uint16(header[len(localMagic)+1:])))
	if _, err := io.ReadFull(peer, link); err != nil {
		return "", err
	}
	return string(link), nil
}

func TestDialTransfersCancellationToSuccessfulClient(t *testing.T) {
	path := shortClientSocketPath(t)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	cleanupUnixListener(t, listener)
	peerReady := make(chan *net.UnixConn, 1)
	go func() {
		peer, acceptErr := listener.AcceptUnix()
		if acceptErr != nil {
			return
		}
		if _, readErr := readSetupRequest(peer); readErr != nil {
			_ = peer.Close()
			return
		}
		if _, writeErr := peer.Write([]byte{1}); writeErr != nil {
			_ = peer.Close()
			return
		}
		peerReady <- peer
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	application, err := Dial(ctx, path, Request{Destination: TargetLink, Value: "ardents-target:v1:transferred-client"})
	if err != nil {
		t.Fatal(err)
	}
	cleanupClient(t, application)
	var peer *net.UnixConn
	select {
	case peer = <-peerReady:
		cleanupUnixConnection(t, peer)
	case <-time.After(time.Second):
		t.Fatal("peer did not accept successful setup")
	}
	cancel()
	select {
	case outcome := <-application.Done():
		if outcome.Class != LocalCancellation {
			t.Fatalf("successful Client cancellation outcome = %+v", outcome)
		}
	case <-time.After(time.Second):
		t.Fatal("successful Client did not inherit setup cancellation")
	}
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := peer.Read(one[:]); err == nil {
		t.Fatal("transferred Client cancellation left the local transport open")
	}
}

func TestDialCancellationRacesAcceptedStatusWithoutLeakingClient(t *testing.T) {
	for attempt := range 50 {
		path := shortClientSocketPath(t)
		listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
		if err != nil {
			t.Fatal(err)
		}
		peerReady := make(chan *net.UnixConn, 1)
		writeStatus := make(chan struct{})
		go func() {
			peer, acceptErr := listener.AcceptUnix()
			if acceptErr != nil {
				return
			}
			if _, readErr := readSetupRequest(peer); readErr != nil {
				_ = peer.Close()
				return
			}
			peerReady <- peer
			<-writeStatus
			_, _ = peer.Write([]byte{1})
		}()
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan struct {
			client Client
			err    error
		}, 1)
		go func() {
			client, dialErr := Dial(ctx, path, Request{Destination: TargetLink, Value: "ardents-target:v1:handoff-race"})
			result <- struct {
				client Client
				err    error
			}{client: client, err: dialErr}
		}()
		var peer *net.UnixConn
		select {
		case peer = <-peerReady:
		case <-time.After(time.Second):
			_ = listener.Close()
			t.Fatal("peer did not receive race setup request")
		}
		start := make(chan struct{})
		go func() {
			<-start
			cancel()
		}()
		go func() {
			<-start
			close(writeStatus)
		}()
		close(start)
		select {
		case opened := <-result:
			if opened.client == nil {
				if !errors.Is(opened.err, context.Canceled) {
					t.Fatalf("race attempt %d returned client=%v err=%v", attempt, opened.client, opened.err)
				}
			} else {
				select {
				case outcome := <-opened.client.Done():
					if outcome.Class != LocalCancellation {
						t.Fatalf("race attempt %d Client outcome = %+v", attempt, outcome)
					}
				case <-time.After(time.Second):
					t.Fatalf("race attempt %d leaked a live Client", attempt)
				}
				if err := opened.client.Close(); err != nil {
					t.Fatalf("race attempt %d Client Close = %v", attempt, err)
				}
			}
		case <-time.After(time.Second):
			t.Fatalf("race attempt %d did not complete Dial", attempt)
		}
		if err := peer.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("race attempt %d peer Close = %v", attempt, err)
		}
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("race attempt %d listener Close = %v", attempt, err)
		}
		cancel()
	}
}

func TestSetupErrorPreservesContextCause(t *testing.T) {
	writeFailure := errors.New("local setup request write failed")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := setupError(ctx, writeFailure); !errors.Is(got, context.Canceled) {
		t.Fatalf("canceled setup write error = %v", got)
	}
	deadline, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	if got := setupError(deadline, writeFailure); !errors.Is(got, context.DeadlineExceeded) {
		t.Fatalf("timed out setup write error = %v", got)
	}
	if got := setupError(context.Background(), writeFailure); !errors.Is(got, writeFailure) {
		t.Fatalf("ordinary setup write error = %v", got)
	}
}

func TestDialDoesNotOpenTransportForAlreadyCanceledContext(t *testing.T) {
	path := shortClientSocketPath(t)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	cleanupUnixListener(t, listener)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, err := Dial(ctx, path, Request{Destination: TargetLink, Value: "ardents-target:v1:already-canceled"})
	if client != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("Dial already canceled = client %v, error %v", client, err)
	}
	if err := listener.SetDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if peer, acceptErr := listener.AcceptUnix(); acceptErr == nil {
		_ = peer.Close()
		t.Fatal("already canceled Dial opened a local transport")
	}
}

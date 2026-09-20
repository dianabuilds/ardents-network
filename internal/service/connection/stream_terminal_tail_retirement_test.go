package connection

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type terminalTailRetirementCarrier struct{ err error }

func (carrier terminalTailRetirementCarrier) Read([]byte) (int, error) { return 0, carrier.err }
func (terminalTailRetirementCarrier) Write(value []byte) (int, error)  { return len(value), nil }
func (terminalTailRetirementCarrier) Close() error                     { return nil }

type authenticatedRetirementPipe struct {
	net.Conn
	localClosed chan struct{}
	peerClosed  <-chan struct{}
	once        sync.Once
}

func newAuthenticatedRetirementPipe() (*authenticatedRetirementPipe, *authenticatedRetirementPipe) {
	left, right := net.Pipe()
	leftClosed, rightClosed := make(chan struct{}), make(chan struct{})
	return &authenticatedRetirementPipe{Conn: left, localClosed: leftClosed, peerClosed: rightClosed},
		&authenticatedRetirementPipe{Conn: right, localClosed: rightClosed, peerClosed: leftClosed}
}

func (carrier *authenticatedRetirementPipe) Read(value []byte) (int, error) {
	read, err := carrier.Conn.Read(value)
	if err != nil {
		select {
		case <-carrier.peerClosed:
			err = ErrAttachmentRetired
		default:
		}
	}
	return read, err
}

func (carrier *authenticatedRetirementPipe) Close() error {
	carrier.once.Do(func() {
		close(carrier.localClosed)
		_ = carrier.Conn.Close()
	})
	return nil
}

func TestTerminalTailAcceptsOnlyAuthenticatedAttachmentRetirement(t *testing.T) {
	for _, test := range []struct {
		name         string
		readErr      error
		wantErr      bool
		wantRecovery bool
	}{
		{name: "authenticated retirement", readErr: ErrAttachmentRetired},
		{name: "plain EOF", readErr: io.EOF, wantErr: true, wantRecovery: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var proposals atomic.Int32
			carrier := terminalTailRetirementCarrier{err: test.readErr}
			attachment, err := NewAttachment(carrier, 1, [32]byte{1}, [32]byte{2}, nil)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now()
			stream := &Stream{ctx: t.Context(), current: attachment, authorized: now, started: now,
				opener: func(context.Context, Recovery) (*Attachment, error) {
					proposals.Add(1)
					return nil, errors.New("replacement unavailable")
				},
				recovery:  Recovery{NoNewRecoveryAfter: now.Add(time.Minute).Unix()},
				postClose: true, localTerminal: true, remoteTerminal: true,
				terminalAcknowledgedGeneration: 1, terminalAckPending: true, terminalAckSent: true,
				terminalAckGeneration: 1, terminalAckConfirmedGeneration: 1,
				terminalConfirmationPending: true, terminalConfirmationSent: true,
				ackSignal: make(chan struct{}, 1), done: make(chan struct{}), resources: func(string, int) uint32 { return 0 }}
			stream.cond = sync.NewCond(&stream.mu)
			err = stream.receiveApplicationBounded(0)
			if (err != nil) != test.wantErr {
				t.Fatalf("receive error = %v, want error %t", err, test.wantErr)
			}
			if got := proposals.Load() != 0; got != test.wantRecovery {
				t.Fatalf("recovery attempted = %t, want %t", got, test.wantRecovery)
			}
		})
	}
}

func TestSuccessfulPeerTerminalTailsRetireConcurrently(t *testing.T) {
	testSuccessfulPeerTerminalTailRetirement(t, true)
}

func TestAuthenticatedPeerRetirementReleasesCompleteTerminalTail(t *testing.T) {
	testSuccessfulPeerTerminalTailRetirement(t, false)
}

func testSuccessfulPeerTerminalTailRetirement(t *testing.T, concurrent bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	clientCarrier, publisherCarrier := newAuthenticatedRetirementPipe()
	clientApplication, clientUser := halfClosePair()
	publisherApplication, publisherUser := halfClosePair()
	defer clientUser.Close()
	defer publisherUser.Close()
	connectionContext, exporter, key := [32]byte{11}, [32]byte{12}, [32]byte{13}
	clientAttachment, err := NewAttachment(clientCarrier, 1, connectionContext, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	publisherAttachment, err := NewAttachment(publisherCarrier, 1, connectionContext, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Minute).Unix()
	unexpectedRecovery := func(context.Context, Recovery) (*Attachment, error) {
		return nil, errors.New("unexpected terminal-tail recovery")
	}
	client, err := NewStream(StreamConfig{Context: ctx, Application: clientApplication, Initial: clientAttachment,
		ContinuityKey: key, Authorized: time.Now(), Client: true, Recovery: Recovery{NoNewRecoveryAfter: deadline}, OpenAttachment: unexpectedRecovery})
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := NewStream(StreamConfig{Context: ctx, Application: publisherApplication, Initial: publisherAttachment,
		ContinuityKey: key, Authorized: time.Now(), Recovery: Recovery{NoNewRecoveryAfter: deadline}, OpenAttachment: unexpectedRecovery})
	if err != nil {
		t.Fatal(err)
	}
	runs := make(chan error, 2)
	go func() { _, runErr := client.RunBounded(32, 32); runs <- runErr }()
	go func() { _, runErr := publisher.RunBounded(32, 32); runs <- runErr }()
	if err := clientUser.CloseInput(); err != nil {
		t.Fatal(err)
	}
	if err := publisherUser.CloseInput(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := <-runs; err != nil {
			t.Fatal(err)
		}
	}
	if concurrent {
		retired := make(chan error, 2)
		start := make(chan struct{})
		for _, stream := range []*Stream{client, publisher} {
			go func() {
				<-start
				retired <- stream.RetireTerminalTail()
			}()
		}
		close(start)
		for range 2 {
			if err := <-retired; err != nil {
				t.Fatal(err)
			}
		}
	} else if err := client.RetireTerminalTail(); err != nil {
		t.Fatal(err)
	}
	for _, stream := range []*Stream{client, publisher} {
		select {
		case <-stream.done:
		case <-ctx.Done():
			t.Fatal("successful peer terminal retirement did not release both tails")
		}
		stream.mu.Lock()
		terminal := stream.terminal
		stream.mu.Unlock()
		if terminal != nil {
			t.Fatalf("successful peer terminal retirement published failure: %v", terminal)
		}
	}
}

func TestSuccessfulTerminalTailRetiresWithoutContextFailure(t *testing.T) {
	application, applicationPeer := halfClosePair()
	defer applicationPeer.Close()
	carrier, peer := net.Pipe()
	defer peer.Close()
	var closed atomic.Int32
	attachment, err := NewAttachment(carrier, 1, [32]byte{1}, [32]byte{2}, func() {
		closed.Add(1)
		_ = carrier.Close()
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	stream := &Stream{ctx: t.Context(), application: application, current: attachment, authorized: now, started: now,
		opener:   func(context.Context, Recovery) (*Attachment, error) { return nil, nil },
		recovery: Recovery{NoNewRecoveryAfter: now.Add(time.Minute).Unix()}, localTerminal: true, remoteTerminal: true,
		terminalAcknowledgedGeneration: 1, terminalAckPending: true, terminalAckSent: true,
		terminalAckGeneration: 1, terminalAckConfirmedGeneration: 1, terminalConfirmationPending: true,
		terminalConfirmationSent: true, ackSignal: make(chan struct{}, 1), done: make(chan struct{}), resources: func(string, int) uint32 { return 0 }}
	stream.cond = sync.NewCond(&stream.mu)
	if !stream.startTerminalTail(func() {}, nil) {
		t.Fatal("eligible terminal-control tail was not started")
	}
	if err := stream.RetireTerminalTail(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stream.done:
	case <-time.After(time.Second):
		t.Fatal("successful terminal-control tail did not retire")
	}
	stream.mu.Lock()
	terminal := stream.terminal
	stream.mu.Unlock()
	if terminal != nil {
		t.Fatalf("successful terminal-control retirement published failure: %v", terminal)
	}
	if closed.Load() != 1 {
		t.Fatalf("terminal-control retirement closed Attachment %d times", closed.Load())
	}
}

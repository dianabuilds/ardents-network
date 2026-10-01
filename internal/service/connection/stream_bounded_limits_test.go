package connection

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type boundedStreamPair struct {
	client, publisher         *Stream
	clientUser, publisherUser *halfCloseApplication
	cancel                    context.CancelFunc
}

func openBoundedStreamPair(t *testing.T) boundedStreamPair {
	t.Helper()
	clientCarrier, publisherCarrier := net.Pipe()
	clientApplication, clientUser := halfClosePair()
	publisherApplication, publisherUser := halfClosePair()
	connectionContext, exporter, key := [32]byte{11}, [32]byte{12}, [32]byte{13}
	clientAttachment, err := NewAttachment(clientCarrier, 1, connectionContext, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	publisherAttachment, err := NewAttachment(publisherCarrier, 1, connectionContext, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	client, err := NewStream(StreamConfig{Context: ctx, Application: clientApplication, Initial: clientAttachment,
		ContinuityKey: key, Authorized: time.Now(), Client: true})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	publisher, err := NewStream(StreamConfig{Context: ctx, Application: publisherApplication, Initial: publisherAttachment,
		ContinuityKey: key, Authorized: time.Now()})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_ = clientUser.Close()
		_ = publisherUser.Close()
	})
	return boundedStreamPair{client: client, publisher: publisher, clientUser: clientUser,
		publisherUser: publisherUser, cancel: cancel}
}

func runBoundedPair(pair boundedStreamPair, limit uint32) <-chan error {
	results := make(chan error, 2)
	go func() { _, err := pair.client.RunBounded(limit, limit); results <- err }()
	go func() { _, err := pair.publisher.RunBounded(limit, limit); results <- err }()
	return results
}

func TestStreamBoundedAcceptsZeroAndExactInputEOF(t *testing.T) {
	for _, test := range []struct {
		name    string
		request []byte
		limit   uint32
	}{
		{name: "zero", limit: 32},
		{name: "exact", request: bytes.Repeat([]byte{'e'}, 32), limit: 32},
	} {
		t.Run(test.name, func(t *testing.T) {
			if uint32(len(test.request)) > test.limit {
				t.Fatal("test request exceeds its limit")
			}
			pair := openBoundedStreamPair(t)
			results := runBoundedPair(pair, test.limit)
			written := make(chan error, 1)
			go func() {
				if len(test.request) > 0 {
					if _, err := pair.clientUser.Write(test.request); err != nil {
						written <- err
						return
					}
				}
				written <- pair.clientUser.CloseInput()
			}()
			request, err := io.ReadAll(pair.publisherUser)
			if err != nil || !bytes.Equal(request, test.request) {
				t.Fatalf("publisher request = %q, %v", request, err)
			}
			if err := pair.publisherUser.CloseInput(); err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadAll(pair.clientUser); err != nil {
				t.Fatal(err)
			}
			if err := <-written; err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := <-results; err != nil {
					t.Fatalf("bounded %s request failed: %v", test.name, err)
				}
			}
		})
	}
}

func TestStreamBoundedRejectsInputPastDirectionalLimit(t *testing.T) {
	const limit = uint32(32)
	pair := openBoundedStreamPair(t)
	results := runBoundedPair(pair, limit)
	written := make(chan error, 1)
	go func() {
		_, err := pair.clientUser.Write(bytes.Repeat([]byte{'x'}, int(limit)+1))
		written <- err
	}()
	select {
	case <-written:
	case <-time.After(time.Second):
		t.Fatal("over-limit Application write did not unblock")
	}
	select {
	case err := <-results:
		if err == nil {
			t.Fatal("over-limit Application input completed cleanly")
		}
	case <-time.After(time.Second):
		t.Fatal("over-limit Application input did not terminate")
	}
	pair.cancel()
	select {
	case err := <-results:
		if err == nil {
			t.Fatal("over-limit peer completed cleanly")
		}
	case <-time.After(time.Second):
		t.Fatal("over-limit peer did not join after cancellation")
	}
}

func TestStreamBoundedDoesNotResetDirectionalLimitAfterRecovery(t *testing.T) {
	const limit = uint32(32)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	oldClient, oldPublisher, fault := newTerminalFaultAdapter(t, terminalFault{dropDataAcknowledgement: true})
	defer fault.Close()
	freshClient, freshPublisher := net.Pipe()
	defer freshClient.Close()
	defer freshPublisher.Close()
	clientApplication, clientUser := halfClosePair()
	publisherApplication, publisherUser := halfClosePair()
	defer clientUser.Close()
	defer publisherUser.Close()
	connectionContext, exporter, key := [32]byte{21}, [32]byte{22}, [32]byte{23}
	recovery := Recovery{NoNewRecoveryAfter: time.Now().Add(time.Minute).Unix()}
	clientRecovered := make(chan struct{})
	var clientRecoveredOnce sync.Once
	var clientOpened, publisherOpened atomic.Int32
	client, err := NewStream(StreamConfig{Context: ctx, Application: clientApplication,
		Initial: terminalRecoveryAttachment(t, oldClient, 1, connectionContext, exporter), ContinuityKey: key,
		Authorized: time.Now(), Client: true, Recovery: recovery, OpenAttachment: func(context.Context, Recovery) (*Attachment, error) {
			clientOpened.Add(1)
			clientRecoveredOnce.Do(func() { close(clientRecovered) })
			return terminalRecoveryAttachment(t, freshClient, 2, connectionContext, exporter), nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := NewStream(StreamConfig{Context: ctx, Application: publisherApplication,
		Initial: terminalRecoveryAttachment(t, oldPublisher, 1, connectionContext, exporter), ContinuityKey: key,
		Authorized: time.Now(), Recovery: recovery, OpenAttachment: func(context.Context, Recovery) (*Attachment, error) {
			publisherOpened.Add(1)
			return terminalRecoveryAttachment(t, freshPublisher, 2, connectionContext, exporter), nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	t.Cleanup(func() {
		cancel()
		_ = clientUser.Close()
		_ = publisherUser.Close()
		_ = freshClient.Close()
		_ = freshPublisher.Close()
		fault.Close()
		joined := make(chan struct{})
		go func() {
			workers.Wait()
			close(joined)
		}()
		select {
		case <-joined:
		case <-time.After(time.Second):
			t.Error("bounded recovery test workers did not join during cleanup")
		}
	})
	results := make(chan error, 2)
	workers.Add(3)
	go func() {
		defer workers.Done()
		_, runErr := client.RunBounded(limit, limit)
		results <- runErr
	}()
	go func() {
		defer workers.Done()
		_, runErr := publisher.RunBounded(limit, limit)
		results <- runErr
	}()
	received := make(chan error, 1)
	go func() {
		defer workers.Done()
		body := make([]byte, limit)
		_, readErr := io.ReadFull(publisherUser, body)
		if readErr == nil && !bytes.Equal(body, bytes.Repeat([]byte{'r'}, int(limit))) {
			readErr = io.ErrUnexpectedEOF
		}
		received <- readErr
	}()
	if _, err := clientUser.Write(bytes.Repeat([]byte{'r'}, int(limit))); err != nil {
		t.Fatal(err)
	}
	select {
	case <-fault.dropped:
	case <-ctx.Done():
		t.Fatal("bounded stream did not enter recovery")
	}
	select {
	case <-clientRecovered:
	case <-ctx.Done():
		t.Fatal("bounded stream did not open its recovery Attachment")
	}
	written := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, writeErr := clientUser.Write([]byte{'x'})
		written <- writeErr
	}()
	select {
	case <-written:
	case <-ctx.Done():
		t.Fatal("post-recovery over-limit write did not unblock")
	}
	select {
	case runErr := <-results:
		if runErr == nil {
			t.Fatal("post-recovery over-limit input completed cleanly")
		}
	case <-ctx.Done():
		t.Fatal("post-recovery over-limit input did not terminate")
	}
	if err := <-received; err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case runErr := <-results:
		if runErr == nil {
			t.Fatal("over-limit peer completed cleanly")
		}
	case <-time.After(time.Second):
		t.Fatal("over-limit peer did not join")
	}
	if clientOpened.Load() == 0 || publisherOpened.Load() == 0 {
		t.Fatalf("replacement Attachments = client %d publisher %d", clientOpened.Load(), publisherOpened.Load())
	}
}

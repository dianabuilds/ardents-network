package connection

import (
	"bytes"
	"context"
	"io"
	"net"
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

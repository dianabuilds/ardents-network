//go:build linux

package textdocument

import (
	"bytes"
	"context"
	"crypto/sha256"
	"net"
	"strconv"
	"sync"
	"testing"
	"testing/iotest"
	"time"
)

func TestWorkerReadinessRequiresExactCompleteCurrentAcknowledgement(t *testing.T) {
	nonce := [32]byte{1}
	digest := sha256.Sum256(nil)
	body := append([]byte("ARDTWR01"), nonce[:]...)
	body = append(body, digest[:]...)
	if err := readWorkerReadiness(iotest.OneByteReader(bytes.NewReader(body)), nonce, digest); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{nil, body[:71], append(bytes.Clone(body), 1), append([]byte("WRONG!!!"), body[8:]...)} {
		if readWorkerReadiness(bytes.NewReader(invalid), nonce, digest) == nil {
			t.Fatal("incomplete or unsolicited readiness accepted")
		}
	}
	if readWorkerReadiness(bytes.NewReader(body), [32]byte{2}, digest) == nil || readWorkerReadiness(bytes.NewReader(body), nonce, [32]byte{2}) == nil {
		t.Fatal("another invocation or snapshot accepted")
	}
}

func TestEndpointInitializationInteroperatesWithFixedWorker(t *testing.T) {
	for _, mode := range []WorkerMode{ReaderWorker, PublisherWorker} {
		t.Run(strconv.Itoa(int(mode)), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			client, worker := net.Pipe()
			defer client.Close()
			finished := make(chan error, 1)
			go func() { finished <- RunWorker(ctx, worker, mode) }()
			t.Cleanup(func() {
				_ = client.Close()
				cancel()
				select {
				case <-finished:
				case <-time.After(3 * time.Second):
					t.Error("fixed worker did not join")
				}
			})
			var snapshot []byte
			if mode == PublisherWorker {
				snapshot = []byte("owner document\n")
			}
			if err := InitializeWorker(ctx, client, mode, [32]byte{1}, snapshot); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEndpointInitializationCancellationJoinsBlockedWorkerReadiness(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, worker := net.Pipe()
	defer client.Close()
	defer worker.Close()
	finished := make(chan error, 1)
	go func() { finished <- InitializeWorker(ctx, client, ReaderWorker, [32]byte{1}, nil) }()
	if _, err := readWorkerInitialization(worker, ReaderWorker); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("cancelled initialization accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("blocked readiness did not join cancellation")
	}
}

type delayedInitializationClose struct {
	net.Conn
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (attachment *delayedInitializationClose) Close() error {
	attachment.once.Do(func() { _ = attachment.Conn.Close(); close(attachment.entered) })
	<-attachment.release
	return nil
}

func TestEndpointInitializationWaitsForCancellationCloseToFinish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client, worker := net.Pipe()
	attachment := &delayedInitializationClose{Conn: client, entered: make(chan struct{}), release: make(chan struct{})}
	finished := make(chan error, 1)
	joined := make(chan struct{})
	var release sync.Once
	unblock := func() { release.Do(func() { close(attachment.release) }) }
	go func() {
		defer close(joined)
		finished <- InitializeWorker(ctx, attachment, ReaderWorker, [32]byte{1}, nil)
	}()
	t.Cleanup(func() {
		cancel()
		unblock()
		_ = worker.Close()
		select {
		case <-joined:
		case <-time.After(time.Second):
			t.Error("initialization did not join")
		}
	})
	if _, err := readWorkerInitialization(worker, ReaderWorker); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-attachment.entered:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not start Close")
	}
	select {
	case <-finished:
		t.Fatal("initialization returned before its Close completed")
	case <-time.After(25 * time.Millisecond):
	}
	unblock()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("cancelled initialization accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("initialization did not join completed Close")
	}
}

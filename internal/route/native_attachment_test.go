package route

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type attachmentTestWorkers struct {
	results chan error
	started int
	joined  int
}

func newAttachmentTestWorkers(capacity int) *attachmentTestWorkers {
	return &attachmentTestWorkers{results: make(chan error, capacity)}
}

func (workers *attachmentTestWorkers) start(work func() error) {
	workers.started++
	go func() { workers.results <- work() }()
}

func (workers *attachmentTestWorkers) wait(t *testing.T) error {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case result := <-workers.results:
		workers.joined++
		return result
	case <-timer.C:
		t.Fatal("attachment test worker did not terminate")
		return nil
	}
}

func (workers *attachmentTestWorkers) join(t *testing.T) {
	t.Helper()
	for workers.joined < workers.started {
		_ = workers.wait(t)
	}
}

func TestAttachmentReturnsImmutableEvidenceAndCleansOnce(t *testing.T) {
	left, right := net.Pipe()
	var closed atomic.Int32
	attachment := &Attachment{connection: left, evidence: Evidence{AuthenticatedTarget: [32]byte{1}, AuthorityPublic: [32]byte{2},
		Publication: []byte("publication"), Generation: 3, AttachmentID: [32]byte{4}}, close: func() error {
		closed.Add(1)
		return left.Close()
	}}
	t.Cleanup(func() {
		_ = attachment.Close()
		_ = right.Close()
	})
	evidence, err := attachment.Evidence()
	if err != nil {
		t.Fatal(err)
	}
	evidence.Publication[0] = 'x'
	again, err := attachment.Evidence()
	if err != nil || string(again.Publication) != "publication" {
		t.Fatalf("attachment evidence = %+v, %v", again, err)
	}
	if err := attachment.Close(); err != nil {
		t.Fatal(err)
	}
	if err := attachment.Close(); err != nil {
		t.Fatal(err)
	}
	if closed.Load() != 1 {
		t.Fatalf("attachment cleanup calls = %d", closed.Load())
	}
}

func TestAttachmentDelegatesCarrierContract(t *testing.T) {
	left, right := net.Pipe()
	attachment := &Attachment{connection: left, evidence: Evidence{AuthenticatedTarget: [32]byte{1}, AttachmentID: [32]byte{2}}, close: left.Close}
	workers := newAttachmentTestWorkers(2)
	t.Cleanup(func() {
		_ = attachment.Close()
		_ = right.Close()
		workers.join(t)
	})
	deadline := time.Now().Add(time.Second)
	if attachment.LocalAddr() == nil || attachment.RemoteAddr() == nil {
		t.Fatal("attachment did not expose carrier addresses")
	}
	if err := attachment.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := attachment.SetReadDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := attachment.SetWriteDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	workers.start(func() error {
		_, err := right.Write([]byte("in"))
		return err
	})
	input := make([]byte, 2)
	if _, err := io.ReadFull(attachment, input); err != nil || string(input) != "in" {
		t.Fatalf("attachment read = %q, %v", input, err)
	}
	if err := workers.wait(t); err != nil {
		t.Fatal(err)
	}
	workers.start(func() error {
		output := make([]byte, 3)
		_, err := io.ReadFull(right, output)
		if err == nil && string(output) != "out" {
			err = errors.New("carrier received unexpected attachment bytes")
		}
		return err
	})
	if _, err := attachment.Write([]byte("out")); err != nil {
		t.Fatal(err)
	}
	if err := workers.wait(t); err != nil {
		t.Fatal(err)
	}
	if err := attachment.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAttachmentCloseJoinsConcurrentCleanupFailure(t *testing.T) {
	left, right := net.Pipe()
	cleanupFailure := errors.New("attachment cleanup failed")
	entered, unblock := make(chan struct{}), make(chan struct{})
	var release sync.Once
	var closed atomic.Int32
	attachment := &Attachment{connection: left, evidence: Evidence{AuthenticatedTarget: [32]byte{1}, AttachmentID: [32]byte{2}}, close: func() error {
		closed.Add(1)
		close(entered)
		<-unblock
		_ = left.Close()
		return cleanupFailure
	}}
	workers := newAttachmentTestWorkers(2)
	t.Cleanup(func() {
		release.Do(func() { close(unblock) })
		_ = attachment.Close()
		_ = right.Close()
		workers.join(t)
	})
	workers.start(attachment.Close)
	timer := time.NewTimer(time.Second)
	select {
	case <-entered:
		if !timer.Stop() {
			<-timer.C
		}
	case <-timer.C:
		t.Fatal("attachment cleanup did not start")
	}
	workers.start(attachment.Close)
	release.Do(func() { close(unblock) })
	for range 2 {
		if err := workers.wait(t); !errors.Is(err, cleanupFailure) {
			t.Fatalf("concurrent attachment cleanup error = %v, want cleanup failure", err)
		}
	}
	if closed.Load() != 1 {
		t.Fatalf("concurrent attachment cleanup calls = %d, want 1", closed.Load())
	}
}
